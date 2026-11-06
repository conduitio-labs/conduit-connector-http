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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/conduitio/conduit-commons/opencdc"
	sdk "github.com/conduitio/conduit-connector-sdk"
	"github.com/matryer/is"
)

var (
	server        *http.Server
	serverRunning bool
)

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

	err := sdk.Util.ParseConfig(ctx,
		map[string]string{
			"url":    url,
			"method": "POST",
		}, dest.Config(),
		Connector.NewSpecification().DestinationParams,
	)
	is.NoErr(err)

	err = dest.Open(ctx)
	is.NoErr(err)
	rec := opencdc.Record{
		Payload: opencdc.Change{
			After: opencdc.RawData(`{"id": "2", "name": "Item 2"}`),
		},
	}
	records, err := dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)
	is.Equal(records, 1)
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

	err := sdk.Util.ParseConfig(ctx,
		map[string]string{
			"url":    url,
			"method": "DELETE",
		}, dest.Config(),
		Connector.NewSpecification().DestinationParams,
	)
	is.NoErr(err)

	err = dest.Open(ctx)
	is.NoErr(err)
	rec := opencdc.Record{}
	records, err := dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)
	is.Equal(records, 1)
	_, ok := resources["1"]
	// resource was deleted
	is.True(!ok)
}

func TestDestination_ValidateConnection(t *testing.T) {
	is := is.New(t)
	// unused url
	url := "http://localhost:8082/resource/1"
	ctx := context.Background()
	dest := NewDestination()
	err := sdk.Util.ParseConfig(ctx,
		map[string]string{
			"url":                url,
			"method":             "DELETE",
			"validateConnection": "false", // don't validate connection
		}, dest.Config(),
		Connector.NewSpecification().DestinationParams,
	)
	is.NoErr(err)
	err = dest.Open(ctx)
	is.NoErr(err) // HEAD request won't be called
}

func TestDestination_DynamicURL(t *testing.T) {
	is := is.New(t)
	runServer()
	url := "http://localhost:8081/resource/{{.Payload.After.id}}"
	ctx := context.Background()
	dest := NewDestination()

	err := sdk.Util.ParseConfig(ctx,
		map[string]string{
			"url":    url,
			"method": "DELETE",
		}, dest.Config(),
		Connector.NewSpecification().DestinationParams,
	)
	is.NoErr(err)

	rec := opencdc.Record{
		Payload: opencdc.Change{
			After: opencdc.StructuredData{
				"id": "3",
			},
		},
	}
	err = dest.Open(ctx)
	is.NoErr(err)
	records, err := dest.Write(ctx, []opencdc.Record{rec})
	is.NoErr(err)
	is.Equal(records, 1)
	_, ok := resources["3"]
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
	"3": {ID: "3", Name: "Item 3"},
}

func runServer() {
	if serverRunning {
		return
	}
	serverRunning = true
	address := ":8081"

	mux := http.NewServeMux()
	mux.HandleFunc("/resource", handleResource)
	mux.HandleFunc("/resource/", handleSingleResource)

	server = &http.Server{
		Addr:         address,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			fmt.Printf("Server error: %s\n", err)
		}
	}()
}

// handleResource handles POST requests to create a new resource
func handleResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Method == http.MethodHead {
		// Respond with headers only
		w.WriteHeader(http.StatusOK)
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
	case http.MethodHead:
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// capture records every request an httptest server receives.
type capture struct {
	mu   sync.Mutex
	reqs []capturedRequest
}

type capturedRequest struct {
	method string
	header http.Header
	body   string
}

func (c *capture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	c.mu.Lock()
	c.reqs = append(c.reqs, capturedRequest{method: r.Method, header: r.Header.Clone(), body: string(b)})
	c.mu.Unlock()
}

func (c *capture) requests() []capturedRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]capturedRequest(nil), c.reqs...)
}

func openCaptureDestination(t *testing.T, cfg map[string]string) (*Destination, *capture) {
	t.Helper()
	is := is.New(t)
	rx := &capture{}
	srv := httptest.NewServer(rx)
	t.Cleanup(srv.Close)
	cfg["url"] = srv.URL
	cfg["validateConnection"] = "false"
	ctx := context.Background()
	dest := &Destination{}
	is.NoErr(sdk.Util.ParseConfig(ctx, cfg, dest.Config(), Connector.NewSpecification().DestinationParams))
	is.NoErr(dest.Open(ctx))
	t.Cleanup(func() { _ = dest.Teardown(ctx) })
	return dest, rx
}

