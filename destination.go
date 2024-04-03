// Copyright © 2024 Meroxa, Inc.
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

//go:generate paramgen -output=paramgen_dest.go DestinationConfig

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	sdk "github.com/conduitio/conduit-connector-sdk"
)

type Destination struct {
	sdk.UnimplementedDestination

	config DestinationConfig
	client *http.Client
	header http.Header
}

type DestinationConfig struct {
	Config

	// Http method to use in the request
	Method string `default:"POST" validate:"inclusion=POST|PUT|DELETE|PATCH"`
}

func NewDestination() sdk.Destination {
	return sdk.DestinationWithMiddleware(&Destination{}, sdk.DefaultDestinationMiddleware()...)
}

func (d *Destination) Parameters() map[string]sdk.Parameter {
	return d.config.Parameters()
}

func (d *Destination) Configure(ctx context.Context, cfg map[string]string) error {
	sdk.Logger(ctx).Info().Msg("Configuring Destination...")
	var config DestinationConfig
	err := sdk.Util.ParseConfig(cfg, &config)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	d.config.URL, err = d.config.addParamsToURL()
	if err != nil {
		return err
	}
	d.header, err = config.Config.getHeader()
	if err != nil {
		return fmt.Errorf("invalid header config: %w", err)
	}
	d.config = config
	return nil
}

func (d *Destination) Open(ctx context.Context) error {
	// create client
	d.client = &http.Client{}

	// check connection
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, d.config.URL, nil)
	if err != nil {
		return fmt.Errorf("error creating HTTP request %q: %w", d.config.URL, err)
	}
	req.Header = d.header
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("error pinging URL %q: %w", d.config.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		return fmt.Errorf("authorization failed, %s: %s", http.StatusText(http.StatusUnauthorized), string(body))
	}

	return nil
}

func (d *Destination) Write(ctx context.Context, records []sdk.Record) (int, error) {
	for i, rec := range records {
		err := d.sendRequest(ctx, rec)
		if err != nil {
			return i, err
		}
	}
	return 0, nil
}

func (d *Destination) sendRequest(ctx context.Context, record sdk.Record) error {
	var body io.Reader
	if record.Payload.After != nil {
		body = bytes.NewReader(record.Payload.After.Bytes())
	}

	// create request
	req, err := http.NewRequestWithContext(ctx, d.config.Method, d.config.URL, body)
	if err != nil {
		return fmt.Errorf("error creating HTTP %s request: %w", d.config.Method, err)
	}
	req.Header = d.header

	// get response
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("error getting data from URL: %w", err)
	}
	defer resp.Body.Close()
	// check if response status is an error code
	if resp.StatusCode >= 400 {
		return fmt.Errorf("got an unexpected response status of %q", resp.Status)
	}
	return nil
}

func (d *Destination) Teardown(ctx context.Context) error {
	if d.client != nil {
		d.client.CloseIdleConnections()
	}
	return nil
}
