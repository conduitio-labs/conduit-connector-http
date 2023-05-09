package http_test

import (
	"context"
	"testing"

	http "github.com/conduitio-labs/conduit-connector-http"
)

func TestTeardown_NoOpen(t *testing.T) {
	con := http.NewDestination()
	err := con.Teardown(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