func deleteRecord() opencdc.Record {
	return opencdc.Record{
		Operation: opencdc.OperationDelete,
		Key:       opencdc.StructuredData{"id": float64(42)},
		Payload:   opencdc.Change{Before: opencdc.StructuredData{"id": float64(42), "name": "gone"}},
	}
}

func writeOne(t *testing.T, dest *Destination, rec opencdc.Record) (int, error) {
	t.Helper()
	return dest.Write(context.Background(), []opencdc.Record{rec})
}

// Regression: a delete used to be POSTed with an empty body and no key, so the
// receiver got a request it could not attribute to any record, and no error.
// With the default (recordHeaders on) the delete is sent and carries its key.
func TestDestination_DefaultDeleteCarriesKeyAndOperation(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH"} {
		t.Run(method, func(t *testing.T) {
			is := is.New(t)
			dest, rx := openCaptureDestination(t, map[string]string{"method": method})
			rec := deleteRecord()
			n, err := writeOne(t, dest, rec)
			is.NoErr(err)
			is.Equal(n, 1)

			reqs := rx.requests()
			is.Equal(len(reqs), 1)
			is.Equal(reqs[0].method, method)
			is.Equal(reqs[0].body, "")
			is.Equal(reqs[0].header.Get(HeaderOperation), "delete")
			key, err := base64.StdEncoding.DecodeString(reqs[0].header.Get(HeaderKey))
			is.NoErr(err)
			is.Equal(string(key), string(rec.Key.Bytes()))
			is.Equal(string(key), `{"id":42}`)
		})
	}
}

func TestDestination_DefaultHeadersOnInsertUpdateDelete(t *testing.T) {
	is := is.New(t)
	dest, rx := openCaptureDestination(t, map[string]string{"method": "POST", "headers": "X-Static: yes"})
	recs := []opencdc.Record{
		{Operation: opencdc.OperationCreate, Key: opencdc.RawData("k1"), Payload: opencdc.Change{After: opencdc.RawData(`{"a":1}`)}},
		{Operation: opencdc.OperationUpdate, Payload: opencdc.Change{After: opencdc.RawData(`{"a":2}`)}}, // no key
		{Operation: opencdc.OperationSnapshot, Key: opencdc.RawData("k3"), Payload: opencdc.Change{After: opencdc.RawData(`{"a":3}`)}},
		deleteRecord(),
	}
	n, err := dest.Write(context.Background(), recs)
	is.NoErr(err)
	is.Equal(n, 4)

	reqs := rx.requests()
	is.Equal(len(reqs), 4)
	want := []struct{ op, key, body string }{
		{"create", base64.StdEncoding.EncodeToString([]byte("k1")), `{"a":1}`},
		{"update", "", `{"a":2}`}, // a key-less record must not inherit the previous key
		{"snapshot", base64.StdEncoding.EncodeToString([]byte("k3")), `{"a":3}`},
		{"delete", base64.StdEncoding.EncodeToString([]byte(`{"id":42}`)), ""},
	}
	for i, w := range want {
		is.Equal(reqs[i].header.Get(HeaderOperation), w.op)
		is.Equal(reqs[i].header.Get(HeaderKey), w.key)
		is.Equal(reqs[i].body, w.body)
		is.Equal(reqs[i].header.Get("X-Static"), "yes")
	}
	// The configured header map is shared by every request and must stay clean.
	is.Equal(dest.header.Get(HeaderOperation), "")
	is.Equal(dest.header.Get(HeaderKey), "")
}

// A statically configured X-Conduit-Key must not be sent for a record that has no
// key: the receiver would attribute the record to the configured value.
func TestDestination_StaticKeyHeaderNotSentForKeylessRecord(t *testing.T) {
	is := is.New(t)
	dest, rx := openCaptureDestination(t, map[string]string{"method": "POST", "headers": HeaderKey + ": static"})
	rec := opencdc.Record{Operation: opencdc.OperationUpdate, Payload: opencdc.Change{After: opencdc.RawData(`{"a":1}`)}}
	_, err := writeOne(t, dest, rec)
	is.NoErr(err)
	reqs := rx.requests()
	is.Equal(len(reqs), 1)
	_, hasKey := reqs[0].header[HeaderKey]
	is.True(!hasKey)
}

