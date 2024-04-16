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

	config  SourceConfig
	client  *http.Client
	limiter *rate.Limiter
	header  http.Header

	extension        *sourceExtension
	lastResponseData map[string]any
	buffer           []sdk.Record
	lastPosition     sdk.Position
}

type SourceConfig struct {
	Config
	// how often the connector will get data from the url
	PollingPeriod time.Duration `json:"pollingPeriod" default:"5m"`
	// Http method to use in the request
	Method string `default:"GET" validate:"inclusion=GET|HEAD|OPTIONS"`

	// The path to a .js file containing the code to prepare the request data.
	// The signature of the function needs to be:
	// `function getRequestData(cfg, previousResponse, position)` where:
	// * `cfg` (a map) is the connector configuration
	// * `previousResponse` (a map) contains data from the previous response (if any), returned by `parseResponse`
	// * `position` (a byte array) contains the starting position of the connector.
	// The function needs to return a Request object.
	GetRequestDataScript string `json:"script.getRequestData"`
	// The path to a .js file containing the code to parse the response.
	// The signature of the function needs to be:
	// `function parseResponse(bytes)` where
	// `bytes` are the original response's raw bytes (i.e. unparsed).
	// The response should be a Response object.
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
	sdk.Logger(ctx).Info().Msg("configuring source...")

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
	sdk.Logger(ctx).Info().Msg("opening source")
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

	err = s.extension.open(ctx)
	if err != nil {
		return fmt.Errorf("failed to open extension: %w", err)
	}

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
	// input: config, lastPosition
	// output: request = URL + Headers
	if len(s.buffer) == 0 {
		err := s.limiter.Wait(ctx)
		if err != nil {
			return sdk.Record{}, err
		}

		err = s.fillBuffer(ctx)
		if err != nil {
			return sdk.Record{}, err
		}
	}

	if len(s.buffer) == 0 {
		return sdk.Record{}, sdk.ErrBackoffRetry
	}

	sdk.Logger(ctx).Trace().Msg("returning record")

	rec := s.buffer[0]
	s.buffer = s.buffer[1:]
	s.lastPosition = rec.Position

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

func (s *Source) fillBuffer(ctx context.Context) error {
	sdk.Logger(ctx).Debug().Msg("filling buffer")
	// create request
	reqData, err := s.getRequestData()
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

	if !s.extension.hasParseResponseData {
		meta := sdk.Metadata{}
		for key, val := range resp.Header {
			meta[key] = strings.Join(val, ",")
		}

		// create record
		now := time.Now().Unix()
		s.buffer = []sdk.Record{
			sdk.Record{
				Payload: sdk.Change{
					Before: nil,
					After:  sdk.RawData(body),
				},
				Metadata:  meta,
				Operation: sdk.OperationCreate,
				Position:  sdk.Position(fmt.Sprintf("unix-%v", now)),
				Key:       sdk.RawData(fmt.Sprintf("%v", now)),
			},
		}
		return nil
	}

	respData, err := s.extension.parseResponseData(body)
	if err != nil {
		return err
	}

	sdk.Logger(ctx).Info().Msgf("parsing %v JS records into SDK records", len(respData.Records))
	for _, jsRec := range respData.Records {
		_ = jsRec
		s.buffer = append(
			s.buffer,
			s.toSDKRecord(jsRec, resp),
		)
	}

	s.lastResponseData = respData.CustomData

	return nil
}

func (s *Source) toSDKRecord(jsRec *jsRecord, resp *http.Response) sdk.Record {
	meta := sdk.Metadata{}
	for key, val := range resp.Header {
		meta[key] = strings.Join(val, ",")
	}

	convertData := func(d interface{}) sdk.Data {
		switch v := d.(type) {
		case sdk.RawData:
			return v
		case map[string]interface{}:
			return sdk.StructuredData(v)
		}
		return nil
	}

	return sdk.SourceUtil{}.NewRecordCreate(
		jsRec.Position,
		meta,
		convertData(jsRec.Key),
		convertData(jsRec.Payload.After),
	)
}

func (s *Source) getRequestData() (*Request, error) {
	if !s.extension.hasGetRequestData {
		return &Request{URL: s.config.URL}, nil
	}

	return s.extension.getRequestData(s.config, s.lastResponseData, s.lastPosition)
}
