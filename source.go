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
	sdk "github.com/conduitio/conduit-connector-sdk"
	"golang.org/x/time/rate"
	"io"
	"net/http"
	"os"
)

type Source struct {
	sdk.UnimplementedSource

	config  SourceConfig
	client  *http.Client
	limiter *rate.Limiter
	header  http.Header

	jsHelper          *jsHelper
	lastResponseStuff map[string]any
	buffer            []sdk.Record
	lastPosition      sdk.Position
}

func NewSource() sdk.Source {
	return sdk.SourceWithMiddleware(&Source{
		jsHelper:          newJSHelper(),
		lastResponseStuff: map[string]any{},
	})
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

	getRequestDataSrc, err := os.ReadFile(s.config.GetRequestDataScript)
	if err != nil {
		return fmt.Errorf("failed reading %v: %w", s.config.GetRequestDataScript, err)
	}

	s.jsHelper.getRequestDataSrc = string(getRequestDataSrc)

	parseResponseSrc, err := os.ReadFile(s.config.ParseResponseScript)
	if err != nil {
		return fmt.Errorf("failed reading %v: %w", s.config.ParseResponseScript, err)
	}

	s.jsHelper.parseResponseSrc = string(parseResponseSrc)

	return nil
}

func (s *Source) Open(ctx context.Context, pos sdk.Position) error {
	// create client
	s.client = &http.Client{}

	// check connection
	_, err := http.NewRequestWithContext(ctx, http.MethodHead, s.config.URL, nil)
	if err != nil {
		return fmt.Errorf("error creating HTTP request %q: %w", s.config.URL, err)
	}

	s.limiter = rate.NewLimiter(rate.Every(s.config.PollingPeriod), 1)

	err = s.jsHelper.Open(ctx)
	if err != nil {
		return fmt.Errorf("failed initializing JS helper: %w", err)
	}

	s.lastPosition = pos

	return nil
}

func (s *Source) Read(ctx context.Context) (sdk.Record, error) {
	sdk.Logger(ctx).Info().Msg("source read called")

	// TODO: Use ErrBackoffRetry when there's nothing new to process.
	err := s.limiter.Wait(ctx)
	if err != nil {
		return sdk.Record{}, err
	}
	rec, err := s.getRecord(ctx)
	if err != nil {
		return sdk.Record{}, fmt.Errorf("error getting data: %w", err)
	}

	sdk.Logger(ctx).Info().Any("record", string(rec.Bytes())).Msg("returning record")
	return rec, nil
}

func (s *Source) getRecord(ctx context.Context) (sdk.Record, error) {
	// input: config, lastPosition
	// output: request = URL + Headers
	if len(s.buffer) == 0 {
		err := s.fillBuffer(ctx)
		if err != nil {
			return sdk.Record{}, err
		}
	}

	if len(s.buffer) == 0 {
		return sdk.Record{}, sdk.ErrBackoffRetry
	}

	rec := s.buffer[0]
	s.buffer = s.buffer[1:]

	s.lastPosition = rec.Position

	sdk.Logger(ctx).Info().Msg("returning single record")
	return rec, nil
}

func (s *Source) Ack(ctx context.Context, position sdk.Position) error {
	sdk.Logger(ctx).Debug().Str("position", string(position)).Msg("got ack")
	return nil
}

func (s *Source) Teardown(ctx context.Context) error {
	if s.client != nil {
		s.client.CloseIdleConnections()
	}
	return nil
}

func (s *Source) fillBuffer(ctx context.Context) error {
	sdk.Logger(ctx).Info().Msg("filling buffer")
	// create request
	reqData, err := s.jsHelper.getRequestData(ctx, s.config, s.lastResponseStuff, s.lastPosition)
	if err != nil {
		return err
	}

	sdk.Logger(ctx).Info().Msg("request URL: " + reqData.URL)
	req, err := http.NewRequestWithContext(ctx, s.config.Method, reqData.URL, nil)
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %w", err)
	}
	req.Header = s.header

	// get response
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("error getting data from URL: %w", err)
	}
	defer resp.Body.Close()
	// check response status
	if resp.StatusCode != http.StatusOK {
		errorMsg := "unknown"
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			errorMsg = string(body)
		}

		return fmt.Errorf(
			"response status should be %v, got status=%v, cause=%v",
			http.StatusOK,
			resp.StatusCode,
			errorMsg,
		)
	}

	// read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading body for response %v: %w", resp, err)
	}

	respData, err := s.jsHelper.parseResponseData(ctx, body)
	if err != nil {
		return err
	}

	s.lastResponseStuff = respData.Stuff

	sdk.Logger(ctx).Info().Msgf("parsing %v JS records into SDK records", len(respData.Records))
	for _, jsRec := range respData.Records {
		_ = jsRec
		s.buffer = append(
			s.buffer,
			sdk.SourceUtil{}.NewRecordCreate(
				jsRec.Position,
				sdk.Metadata{
					"foo": "bar",
				},
				*jsRec.Key.(*sdk.RawData),
				*jsRec.Payload.After.(*sdk.RawData),
			),
		)
	}

	sdk.Logger(ctx).Info().Msgf("all JS records parsed, buffer size %v", len(s.buffer))

	return nil
}
