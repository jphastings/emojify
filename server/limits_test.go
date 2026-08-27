// server/limits_test.go
package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
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

// TestBoundedConcurrencyShedsLoadWithin503 exercises the deadline added to
// withBoundedConcurrency: with no deadline on the request context, a request
// queued behind a saturated semaphore waits forever instead of being shed.
// This saturates a single-worker semaphore with a handler that blocks until
// released, then confirms a second request gets 503 Overloaded within
// roughly the configured queue-wait, rather than hanging.
func TestBoundedConcurrencyShedsLoadWithin503(t *testing.T) {
	const queueWait = 200 * time.Millisecond

	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)

	blockingNext := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started) // only reached once the semaphore's single slot is held
		<-release
		w.WriteHeader(http.StatusOK)
	})
	handler := withBoundedConcurrencyLimits(1, queueWait, blockingNext)

	go func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}()
	<-started // the one worker slot is now occupied and held

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (semaphore saturated by a still-blocked handler)", rec.Code)
	}
	if elapsed < queueWait/2 {
		t.Errorf("503 returned after %v, suspiciously fast for a %v queue-wait deadline — expected it to actually wait out the deadline, not reject immediately", elapsed, queueWait)
	}
	if elapsed > 5*queueWait {
		t.Errorf("503 returned after %v, want roughly %v — the request appears to have waited far longer than the configured deadline", elapsed, queueWait)
	}
}

// TestRateLimitLRURefreshesRecencyOnHit guards against PeekOrAdd's internal
// peek (not touch) never refreshing an entry's LRU recency on a cache hit,
// which degrades eviction to insertion order: a bucket that is repeatedly
// hit can still be evicted ahead of buckets inserted later but never
// touched again. requestsPerSecond is 0 so, absent any eviction, a bucket
// that has exhausted its single-token burst never refills and stays
// rate-limited (429) for the rest of the test — an unexpected 200 later
// means its bucket was evicted and silently recreated with a fresh burst.
func TestRateLimitLRURefreshesRecencyOnHit(t *testing.T) {
	cfg := limitConfig{requestsPerSecond: 0, burst: 1}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := withRateLimit(cfg, next)

	doReq := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	fillerIP := func(batch, i int) string {
		return fmt.Sprintf("10.%d.%d.%d", batch, i/256, i%256)
	}

	const activeIP = "203.0.113.1"
	if code := doReq(activeIP); code != http.StatusOK {
		t.Fatalf("first request for active IP = %d, want 200 (fresh bucket, burst 1)", code)
	}
	if code := doReq(activeIP); code != http.StatusTooManyRequests {
		t.Fatalf("second immediate request for active IP = %d, want 429 (burst exhausted)", code)
	}

	const halfCache = bucketCacheSize / 2
	for i := 0; i < halfCache; i++ {
		doReq(fillerIP(1, i)) // fills roughly half the cache with untouched entries older than the re-touch below
	}

	// Re-touch the active IP. With the fix (Get on a PeekOrAdd hit), this
	// moves it to the most-recently-used position, ahead of every filler
	// IP inserted so far.
	if code := doReq(activeIP); code != http.StatusTooManyRequests {
		t.Fatalf("re-touch of active IP = %d, want 429 (still its own rate-limited bucket)", code)
	}

	// One more than half the cache: pushes total distinct entries one past
	// bucketCacheSize, forcing exactly the eviction(s) needed to distinguish
	// the two orderings. Without the fix, activeIP (never recency-bumped)
	// is the globally oldest entry and gets evicted here; with the fix, the
	// untouched batch-1 filler entries are older and get evicted instead.
	for i := 0; i < halfCache+1; i++ {
		doReq(fillerIP(2, i))
	}

	if code := doReq(activeIP); code != http.StatusTooManyRequests {
		t.Errorf("active IP got %d after cache churn, want 429 — its bucket should have survived eviction because its last hit refreshed LRU recency; 200 means it was evicted and silently recreated with a fresh burst", code)
	}
}
