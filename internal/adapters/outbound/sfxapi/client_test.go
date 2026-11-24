// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package sfxapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testKey = "sk-test-secret-key"

// newTestClient points a Client at a fake server; no network, no sleeps
// (only non-retried statuses are used, so Backoff is never waited on).
func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Client{HTTP: srv.Client(), Key: testKey, BaseURL: srv.URL}
}

func TestSubscriptionReadsCreditCounter(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("xi-api-key"); got != testKey {
			t.Errorf("xi-api-key = %q, want the configured key", got)
		}
		_, _ = w.Write([]byte(`{"character_count":10,"character_limit":100}`))
	})
	s, err := c.Subscription(context.Background())
	if err != nil {
		t.Fatalf("Subscription: %v", err)
	}
	if s.Used != 10 || s.Limit != 100 {
		t.Fatalf("Subscription = %+v, want used 10 limit 100", s)
	}
}

func TestUnexpectedStatusReturnsErrorWithRedactedBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request echoing " + testKey))
	})
	_, err := c.Synthesize(context.Background(), "door", 1, 0.3)
	if err == nil {
		t.Fatal("Synthesize on HTTP 400 returned no error")
	}
	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("error %q does not name HTTP 400", err)
	}
	if strings.Contains(err.Error(), testKey) {
		t.Fatalf("error %q leaks the API key", err)
	}
	if !strings.Contains(err.Error(), "[key]") {
		t.Fatalf("error %q does not show the redacted key marker", err)
	}
}

func TestUnauthorizedIsFatalAndNamesKeyEnv(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := c.Subscription(context.Background())
	var fatal FatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("error %v is not a FatalError", err)
	}
	if !strings.Contains(fatal.Msg, KeyEnv) {
		t.Fatalf("message %q does not name %s", fatal.Msg, KeyEnv)
	}
}
