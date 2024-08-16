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
	"net/http"
	"testing"
	"time"

	"github.com/conduitio/conduit-commons/opencdc"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/matryer/is"
	"go.uber.org/mock/gomock"
)

// ResourceMap stores resources
var ResourceMap = map[string]string{
	"resource1": "This is resource 1",
	"resource2": "This is resource 2",
}

// createServer creates an HTTP server.
// Returns a function that shuts down the server.
func createServer(t *testing.T) {
	// Define the server address
	address := ":8082"

	// Create a new HTTP server
	server := http.NewServeMux()

	// Handler for GET requests
	server.HandleFunc("/resource/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Extract resource name from URL
			resourceName := r.URL.Path[len("/resource/"):]
			resource, found := ResourceMap[resourceName]
			if !found {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			// Return the resource
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", resource)
		case http.MethodHead:
			// Respond with headers only
			w.WriteHeader(http.StatusOK)
		case http.MethodOptions:
			// Respond with allowed methods
			w.Header().Set("Allow", "GET, HEAD, OPTIONS")
			w.WriteHeader(http.StatusOK)
		default:
			// Method not allowed
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	serverInstance := &http.Server{
		Addr:         address,
		Handler:      server,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start the HTTP server
	go func() {
		err := serverInstance.ListenAndServe()
		if err != nil {
			fmt.Printf("Server error: %s\n", err)
		}
	}()

	t.Cleanup(func() {
		err := serverInstance.Shutdown(context.Background())
		if err != nil {
			fmt.Printf("Server error: %s\n", err)
		}
	})
}

func TestTeardownSource_NoOpen(t *testing.T) {
	con := NewSource()
	err := con.Teardown(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestSource_Get(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	src := NewSource()
	createServer(t)

	err := src.Configure(ctx, map[string]string{
		"url":    "http://localhost:8082/resource/resource1",
		"method": "GET",
	})
	is.NoErr(err)

	err = src.Open(ctx, opencdc.Position{})
	is.NoErr(err)

	rec, err := src.Read(ctx)
	is.NoErr(err)
	is.True(string(rec.Payload.After.Bytes()) == "This is resource 1")
}

func TestSource_Options(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	src := NewSource()
	createServer(t)

	err := src.Configure(ctx, map[string]string{
		"url":    "http://localhost:8082/resource/resource1",
		"method": "OPTIONS",
	})
	is.NoErr(err)

	err = src.Open(ctx, opencdc.Position{})
	is.NoErr(err)

	rec, err := src.Read(ctx)
	is.NoErr(err)
	meta, ok := rec.Metadata["Allow"]
	is.True(ok)
	is.Equal(meta, "GET, HEAD, OPTIONS")
}

func TestSource_Head(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	src := NewSource()
	createServer(t)

	err := src.Configure(ctx, map[string]string{
		"url":    "http://localhost:8082/resource/",
		"method": "HEAD",
	})
	is.NoErr(err)

	err = src.Open(ctx, opencdc.Position{})
	is.NoErr(err)

	_, err = src.Read(ctx)
	is.NoErr(err)
}

func TestSource_ConfigureWithScripts(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()

	src := NewSource().(*Source)
	cfg := map[string]string{
		"url":                   "http://localhost:8082/resource/default-resource",
		"method":                "GET",
		"script.getRequestData": "./test/get_request_data.js",
		"script.parseResponse":  "./test/parse_response.js",
	}

	createServer(t)

	err := src.Configure(ctx, cfg)
	is.NoErr(err)

	err = src.Open(ctx, nil)
	is.NoErr(err)

	is.True(src.requestBuilder != nil)
	is.True(src.responseParser != nil)
}

func TestSource_CustomRequest(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()

	src := NewSource().(*Source)
	cfg := map[string]string{
		"url":    "http://localhost:8082/resource/default-resource",
		"method": "GET",
	}
	var previousResp map[string]interface{}
	pos := opencdc.Position("test-position")

	rb := NewMockRequestBuilder(gomock.NewController(t))
	rb.EXPECT().
		build(ctx, previousResp, pos).
		Return(&Request{URL: "http://localhost:8082/resource/resource1"}, nil)
	src.requestBuilder = rb

	createServer(t)

	err := src.Configure(ctx, cfg)
	is.NoErr(err)

	err = src.Open(ctx, pos)
	is.NoErr(err)

	rec, err := src.Read(ctx)
	is.NoErr(err)
	is.True(string(rec.Payload.After.Bytes()) == "This is resource 1")
}

func TestSource_ParseResponse(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()

	src := NewSource().(*Source)
	cfg := map[string]string{
		"url":    "http://localhost:8082/resource/resource1",
		"method": "GET",
	}
	want := opencdc.Record{
		Position:  opencdc.Position("pagination-token"),
		Operation: opencdc.OperationUpdate,
		Metadata:  map[string]string{"foo": "bar"},
		Key:       opencdc.RawData("record-key"),
		Payload: opencdc.Change{
			After: opencdc.StructuredData{
				"field-a": "value-a",
			},
		},
	}

	rp := NewMockResponseParser(gomock.NewController(t))
	rp.EXPECT().
		parse(ctx, []byte("This is resource 1")).
		Return(
			&Response{Records: []*jsRecord{{
				Position:  []byte("pagination-token"),
				Operation: "update",
				Metadata:  map[string]string{"foo": "bar"},
				Key:       opencdc.RawData("record-key"),
				Payload: jsPayload{
					After: map[string]interface{}{
						"field-a": "value-a",
					},
				},
			}}},
			nil,
		)
	src.responseParser = rp

	createServer(t)

	err := src.Configure(ctx, cfg)
	is.NoErr(err)

	err = src.Open(ctx, nil)
	is.NoErr(err)

	got, err := src.Read(ctx)
	is.NoErr(err)
	want.Metadata["Content-Length"] = got.Metadata["Content-Length"]
	want.Metadata["Content-Type"] = got.Metadata["Content-Type"]
	want.Metadata["Date"] = got.Metadata["Date"]
	want.Metadata["opencdc.readAt"] = got.Metadata["opencdc.readAt"]

	diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(opencdc.Record{}))
	if diff != "" {
		t.Errorf("mismatch (-want +got): %s", diff)
	}
}
