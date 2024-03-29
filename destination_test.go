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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	sdk "github.com/conduitio/conduit-connector-sdk"
	"github.com/matryer/is"
)

var serverRunning bool

func TestMain(m *testing.M) {
	runServer()
	os.Exit(m.Run())
}

func TestTeardown_NoOpen(t *testing.T) {
	con := NewDestination()
	err := con.Teardown(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDestination_Post(t *testing.T) {
	is := is.New(t)
	runServer()
	url := "http://localhost:8081/resource"
	ctx := context.Background()
	dest := NewDestination()
	err := dest.Configure(ctx, map[string]string{
		"url":    url,
		"method": "POST",
	})
	is.NoErr(err)
	err = dest.Open(ctx)
	is.NoErr(err)
	rec := sdk.Record{
		Payload: sdk.Change{
			After: sdk.RawData(`{"id": "2", "name": "Item 2"}`),
		},
	}
	_, err = dest.Write(ctx, []sdk.Record{rec})
	_, ok := resources["2"]
	is.True(ok)
	is.True(resources["2"].Name == "Item 2")
}

func TestDestination_Delete(t *testing.T) {
	is := is.New(t)
	runServer()
	url := "http://localhost:8081/resource/1"
	ctx := context.Background()
	dest := NewDestination()
	err := dest.Configure(ctx, map[string]string{
		"url":    url,
		"method": "DELETE",
	})
	is.NoErr(err)
	err = dest.Open(ctx)
	is.NoErr(err)
	rec := sdk.Record{}
	_, err = dest.Write(ctx, []sdk.Record{rec})
	_, ok := resources["1"]
	// resource was deleted
	is.True(!ok)
}

// resource represents a dummy resource
type resource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// init with one resource
var resources = map[string]resource{
	"1": {ID: "1", Name: "Item 1"},
}

func runServer() {
	if serverRunning {
		return
	}
	serverRunning = true
	address := ":8081"

	http.HandleFunc("/resource", handleResource)
	http.HandleFunc("/resource/", handleSingleResource)

	go func() {
		err := http.ListenAndServe(address, nil)
		if err != nil {
			fmt.Printf("Server error: %s\n", err)
		}
	}()
}

// handleResource handles POST requests to create a new resource
func handleResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var newResource resource
	err := json.NewDecoder(r.Body).Decode(&newResource)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	resources[newResource.ID] = newResource
	w.WriteHeader(http.StatusCreated)
}

// handleSingleResource handles DELETE, PATCH, and PUT requests for a single resource
func handleSingleResource(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/resource/"):]

	switch r.Method {
	case http.MethodDelete:
		delete(resources, id)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPatch:
		var updatedResource resource
		err := json.NewDecoder(r.Body).Decode(&updatedResource)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		resources[id] = updatedResource
		w.WriteHeader(http.StatusOK)
	case http.MethodPut:
		var newResource resource
		err := json.NewDecoder(r.Body).Decode(&newResource)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		resources[id] = newResource
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
