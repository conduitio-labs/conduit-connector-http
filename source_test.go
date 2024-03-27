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
	"github.com/matryer/is"
	"testing"
)

func TestTeardownSource_NoOpen(t *testing.T) {
	con := NewSource()
	err := con.Teardown(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestSource_Read_Gmail(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()

	cfg := map[string]string{
		"url": "https://gmail.googleapis.com/gmail/v1/users/muslim156@gmail.com/messages",
		"script.getRequestData": `
function getRequestData(cfg) {
	let data = new RequestData()
	data.URL = "https://gmail.googleapis.com/gmail/v1/users/muslim156@gmail.com/messages?access_token=ya29.a0Ad52N38gL8W075jtDXyJP3ADp57Eoq-VkwQDi4dqw-dHPWdwuWmk5R-lYtLJWZJ_MP0z9UVpEl9g05ZS0KpyWvbt2R4dSRbBY1ny9h5x6v6PwtIUU9XVTg4K6OJiVkZUYa1yi5fBn0uH80_VSOVNe5xkhEpHv-ueGNclaCgYKASMSARMSFQHGX2MifgkXdC1HwfewF47GPkawag0171"
	return data
}`,
	}

	conn := NewSource()
	is.NoErr(conn.Configure(ctx, cfg))
	is.NoErr(conn.Open(ctx, nil))
	_, err := conn.Read(ctx)
	is.NoErr(err)
}
