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
	"encoding/json"
	"fmt"
	sdk "github.com/conduitio/conduit-connector-sdk"
	"github.com/matryer/is"
	"github.com/rs/zerolog"
	"os"
	"testing"
)

func TestTeardownSource_NoOpen(t *testing.T) {
	con := NewSource()
	err := con.Teardown(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestSource_Read_Connections(t *testing.T) {
	is := is.New(t)
	ctx := zerolog.New(zerolog.NewTestWriter(t)).WithContext(context.Background())

	tokenBytes, err := os.ReadFile("/home/haris/GolandProjects/go-playground/token.json")
	is.NoErr(err)

	tokenMap := map[string]string{}
	is.NoErr(json.Unmarshal(tokenBytes, &tokenMap))

	token := tokenMap["access_token"]

	cfg := map[string]string{
		"url":                   "https://gmail.googleapis.com/gmail/v1/users/muslim156@gmail.com/messages",
		"headers":               "Authorization: Bearer " + token,
		"script.getRequestData": "get_request_data.js",
		"script.parseResponse":  "parse_response.js",
	}

	conn := NewSource()
	is.NoErr(conn.Configure(ctx, cfg))
	is.NoErr(conn.Open(ctx, sdk.Position("MisA-QgvlQAAABII9efX7KOZhQMQ9efX7KOZhQMyfNvrQeYCCHBEVP_9sTW5OiQ2NjA0ODM0ZS0wMDAwLTIyNDQtOWMzMy1hYzNlYjE0Mjk0NzQ=")))

	for i := 0; i < 10; i++ {
		rec, err := conn.Read(ctx)
		is.NoErr(err)
		fmt.Println("got record with position: " + string(rec.Position))
		fmt.Println(string(rec.Payload.After.Bytes()))
	}

	is.NoErr(conn.Teardown(ctx))
}
