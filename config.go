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
	"fmt"
	"net/http"
	"strings"
	"time"

	sdk "github.com/conduitio/conduit-connector-sdk"
)

type SourceConfig struct {
	// Http url to use in the request
	URL string `json:"url" validate:"required"`
	// how often the connector will get data from the url
	PollingPeriod time.Duration `json:"pollingPeriod" default:"5m"`
	// Http method to use in the request
	Method string `default:"GET" validate:"inclusion=GET|POST|PUT|DELETE|PATCH|HEAD|CONNECT|OPTIONS|TRACE"`
	// Http headers to use in the request, comma separated list of : separated pairs
	Headers []string
	// parameters to use in the request, & separated list of = separated pairs
	Params string
	// Http body to use in the request
	Body string

	GetRequestDataScript string `json:"script.getRequestData"`
	// The path to a .js file containing the processor code.
	ParseResponseScript string `json:"script.parseResponse"`
}

func (s SourceConfig) ParseConfig(cfg map[string]string) (SourceConfig, http.Header, error) {
	err := sdk.Util.ParseConfig(cfg, &s)
	if err != nil {
		return SourceConfig{}, nil, fmt.Errorf("invalid config: %w", err)
	}
	header, err := s.parseHeaders()
	if err != nil {
		return SourceConfig{}, nil, fmt.Errorf("invalid config: %w", err)
	}
	if s.Params != "" {
		s.URL = s.URL + "?" + s.Params
	}
	return s, header, nil
}

func (s SourceConfig) parseHeaders() (http.Header, error) {
	// create a new empty header
	header := http.Header{}

	// iterate over the pairs and add them to the header
	for _, pair := range s.Headers {
		// split each pair into key and value
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid headers value: %s", pair)
		}

		// trim any spaces from the key and value
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Add to header
		header.Add(key, value)
	}
	return header, nil
}
