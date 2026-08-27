// server/limits_test.go
package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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

// TestRateLimitConcurrentBurstFromSameIP guards against a check-then-act race
// in the per-IP bucket lookup: if concurrent requests for a brand-new IP can
// each observe "no bucket yet" and construct their own private limiter before
// any of them gets stored, every one of those private limiters starts fresh
// and Allow()s its single request — defeating burst:1 during exactly the
// concurrent-burst pattern the limiter exists to stop. A sequential
// request-then-request test can't observe this; it requires genuine
// concurrency against a single, not-yet-seen IP.
//
// Requests are dispatched directly against the handler (rather than over a
// real httptest.NewServer network round trip) and released from a shared
// start gate: the per-connection accept/dispatch overhead of a real network
// round trip otherwise swamps the get-then-add race window, making the race
// far less likely to be observed even when present.
//
// The race window (between the lookup missing and the new bucket being
// stored) is narrow, so any single burst only has a middling chance of
// landing inside it. This runs many independent bursts, each against its own
// fresh handler and IP, and fails if any single burst let more than one
// request through — that keeps the false-negative rate on the buggy
// get-then-add code negligible while the atomic fix can never fail this
// regardless of attempt count.
func TestRateLimitConcurrentBurstFromSameIP(t *testing.T) {
	const attempts = 20
	const burstSize = 100

	for attempt := 0; attempt < attempts; attempt++ {
		m := newTestMatcher(t)
		handler := newWithLimits(m, limitConfig{requestsPerSecond: 1, burst: 1})
		statuses := make([]int, burstSize)

		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < burstSize; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				req := httptest.NewRequest(http.MethodGet, "/xrpc/me.byjp.emojify.suggestEmojis?text=hello+there", nil)
				req.RemoteAddr = fmt.Sprintf("203.0.113.%d:12345", attempt) // same simulated client IP within an attempt, unique across attempts
				rec := httptest.NewRecorder()
				<-start
				handler.ServeHTTP(rec, req)
				statuses[i] = rec.Code
			}(i)
		}
		close(start)
		wg.Wait()

		var ok, limited int
		for _, status := range statuses {
			switch status {
			case http.StatusOK:
				ok++
			case http.StatusTooManyRequests:
				limited++
			default:
				t.Fatalf("attempt %d: unexpected status %d", attempt, status)
			}
		}
		if ok > 1 {
			t.Fatalf("attempt %d: %d/%d concurrent requests from the same IP succeeded with burst=1 (want at most 1); the rate limiter's get-then-add race let multiple private limiters through", attempt, ok, burstSize)
		}
		if ok+limited != burstSize {
			t.Fatalf("attempt %d: ok(%d)+limited(%d) = %d, want %d", attempt, ok, limited, ok+limited, burstSize)
		}
	}
}
