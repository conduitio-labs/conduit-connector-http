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
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	sdk "github.com/conduitio/conduit-connector-sdk"
	"golang.org/x/time/rate"
)

type Source struct {
	sdk.UnimplementedSource

	config    SourceConfig
	client    *http.Client
	limiter   *rate.Limiter
	header    http.Header
	extension *sourceExtension
}

type SourceConfig struct {
	Config
	// how often the connector will get data from the url
	PollingPeriod time.Duration `json:"pollingPeriod" default:"5m"`
	// Http method to use in the request
	Method string `default:"GET" validate:"inclusion=GET|HEAD|OPTIONS"`

	// The path to a .js file containing the code to prepare the request data.
	GetRequestDataScript string `json:"script.getRequestData"`
	// The path to a .js file containing the code to parse the response.
	ParseResponseScript string `json:"script.parseResponse"`
}

func NewSource() sdk.Source {
	return sdk.SourceWithMiddleware(
		&Source{extension: newSourceExtension()},
	)
}

func (s *Source) Parameters() map[string]sdk.Parameter {
	return s.config.Parameters()
}

func (s *Source) Configure(ctx context.Context, cfg map[string]string) error {
	sdk.Logger(ctx).Info().Msg("Configuring Source...")

	var config SourceConfig
	err := sdk.Util.ParseConfig(cfg, &config)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	s.config.URL, err = s.config.addParamsToURL()
	if err != nil {
		return err
	}
	s.header, err = config.Config.getHeader()
	if err != nil {
		return fmt.Errorf("invalid header config: %w", err)
	}

	err = s.extension.configure(config.GetRequestDataScript, config.ParseResponseScript)
	if err != nil {
		return fmt.Errorf("failed configuring JS extension: %w", err)
	}
	s.config = config

	return nil
}

func (s *Source) Open(ctx context.Context, pos sdk.Position) error {
	// create client
	s.client = &http.Client{}

	// check connection
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, s.config.URL, nil)
	if err != nil {
		return fmt.Errorf("error creating HTTP request %q: %w", s.config.URL, err)
	}
	req.Header = s.header
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error pinging URL %q: %w", s.config.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		return fmt.Errorf("authorization failed, %s: %s", http.StatusText(http.StatusUnauthorized), string(body))
	}
	s.limiter = rate.NewLimiter(rate.Every(s.config.PollingPeriod), 1)

	return nil
}

func (s *Source) Read(ctx context.Context) (sdk.Record, error) {
	err := s.limiter.Wait(ctx)
	if err != nil {
		return sdk.Record{}, err
	}
	rec, err := s.getRecord(ctx)
	if err != nil {
		return sdk.Record{}, fmt.Errorf("error getting data: %w", err)
	}
	return rec, nil
}

func (s *Source) getRecord(ctx context.Context) (sdk.Record, error) {
	// create GET request
	req, err := http.NewRequestWithContext(ctx, s.config.Method, s.config.URL, nil)
	if err != nil {
		return sdk.Record{}, fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.Header = s.header
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
	// add response header to metadata
	meta := sdk.Metadata{}
	for key, val := range resp.Header {
		meta[key] = strings.Join(val, ",")
	}

	// create record
	now := time.Now().Unix()
	rec := sdk.Record{
		Payload: sdk.Change{
			Before: nil,
			After:  sdk.RawData(body),
		},
		Metadata:  meta,
		Operation: sdk.OperationCreate,
		Position:  sdk.Position(fmt.Sprintf("unix-%v", now)),
		Key:       sdk.RawData(fmt.Sprintf("%v", now)),
	}
	return rec, nil
}

func (s *Source) Ack(ctx context.Context, position sdk.Position) error {
	sdk.Logger(ctx).Debug().Str("position", string(position)).Msg("got ack")
	return nil
}

func (s *Source) Teardown(context.Context) error {
	if s.client != nil {
		s.client.CloseIdleConnections()
	}
	return nil
}
