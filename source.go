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

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/conduitio/conduit-commons/opencdc"
	sdk "github.com/conduitio/conduit-connector-sdk"
	"golang.org/x/time/rate"
)

//go:generate mockgen -destination=mock_request_builder.go -source=source.go -package=http -mock_names=requestBuilder=MockRequestBuilder . requestBuilder
//go:generate mockgen -destination=mock_response_parser.go -source=source.go -package=http -mock_names=responseParser=MockResponseParser . responseParser

type requestBuilder interface {
	build(
		ctx context.Context,
		previousResponseData map[string]any,
		position opencdc.Position,
	) (*Request, error)
}

type responseParser interface {
	parse(ctx context.Context, responseBytes []byte) (*Response, error)
}

type Source struct {
	sdk.UnimplementedSource

	config SourceConfig
	header http.Header

	client  *http.Client
	limiter *rate.Limiter

	lastResponseData map[string]any
	buffer           []opencdc.Record
	lastPosition     opencdc.Position

	requestBuilder requestBuilder
	responseParser responseParser
}

func (s *Source) Config() sdk.SourceConfig {
	return &s.config
}

type SourceConfig struct {
	sdk.DefaultSourceMiddleware

	Config

	// Http url to send requests to
	URL string `json:"url" validate:"required"`

	// how often the connector will get data from the url
	PollingPeriod time.Duration `json:"pollingPeriod" default:"5m"`

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

	// HTTP method to use in the request
	Method string `default:"GET" validate:"inclusion=GET|HEAD|OPTIONS"`
}

func NewSource() sdk.Source {
	return sdk.SourceWithMiddleware(&Source{})
}

func (c *SourceConfig) Validate(ctx context.Context) error {
	var errs []error

	if err := c.DefaultSourceMiddleware.Validate(ctx); err != nil {
		errs = append(errs, err)
	}

	// Custom validations
	_, err := c.addParamsToURL(c.URL)
	if err != nil {
		errs = append(errs, err)
	}

	_, err = c.getHeader()
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid header config: %w", err))
	}

	return errors.Join(errs...)
}

func (s *Source) Open(ctx context.Context, pos opencdc.Position) error {
	var err error

	// These were already validated
	s.config.URL, _ = s.config.addParamsToURL(s.config.URL)
	s.header, _ = s.config.getHeader()

	if s.config.GetRequestDataScript != "" {
		s.requestBuilder, err = newJSRequestBuilder(ctx, s.config, s.config.GetRequestDataScript)
		if err != nil {
			return fmt.Errorf("failed initializing %v: %w", getRequestDataFn, err)
		}
	}

	if s.config.ParseResponseScript != "" {
		s.responseParser, err = newJSResponseParser(ctx, s.config.ParseResponseScript)
		if err != nil {
			return fmt.Errorf("failed initializing %v: %w", parseResponseFn, err)
		}
	}

	sdk.Logger(ctx).Info().Msg("opening source")

	s.config.URL, err = s.config.addParamsToURL(s.config.URL)
	if err != nil {
		return err
	}

	s.client = &http.Client{}

	if err := s.testConnection(ctx); err != nil {
		return fmt.Errorf("failed connection test: %w", err)
	}

	s.limiter = rate.NewLimiter(rate.Every(s.config.PollingPeriod), 1)
	s.lastPosition = pos

	return nil
}

