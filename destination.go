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
	"net/url"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/conduitio/conduit-commons/config"
	"github.com/conduitio/conduit-commons/opencdc"
	sdk "github.com/conduitio/conduit-connector-sdk"
)

type Destination struct {
	sdk.UnimplementedDestination

	config  DestinationConfig
	client  *http.Client
	header  http.Header
	urlTmpl *template.Template
}

type DestinationConfig struct {
	Config

	// URL is a Go template expression for the URL used in the HTTP request, using Go [templates](https://pkg.go.dev/text/template).
	// The value provided to the template is [opencdc.Record](https://github.com/ConduitIO/conduit-connector-sdk/blob/bfc1d83eb75460564fde8cb4f8f96318f30bd1b4/record.go#L81),
	// so the template has access to all its fields (e.g. .Position, .Key, .Metadata, and so on). We also inject all template functions provided by [sprig](https://masterminds.github.io/sprig/)
	// to make it easier to write templates.
	URL string `json:"url" validate:"required"`
	// Http method to use in the request
	Method string `default:"POST" validate:"inclusion=POST|PUT|DELETE|PATCH"`
}

func NewDestination() sdk.Destination {
	return sdk.DestinationWithMiddleware(&Destination{}, sdk.DefaultDestinationMiddleware()...)
}

func (d *Destination) Parameters() config.Parameters {
	return d.config.Parameters()
}

func (d *Destination) Configure(ctx context.Context, cfg config.Config) error {
	sdk.Logger(ctx).Info().Msg("Configuring Destination...")
	err := sdk.Util.ParseConfig(ctx, cfg, &d.config, d.config.Parameters())
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	d.header, err = d.config.getHeader()
	if err != nil {
		return fmt.Errorf("invalid header config: %w", err)
	}
	if strings.Contains(d.config.URL, "{{") {
		// create URL template
		d.urlTmpl, err = template.New("").Funcs(sprig.FuncMap()).Parse(d.config.URL)
		if err != nil {
			return fmt.Errorf("error while parsing the URL template: %w", err)
		}
	}
	return nil
}

func (d *Destination) Open(ctx context.Context) error {
	// create client
	d.client = &http.Client{}

	// check connection
	if d.config.ValidateConnection {
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
		if resp.StatusCode >= 400 {
			return fmt.Errorf("invalid response status code: (%d) %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}
	}

	return nil
}

func (d *Destination) Write(ctx context.Context, records []opencdc.Record) (int, error) {
	for i, rec := range records {
		err := d.sendRequest(ctx, rec)
		if err != nil {
			return i, err
		}
	}
	return len(records), nil
}

func (d *Destination) getURL(rec opencdc.Record) (string, error) {
	URL, err := d.EvaluateURL(rec)
	if err != nil {
		return "", err
	}
	URL, err = d.config.addParamsToURL(URL)
	if err != nil {
		return "", err
	}
	return URL, nil
}

func (d *Destination) EvaluateURL(rec opencdc.Record) (string, error) {
	if d.urlTmpl == nil {
		return d.config.URL, nil
	}
	var b bytes.Buffer
	err := d.urlTmpl.Execute(&b, rec)
	if err != nil {
		return "", fmt.Errorf("error while evaluating URL template: %w", err)
	}
	u, err := url.Parse(b.String())
	if err != nil {
		return "", fmt.Errorf("error parsing URL: %w", err)
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", fmt.Errorf("error parsing URL query: %w", err)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (d *Destination) sendRequest(ctx context.Context, record opencdc.Record) error {
	var body io.Reader
	if record.Payload.After != nil {
		body = bytes.NewReader(record.Payload.After.Bytes())
	}
	URL, err := d.getURL(record)
	if err != nil {
		return err
	}
	// create request
	req, err := http.NewRequestWithContext(ctx, d.config.Method, URL, body)
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