// "No key" is unambiguous: nil, empty raw and typed-nil structured keys all send no
// X-Conduit-Key header (not an empty value, not base64 of "null").
func TestDestination_NoKeyShapesSendNoKeyHeader(t *testing.T) {
	var nilStruct opencdc.StructuredData
	cases := map[string]opencdc.Data{
		"nil key":               nil,
		"empty raw key":         opencdc.RawData(nil),
		"empty non-nil raw key": opencdc.RawData([]byte{}),
		"typed-nil structured":  nilStruct,
		"empty structured key":  opencdc.StructuredData{},
	}
	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			is := is.New(t)
			dest, rx := openCaptureDestination(t, map[string]string{"method": "POST"})
			rec := opencdc.Record{Operation: opencdc.OperationUpdate, Key: key,
				Payload: opencdc.Change{After: opencdc.RawData(`{"a":1}`)}}
			_, err := writeOne(t, dest, rec)
			is.NoErr(err)
			reqs := rx.requests()
			is.Equal(len(reqs), 1)
			_, hasKey := reqs[0].header[HeaderKey]
			is.True(!hasKey)
			is.Equal(reqs[0].header.Get(HeaderOperation), "update")
		})
	}
}

// With recordHeaders explicitly off, a record without a body is never sent.
func TestDestination_ExplicitFalseRejectsBodilessRecords(t *testing.T) {
	cases := map[string]opencdc.Record{
		"delete (no After)": deleteRecord(),
		"empty raw After": {
			Operation: opencdc.OperationUpdate, Key: opencdc.RawData("k"),
			Payload: opencdc.Change{After: opencdc.RawData(nil)},
		},
		"empty non-nil raw After": {
			Operation: opencdc.OperationUpdate, Key: opencdc.RawData("k"),
			Payload: opencdc.Change{After: opencdc.RawData([]byte{})},
		},
	}
	for _, method := range []string{"POST", "PUT", "PATCH"} {
		for name, rec := range cases {
			t.Run(method+"/"+name, func(t *testing.T) {
				is := is.New(t)
				dest, rx := openCaptureDestination(t, map[string]string{"method": method, "recordHeaders": "false"})
				n, err := writeOne(t, dest, rec)
				is.True(err != nil)
				is.True(strings.Contains(err.Error(), "recordHeaders"))
				is.Equal(n, 0)
				is.Equal(len(rx.requests()), 0) // nothing reached the endpoint
			})
		}
	}
}

// With recordHeaders explicitly off and a body, requests are as before.
func TestDestination_ExplicitFalseSendsNoHeadersForRecordsWithBody(t *testing.T) {
	is := is.New(t)
	dest, rx := openCaptureDestination(t, map[string]string{"method": "POST", "recordHeaders": "false"})
	rec := opencdc.Record{Operation: opencdc.OperationCreate, Key: opencdc.RawData("k"),
		Payload: opencdc.Change{After: opencdc.RawData(`{"a":1}`)}}
	_, err := writeOne(t, dest, rec)
	is.NoErr(err)
	reqs := rx.requests()
	is.Equal(len(reqs), 1)
	is.Equal(reqs[0].body, `{"a":1}`)
	_, hasOp := reqs[0].header[HeaderOperation]
	_, hasKey := reqs[0].header[HeaderKey]
	is.True(!hasOp && !hasKey)
}

// With the default on, an empty (non-nil) After is sent like a delete: headers, no body.
func TestDestination_DefaultEmptyAfterIsSentWithHeaders(t *testing.T) {
	is := is.New(t)
	dest, rx := openCaptureDestination(t, map[string]string{"method": "PUT"})
	rec := opencdc.Record{Operation: opencdc.OperationUpdate, Key: opencdc.RawData("k"),
		Payload: opencdc.Change{After: opencdc.RawData([]byte{})}}
	_, err := writeOne(t, dest, rec)
	is.NoErr(err)
	reqs := rx.requests()
	is.Equal(len(reqs), 1)
	is.Equal(reqs[0].body, "")
	is.Equal(reqs[0].header.Get(HeaderOperation), "update")
	is.Equal(reqs[0].header.Get(HeaderKey), base64.StdEncoding.EncodeToString([]byte("k")))
}

// A DELETE-method destination still sends a bodiless request for a record without
// After: that is its purpose (TestDestination_Delete), with or without the headers.
func TestDestination_DeleteMethodAllowsBodilessRecords(t *testing.T) {
	for _, rh := range []string{"true", "false"} {
		t.Run("recordHeaders="+rh, func(t *testing.T) {
			is := is.New(t)
			dest, rx := openCaptureDestination(t, map[string]string{"method": "DELETE", "recordHeaders": rh})
			_, err := writeOne(t, dest, deleteRecord())
			is.NoErr(err)
			reqs := rx.requests()
			is.Equal(len(reqs), 1)
			is.Equal(reqs[0].method, "DELETE")
			is.Equal(reqs[0].header.Get(HeaderOperation) != "", rh == "true")
		})
	}
}
