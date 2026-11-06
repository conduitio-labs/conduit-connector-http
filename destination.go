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
	"encoding/base64"
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

	// HTTP method to use in the request
	Method string `default:"POST" validate:"inclusion=POST|PUT|DELETE|PATCH"`

	// RecordHeaders adds the record's identity to every request, so the
	// receiver can tell which record a request is about and what happened to
	// it: the header `X-Conduit-Operation` (create, update, delete or snapshot)
	// and, when the record has a key, `X-Conduit-Key` (the key's bytes, base64
	// encoded; a structured key is its JSON encoding). The request body is not
	// changed, so a delete arrives with an empty body and these headers. Keys
	// appear in request headers, which proxies and servers may log, and a very
	// large key can exceed a server's header size limit. If you turn this off, a
	// record without a payload.after body (a delete) cannot be sent with POST,
	// PUT or PATCH and the write fails instead, because the request would carry
	// neither a body nor a key.
	RecordHeaders bool `json:"recordHeaders" default:"true"`
}

const (
	// HeaderOperation carries the record's operation when RecordHeaders is set.
	HeaderOperation = "X-Conduit-Operation"
	// HeaderKey carries the base64 encoded record key when RecordHeaders is set.
	HeaderKey = "X-Conduit-Key"
)

func (c *DestinationConfig) Validate(ctx context.Context) error {
	var errs []error
	var err error

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
	// A nil or empty After means the record has nothing to send as a body.
	if record.Payload.After != nil && len(record.Payload.After.Bytes()) > 0 {
		body = bytes.NewReader(record.Payload.After.Bytes())
	} else if d.config.Method != http.MethodDelete && !d.config.RecordHeaders {
		// A record without a payload.after body is a delete. Sending it would
		// produce an empty request with no key, which the receiver cannot
		// attribute to any record, so the delete would be lost without an error.
		// Fail instead. A DELETE request has no body by design, and with
		// RecordHeaders the request names the record.
		return fmt.Errorf("record has no payload.after (operation %q) and would be sent as an empty %s "+
			"request that identifies no record: set recordHeaders to true to send its key and operation, "+
			"or drop deletes upstream", record.Operation, d.config.Method)
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
	if d.config.RecordHeaders {
		// Per request copy: d.header is shared by every request.
		req.Header = recordHeaders(d.header, record)
	}

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

// recordHeaders returns a copy of base with the record's operation and key.
func recordHeaders(base http.Header, record opencdc.Record) http.Header {
	h := base.Clone()
	if h == nil {
		h = http.Header{}
	}
	h.Set(HeaderOperation, record.Operation.String())
	h.Del(HeaderKey) // never inherit a configured value for another record's key
	if record.Key != nil {
		h.Set(HeaderKey, base64.StdEncoding.EncodeToString(record.Key.Bytes()))
	}
	return h
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
