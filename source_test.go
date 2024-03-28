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
	token := "ya29.a0Ad52N38SR_KyZk_B0D1cmBo_Afb8OU_MA19aLii8ZSi5BD4zPmM7ZVvTjP25jqaJmrYFfDSDX6kqBmWwbonoTyILPf9hwCDq3xD-Wy1-Yx8zFaG-TtORtv2uqz3ceWJhfXAvGTaNu8afKxrRM4C0-WfGy2GL0FB6K-ybaCgYKAVYSARMSFQHGX2MiOBgz3Iu8279vZMJisK_yqg0171"

	getRequestScript, err := os.ReadFile("get_request_data.js")
	is.NoErr(err)

	parseResponseScript, err := os.ReadFile("parse_response.js")
	is.NoErr(err)

	cfg := map[string]string{
		"url":                   "https://gmail.googleapis.com/gmail/v1/users/muslim156@gmail.com/messages",
		"headers":               "Authorization: Bearer " + token,
		"script.getRequestData": string(getRequestScript),
		"script.parseResponse":  string(parseResponseScript),
	}

	conn := NewSource()
	is.NoErr(conn.Configure(ctx, cfg))
	is.NoErr(conn.Open(ctx, sdk.Position("MisA-QgvlQAAABII0vyq5K-XhQMQ0vyq5K-XhQPJ55wmBJDhWn3r0s5QoqQNOiQ2NjA0N2U3MC0wMDAwLTJkYTMtYmU4Ny0wMDFhMTE0ZDM1Yjg=")))

	for i := 0; i < 10; i++ {
		rec, err := conn.Read(ctx)
		is.NoErr(err)
		fmt.Println("got position: " + string(rec.Position))
	}

	is.NoErr(conn.Teardown(ctx))
}
