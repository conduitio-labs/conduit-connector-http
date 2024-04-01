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
	"net/http"
	"testing"

	"github.com/matryer/is"
)

func TestConfig_URL(t *testing.T) {
	is := is.New(t)
	config := Config{
		URL:    "http://localhost:8082/resource",
		Params: "name:resource1,id:1",
	}
	want := "http://localhost:8082/resource?name=resource1&id=1"
	got := config.addParamsToURL()
	is.True(got == want)
}

func TestConfig_Headers(t *testing.T) {
	is := is.New(t)
	config := Config{
		URL:     "http://localhost:8082/resource",
		Headers: []string{"header1:val1", "header2:val2"},
	}
	want := http.Header{}
	want.Add("header1", "val1")
	want.Add("header2", "val2")
	got, err := config.getHeader()
	is.NoErr(err)
	is.True(got.Get("header1") == want.Get("header1"))
	is.True(got.Get("header2") == want.Get("header2"))
}
