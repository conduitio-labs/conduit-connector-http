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
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Config struct {
	// Http headers to use in the request, comma separated list of : separated pairs
	Headers []string
	// parameters to use in the request, use params.* as the config key and specify its value, ex: set "params.id" as "1".
	Params map[string]string
	// ValidateConnection sends a HEAD request when opening the connector to check if the connection works.
	ValidateConnection bool `json:"validateConnection" default:"true"`
}

func (c *Config) addParamsToURL(origURL string) (string, error) {
	parsedURL, err := url.Parse(origURL)
	if err != nil {
		return "", fmt.Errorf("error parsing URL: %w", err)
	}
	// Parse existing query parameters
	existingParams := parsedURL.Query()
	// Add config params
	for key, val := range c.Params {
		existingParams.Add(key, val)
	}
	// Update query parameters in the URL struct
	parsedURL.RawQuery = existingParams.Encode()

	return parsedURL.String(), nil
}

func (c *Config) getHeader() (http.Header, error) {
	// create a new empty header
	header := http.Header{}

	// iterate over the pairs and add them to the header
	for _, pair := range c.Headers {
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
