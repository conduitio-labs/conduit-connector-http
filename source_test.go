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

	sdk "github.com/conduitio/conduit-connector-sdk"
	"github.com/matryer/is"
)

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
	_, err := createServer()
	is.NoErr(err)
	err = src.Configure(ctx, map[string]string{
		"url":    "http://localhost:8082/resource/resource1",
		"method": "GET",
	})
	is.NoErr(err)
	err = src.Open(ctx, sdk.Position{})
	is.NoErr(err)
	rec, err := src.Read(ctx)
	is.NoErr(err)
	is.True(string(rec.Payload.After.Bytes()) == "This is resource 1")
}

func TestSource_Options(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()
	src := NewSource()
	_, err := createServer()
	is.NoErr(err)
	err = src.Configure(ctx, map[string]string{
		"url":    "http://localhost:8082/resource/resource1",
		"method": "OPTIONS",
	})
	is.NoErr(err)
	err = src.Open(ctx, sdk.Position{})
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
	_, err := createServer()
	is.NoErr(err)
	err = src.Configure(ctx, map[string]string{
		"url":    "http://localhost:8082/resource/",
		"method": "HEAD",
	})
	is.NoErr(err)
	err = src.Open(ctx, sdk.Position{})
	is.NoErr(err)
	_, err = src.Read(ctx)
	is.NoErr(err)
}

// ResourceMap stores resources
var ResourceMap = map[string]string{
	"resource1": "This is resource 1",
	"resource2": "This is resource 2",
}

func createServer() (*http.ServeMux, error) {
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
		ReadTimeout:  10 * time.Second, // Set your desired read timeout
		WriteTimeout: 10 * time.Second, // Set your desired write timeout
	}
	// Start the HTTP server
	go func() {
		err := serverInstance.ListenAndServe()
		if err != nil {
			fmt.Printf("Server error: %s\n", err)
		}
	}()
	return server, nil
}
