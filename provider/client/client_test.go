package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// flakyRoundTripper fails the first failFirst calls with a transport-level error
// (as if the request never reached the server), then delegates to next.
type flakyRoundTripper struct {
	failFirst int32
	next      http.RoundTripper
}

func (f *flakyRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if atomic.AddInt32(&f.failFirst, -1) >= 0 {
		return nil, fmt.Errorf("simulated transient failure")
	}
	return f.next.RoundTrip(r)
}

// TestServerVersionCachesDefinitiveAnswer verifies that a successful /version
// response (and a "no such endpoint" non-200) is fetched once and memoized.
func TestServerVersionCachesDefinitiveAnswer(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantVer string
	}{
		{name: "ok", status: http.StatusOK, body: "2.16.2\n", wantVer: "2.16.2"},
		{name: "endpoint absent", status: http.StatusNotFound, body: "nope", wantVer: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&hits, 1)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c, err := NewClient(srv.URL, "", "", "", false)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			for i := 0; i < 3; i++ {
				got, err := c.ServerVersion(context.Background())
				if err != nil {
					t.Fatalf("call %d: unexpected error: %v", i, err)
				}
				if got != tt.wantVer {
					t.Fatalf("call %d: version = %q, want %q", i, got, tt.wantVer)
				}
			}
			if got := atomic.LoadInt32(&hits); got != 1 {
				t.Fatalf("server hit %d times, want 1 (result should be cached)", got)
			}
		})
	}
}

// TestServerVersionRetriesTransientFailure verifies that a transient failure is
// NOT cached: a first failed call returns an error, and a later call succeeds
// instead of the process being permanently downgraded to "unknown version".
func TestServerVersionRetriesTransientFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("3.1.4"))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "", "", "", false)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.httpClient = &http.Client{Transport: &flakyRoundTripper{failFirst: 1, next: http.DefaultTransport}}

	if _, err := c.ServerVersion(context.Background()); err == nil {
		t.Fatal("first call: want transient error, got nil")
	}
	got, err := c.ServerVersion(context.Background())
	if err != nil {
		t.Fatalf("second call: unexpected error: %v", err)
	}
	if got != "3.1.4" {
		t.Fatalf("second call: version = %q, want %q (failure must not be cached)", got, "3.1.4")
	}
}
