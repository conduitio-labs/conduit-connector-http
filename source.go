// Copyright © 2023 Meroxa, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	sdk "github.com/conduitio/conduit-connector-sdk"
	"golang.org/x/time/rate"
)

type Source struct {
	sdk.UnimplementedSource

	config   SourceConfig
	position sdk.Position
	messages chan sdk.StructuredData
	errs     chan error
	client   *http.Client
	limiter  *rate.Limiter
	header   http.Header
}

func NewSource() sdk.Source {
	return sdk.SourceWithMiddleware(&Source{})
}

func (s *Source) Parameters() map[string]sdk.Parameter {
	return s.config.Parameters()
}

func (s *Source) Configure(ctx context.Context, cfg map[string]string) error {
	sdk.Logger(ctx).Info().Msg("Configuring Source...")
	config, header, err := s.config.ParseConfig(cfg)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	s.config = config
	s.header = header
	return nil
}

func (s *Source) Open(ctx context.Context, pos sdk.Position) error {
	// create client
	s.client = &http.Client{}

	// check connection
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, s.config.url, nil)
	if err != nil {
		return fmt.Errorf("error creating HTTP request %q: %w", s.config.url, err)
	}
	// set headers
	req.Header = s.header
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error pinging URL %q: %w", s.config.url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authorization failed, check your key")
	}

	s.limiter = rate.NewLimiter(rate.Every(s.config.PollingPeriod), 1)
	s.position = pos
	s.messages = make(chan sdk.StructuredData, 10)
	s.errs = make(chan error)

	// spawn go routine
	go s.getRecords(ctx)

	return nil
}

func (s *Source) Read(ctx context.Context) (sdk.Record, error) {
	select {
	case msg := <-s.messages:
		rec, err := s.createRecord(msg)
		if err != nil {
			return sdk.Record{}, fmt.Errorf("error creating record: %w", err)
		}
		return rec, nil
	case err := <-s.errs:
		return sdk.Record{}, err
	case <-ctx.Done():
		return sdk.Record{}, ctx.Err()
	default:
		return sdk.Record{}, sdk.ErrBackoffRetry
	}
}

func (s *Source) getRecords(ctx context.Context) {
	for {
		// get response body
		body, err := s.getResponse(ctx)
		if err != nil {
			s.errs <- fmt.Errorf("failed to get response: %w", err)
			return
		}

		// parse json array
		var msgs []sdk.StructuredData
		err = json.Unmarshal(body, &msgs)
		if err != nil {
			s.errs <- fmt.Errorf("failed to unmarshal body as JSON Array: %w", err)
			return
		}

		// validate messages
		err = s.validateMessages(msgs)
		if err != nil {
			s.errs <- fmt.Errorf("invalid message format: %w", err)
			return
		}

		// loop in reverse, to start with the oldest message
		for i := len(msgs) - 1; i >= 0; i-- {
			s.messages <- msgs[i]
			if i == 0 {
				// first message in the slice, is the latest message in the channel, so start reading messages
				// from after this one
				s.position = sdk.Position(msgs[i]["id"].(string))
			}
		}

		// delay
		err = s.limiter.Wait(ctx)
		if err != nil {
			s.errs <- err
			return
		}
	}
}

func (s *Source) createRecord(msg sdk.StructuredData) (sdk.Record, error) {
	id := msg["id"].(string)
	rec := sdk.Record{
		Payload: sdk.Change{
			Before: nil,
			After:  msg,
		},
		Operation: sdk.OperationCreate,
		Position:  sdk.Position(id),
		Key:       sdk.RawData(id),
	}
	return rec, nil
}

func (s *Source) validateMessages(msgs []sdk.StructuredData) error {
	for _, msg := range msgs {
		if _, ok := msg["id"].(string); !ok {
			return fmt.Errorf("id field not found")
		}
		if _, ok := msg["content"].(string); !ok {
			return fmt.Errorf("content field not found")
		}
		if _, ok := msg["type"]; !ok {
			return fmt.Errorf("type field not found")
		}
		author, ok := msg["author"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("author field not found")
		}
		if _, ok := author["username"].(string); !ok {
			return fmt.Errorf("author.username field not found")
		}
	}
	// type 7 joined the server
	// type 19 is a reply
	// type 0 is default
	return nil
}

func (s *Source) getResponse(ctx context.Context) ([]byte, error) {
	url := s.config.url
	if s.position != nil {
		url = s.config.url + "?after=" + string(s.position)
	}
	// create GET request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.Header = s.header
	// get response
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error getting data from URL: %w", err)
	}
	defer resp.Body.Close()
	// check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response status should be %v, got status=%v", http.StatusOK, resp.StatusCode)
	}
	// read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body for response %v: %w", resp, err)
	}
	return body, nil
}

func (s *Source) Ack(ctx context.Context, position sdk.Position) error {
	sdk.Logger(ctx).Debug().Str("position", string(position)).Msg("got ack")
	return nil
}

func (s *Source) Teardown(ctx context.Context) error {
	if s.client != nil {
		s.client.CloseIdleConnections()
	}
	if s.messages != nil {
		close(s.messages)
	}
	if s.errs != nil {
		close(s.errs)
	}
	return nil
}
