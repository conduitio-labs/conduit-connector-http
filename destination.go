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

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
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

func (d *Destination) Config() sdk.DestinationConfig {
	return &d.config
}

type DestinationConfig struct {
	sdk.DefaultDestinationMiddleware

	Config

	// URL is a Go template expression for the URL used in the HTTP request, using Go [templates](https://pkg.go.dev/text/template).
	// The value provided to the template is [opencdc.Record](https://conduit.io/docs/using/opencdc-record),
	// so the template has access to all its fields (e.g. .Position, .Key, .Metadata, and so on). We also inject all template functions provided by [sprig](https://masterminds.github.io/sprig/)
	// to make it easier to write templates.
	URL string `json:"url" validate:"required"`

	// Http method to use in the request
	Method string `default:"POST" validate:"inclusion=POST|PUT|DELETE|PATCH"`
}

func (c *DestinationConfig) Validate(ctx context.Context) error {
	var errs []error
	var err error

	if err = c.Config.Validate(ctx); err != nil {
		errs = append(errs, err)
	}

	if err = c.DefaultDestinationMiddleware.Validate(ctx); err != nil {
		errs = append(errs, err)
	}

	// Custom validations
	_, err = c.getHeader()
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid header config: %w", err))
	}

	if c.hasURLTemplate() {
		_, err = template.New("").Funcs(sprig.FuncMap()).Parse(c.URL)
		if err != nil {
			errs = append(errs, fmt.Errorf("error while parsing the URL template: %w", err))
		}
	}

	return errors.Join(errs...)
}

func NewDestination() sdk.Destination {
	return sdk.DestinationWithMiddleware(&Destination{})
}

func (d *Destination) Open(ctx context.Context) error {
	d.client = &http.Client{}

	// ignore errors (these were already validated in Validate())
	d.header, _ = d.config.getHeader()

	if d.config.hasURLTemplate() {
		d.urlTmpl, _ = template.New("").Funcs(sprig.FuncMap()).Parse(d.config.URL)
	}

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
	if resp.StatusCode >= 400 {
		return fmt.Errorf("invalid response status code: (%d) %s", resp.StatusCode, http.StatusText(resp.StatusCode))
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

func (c *DestinationConfig) hasURLTemplate() bool {
	return strings.Contains(c.URL, "{{") || strings.Contains(c.URL, "}}")
}
