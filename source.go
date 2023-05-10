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

package http

//go:generate paramgen -output=paramgen_src.go SourceConfig

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/time/rate"
	"io"
	"net/http"
	"time"

	sdk "github.com/conduitio/conduit-connector-sdk"
)

type Source struct {
	sdk.UnimplementedSource

	config  SourceConfig
	client  *http.Client
	limiter *rate.Limiter
}

type SourceConfig struct {
	URL           string        `json:"url" validate:"required"`
	PollingPeriod time.Duration `json:"pollingPeriod" default:"5m"`
}

func NewSource() sdk.Source {
	// Create Source and wrap it in the default middleware.
	return sdk.SourceWithMiddleware(&Source{})
}

func (s *Source) Parameters() map[string]sdk.Parameter {
	// Parameters is a map of named Parameters that describe how to configure
	// the Source. Parameters can be generated from SourceConfig with paramgen.
	return s.config.Parameters()
}

func (s *Source) Configure(ctx context.Context, cfg map[string]string) error {
	// Configure is the first function to be called in a connector. It provides
	// the connector with the configuration that can be validated and stored.
	// In case the configuration is not valid it should return an error.
	// Testing if your connector can reach the configured data source should be
	// done in Open, not in Configure.
	// The SDK will validate the configuration and populate default values
	// before calling Configure. If you need to do more complex validations you
	// can do them manually here.

	sdk.Logger(ctx).Info().Msg("Configuring Source...")
	err := sdk.Util.ParseConfig(cfg, &s.config)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return nil
}

func (s *Source) Open(ctx context.Context, pos sdk.Position) error {
	// Open is called after Configure to signal the plugin it can prepare to
	// start producing records. If needed, the plugin should open connections in
	// this function. The position parameter will contain the position of the
	// last record that was successfully processed, Source should therefore
	// start producing records after this position. The context passed to Open
	// will be cancelled once the plugin receives a stop signal from Conduit.

	s.client = &http.Client{}

	// check connection
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, s.config.URL, nil)
	if err != nil {
		return fmt.Errorf("error creating HTTP request %q: %w", s.config.URL, err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error pinging URL %q: %w", s.config.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authorization failed, check your key")
	}

	s.limiter = rate.NewLimiter(rate.Every(s.config.PollingPeriod), 1)

	return nil
}

func (s *Source) Read(ctx context.Context) (sdk.Record, error) {
	// Read returns a new Record and is supposed to block until there is either
	// a new record or the context gets cancelled. It can also return the error
	// ErrBackoffRetry to signal to the SDK it should call Read again with a
	// backoff retry.
	// If Read receives a cancelled context or the context is cancelled while
	// Read is running it must stop retrieving new records from the source
	// system and start returning records that have already been buffered. If
	// there are no buffered records left Read must return the context error to
	// signal a graceful stop. If Read returns ErrBackoffRetry while the context
	// is cancelled it will also signal that there are no records left and Read
	// won't be called again.
	// After Read returns an error the function won't be called again (except if
	// the error is ErrBackoffRetry, as mentioned above).
	// Read can be called concurrently with Ack.

	// TODO: Use ErrBackoffRetry when there's nothing new to process.
	err := s.limiter.Wait(ctx)
	if err != nil {
		return sdk.Record{}, err
	}
	rec, err := s.getRecord(ctx)
	if err != nil {
		return sdk.Record{}, fmt.Errorf("error getting data: %w", err)
	}
	return rec, nil
	return sdk.Record{}, nil
}

func (s *Source) getRecord(ctx context.Context) (sdk.Record, error) {
	// create GET request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.config.URL, nil)
	if err != nil {
		return sdk.Record{}, fmt.Errorf("error creating HTTP request: %w", err)
	}
	// get response
	resp, err := s.client.Do(req)
	if err != nil {
		return sdk.Record{}, fmt.Errorf("error getting data from URL: %w", err)
	}
	defer resp.Body.Close()
	// check response status
	if resp.StatusCode != http.StatusOK {
		return sdk.Record{}, fmt.Errorf("response status should be %v, got status=%v", http.StatusOK, resp.StatusCode)
	}
	// read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sdk.Record{}, fmt.Errorf("error reading body for response %v: %w", resp, err)
	}
	// parse json
	var structData sdk.StructuredData
	err = json.Unmarshal(body, &structData)
	if err != nil {
		return sdk.Record{}, fmt.Errorf("failed to unmarshal body as JSON: %w", err)
	}
	// create record
	now := time.Now().Unix()

	rec := sdk.Record{
		Payload: sdk.Change{
			Before: nil,
			After:  structData,
		},
		Operation: sdk.OperationCreate,
		Position:  sdk.Position(fmt.Sprintf("unix-%v", now)),
		Key:       sdk.RawData(fmt.Sprintf("%v", now)),
	}
	return rec, nil
}

func (s *Source) Ack(ctx context.Context, position sdk.Position) error {
	// Ack signals to the implementation that the record with the supplied
	// position was successfully processed. This method might be called after
	// the context of Read is already cancelled, since there might be
	// outstanding acks that need to be delivered. When Teardown is called it is
	// guaranteed there won't be any more calls to Ack.
	// Ack can be called concurrently with Read.
	return nil
}

func (s *Source) Teardown(ctx context.Context) error {
	// Teardown signals to the plugin that there will be no more calls to any
	// other function. After Teardown returns, the plugin should be ready for a
	// graceful shutdown.

	if s.client != nil {
		s.client.CloseIdleConnections()
	}
	return nil
}