func (s *Source) testConnection(ctx context.Context) error {
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
	if resp.StatusCode >= 300 {
		return fmt.Errorf("invalid response status code: (%d) %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	s.limiter = rate.NewLimiter(rate.Every(s.config.PollingPeriod), 1)

	return nil
}

func (s *Source) Read(ctx context.Context) (opencdc.Record, error) {
	rec, err := s.getRecord(ctx)
	if err != nil {
		return opencdc.Record{}, fmt.Errorf("error getting data: %w", err)
	}
	return rec, nil
}

func (s *Source) getRecord(ctx context.Context) (opencdc.Record, error) {
	if len(s.buffer) == 0 {
		err := s.limiter.Wait(ctx)
		if err != nil {
			return opencdc.Record{}, err
		}

		err = s.fillBuffer(ctx)
		if err != nil {
			return opencdc.Record{}, err
		}
	}

	if len(s.buffer) == 0 {
		return opencdc.Record{}, sdk.ErrBackoffRetry
	}

	sdk.Logger(ctx).Trace().Msg("returning record")

	rec := s.buffer[0]
	s.buffer = s.buffer[1:]
	s.lastPosition = rec.Position

	return rec, nil
}

func (s *Source) Ack(ctx context.Context, position opencdc.Position) error {
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
	reqData, err := s.getRequestData(ctx)
	if err != nil {
		return err
	}

	sdk.Logger(ctx).Debug().Msg("request URL: " + reqData.URL)
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

	// NB: Conduit's built-in HTTP processor parses responses in the same way
	if resp.StatusCode >= 300 {
		return s.buildError(resp)
	}

	err = s.parseResponse(ctx, resp)
	if err != nil {
		return fmt.Errorf("failed parsing response: %w", err)
	}

	return nil
}

func (s *Source) buildError(resp *http.Response) error {
	errorMsg := "unknown"
	body, err := io.ReadAll(resp.Body)
	if err == nil {
		errorMsg = string(body)
	}

	return fmt.Errorf(
		"expected response status 200, but got status=%v, cause=%v",
		resp.StatusCode,
		errorMsg,
	)
}

func (s *Source) getRequestData(ctx context.Context) (*Request, error) {
	if s.requestBuilder == nil {
		return &Request{URL: s.config.URL}, nil
	}

	return s.requestBuilder.build(ctx, s.lastResponseData, s.lastPosition)
}

func (s *Source) parseResponse(ctx context.Context, resp *http.Response) error {
	sdk.Logger(ctx).Debug().Msg("parsing response")

	// read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading body for response %v: %w", resp, err)
	}

	// no custom parsing, the whole response is transformed into a record
	if s.responseParser == nil {
		s.buffer = append(s.buffer, s.parseAsSingleRecord(resp, body))
		return nil
	}

	respData, err := s.responseParser.parse(ctx, body)
	if err != nil {
		return err
	}

	sdk.Logger(ctx).Debug().Int("count", len(respData.Records)).Msg("parsing JS records into SDK records")

	for _, jsRec := range respData.Records {
		rec, err := s.toSDKRecord(jsRec, resp)
		if err != nil {
			return fmt.Errorf("failed converting JS record to opencdc.Record: %w", err)
		}
		s.buffer = append(s.buffer, rec)
	}
	s.lastResponseData = respData.CustomData

	return nil
}

func (s *Source) toSDKRecord(jsRec *jsRecord, resp *http.Response) (opencdc.Record, error) {
	toSDKData := func(d interface{}) opencdc.Data {
		switch v := d.(type) {
		case opencdc.RawData:
			return v
		case map[string]interface{}:
			return opencdc.StructuredData(v)
		}
		return nil
	}

	var op opencdc.Operation
	err := op.UnmarshalText([]byte(jsRec.Operation))
	if err != nil {
		return opencdc.Record{}, fmt.Errorf("could not unmarshal operation: %w", err)
	}

	meta := s.headersToMetadata(resp.Header)
	maps.Copy(meta, jsRec.Metadata)

	return opencdc.Record{
		Position:  jsRec.Position,
		Operation: op,
		Metadata:  meta,
		Key:       toSDKData(jsRec.Key),
		Payload: opencdc.Change{
			Before: toSDKData(jsRec.Payload.Before),
			After:  toSDKData(jsRec.Payload.After),
		},
	}, nil
}

func (s *Source) parseAsSingleRecord(resp *http.Response, body []byte) opencdc.Record {
	now := time.Now().Unix()
	return opencdc.Record{
		Payload: opencdc.Change{
			Before: nil,
			After:  opencdc.RawData(body),
		},
		Metadata:  s.headersToMetadata(resp.Header),
		Operation: opencdc.OperationCreate,
		Position:  opencdc.Position(fmt.Sprintf("unix-%v", now)),
		Key:       opencdc.RawData(fmt.Sprintf("%v", now)),
	}
}

func (s *Source) headersToMetadata(header http.Header) opencdc.Metadata {
	meta := opencdc.Metadata{}
	for key, val := range header {
		meta[key] = strings.Join(val, ",")
	}
	meta.SetReadAt(time.Now())

	return meta
}
