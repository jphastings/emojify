// server/limits_test.go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSHeaderOnXRPCRoute(t *testing.T) {
	handler := New(newTestMatcher(t))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/xrpc/me.byjp.emojify.suggestEmojis?text=hello+there")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want \"*\"", got)
	}
}

func TestRateLimitReturns429WhenExceeded(t *testing.T) {
	m := newTestMatcher(t)
	handler := newWithLimits(m, limitConfig{requestsPerSecond: 1, burst: 1})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	url := srv.URL + "/xrpc/me.byjp.emojify.suggestEmojis?text=hello+there"
	first, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	first.Body.Close()

	second, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 (rate limit of 1 req/s, burst 1, hit twice immediately)", second.StatusCode)
	}
}
