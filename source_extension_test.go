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
	"context"
	sdk "github.com/conduitio/conduit-connector-sdk"
	"github.com/google/go-cmp/cmp"
	"testing"

	"github.com/matryer/is"
)

func TestSourceExtension_GetRequestData(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()

	underTest := newSourceExtension()
	err := underTest.configure(
		"./test/get_request_data.js",
		"./test/parse_response.js",
	)
	is.NoErr(err)

	err = underTest.open(ctx)
	is.NoErr(err)

	data, err := underTest.getRequestData(
		SourceConfig{
			Config: Config{URL: "http://example.com"},
		},
		map[string]any{
			"nextPageToken": "abc",
		},
		sdk.Position(""),
	)
	is.NoErr(err)
	is.Equal("http://example.com/?pageToken=abc&pageSize=2", data.URL)
}

func TestSourceExtension_ParseResponse(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()

	underTest := newSourceExtension()
	err := underTest.configure(
		"./test/get_request_data.js",
		"./test/parse_response.js",
	)
	is.NoErr(err)

	err = underTest.open(ctx)
	is.NoErr(err)

	resp, err := underTest.parseResponseData([]byte(`{
	"nextSyncToken": "xyz",
	"some_objects": [
		{
			"field_a": "value_a"
		},
		{
			"field_b": "value_b"
		}
	]
}`))
	is.NoErr(err)

	diff := cmp.Diff(
		resp.Records,
		[]*jsRecord{
			{
				Position: []byte("xyz"),
				Metadata: make(map[string]string),
				Payload: jsPayload{
					After: sdk.RawData(`{"field_a":"value_a"}`),
				},
			},
			{
				Position: []byte("xyz"),
				Metadata: make(map[string]string),
				Payload: jsPayload{
					After: sdk.RawData(`{"field_b":"value_b"}`),
				},
			},
		},
	)
	is.Equal("", diff)
}
