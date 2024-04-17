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
	"testing"

	sdk "github.com/conduitio/conduit-connector-sdk"
	"github.com/google/go-cmp/cmp"
	"github.com/matryer/is"
	"github.com/rs/zerolog"
)

func TestSourceExtension_GetRequestData(t *testing.T) {
	is := is.New(t)
	ctx := zerolog.New(zerolog.NewTestWriter(t)).WithContext(context.Background())

	underTest, err := newJSRequestBuilder(
		ctx,
		map[string]string{
			"url": "http://example.com",
		},
		"./test/get_request_data.js",
	)
	is.NoErr(err)

	data, err := underTest.build(
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
	ctx := zerolog.New(zerolog.NewTestWriter(t)).WithContext(context.Background())

	underTest, err := newJSResponseParser(ctx, "./test/parse_response.js")
	is.NoErr(err)

	resp, err := underTest.parse([]byte(`{
	"nextSyncToken": "xyz",
	"some_objects": [
		{
			"id": "id-a",
			"action": "update",
			"field_a": "value_a"
		},
		{
			"id": "id-b",
			"field_b": "value_b",
			"field_c": "value_c"
		}
	]
}`))
	is.NoErr(err)

	diff := cmp.Diff(
		resp.Records,
		[]*jsRecord{
			{
				Position:  []byte("xyz"),
				Key:       sdk.RawData("id-a"),
				Metadata:  make(map[string]string),
				Operation: "update",
				Payload: jsPayload{
					After: map[string]any{"field_a": "value_a"},
				},
			},
			{
				Position: []byte("xyz"),
				Key:      sdk.RawData("id-b"),
				Metadata: make(map[string]string),
				Payload: jsPayload{
					After: map[string]any{
						"field_b": "value_b",
						"field_c": "value_c",
					},
				},
			},
		},
	)
	is.Equal("", diff)
}
