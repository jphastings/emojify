// server/limits.go
package server

import (
	"context"
	"net"
	"net/http"
	"runtime"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/time/rate"
)

type limitConfig struct {
	requestsPerSecond float64
	burst             int
}

var defaultLimitConfig = limitConfig{requestsPerSecond: 2, burst: 5}

const bucketCacheSize = 4096

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}

func withRateLimit(cfg limitConfig, next http.Handler) http.Handler {
	buckets, err := lru.New[string, *rate.Limiter](bucketCacheSize)
	if err != nil {
		panic(err) // bucketCacheSize is a positive compile-time constant; New only errors on size<=0
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		// PeekOrAdd is a single locked get-or-insert: Get-then-Add here would
		// let concurrent requests for a brand-new IP each construct and use
		// their own private limiter before any Add() lands, defeating the
		// burst limit during exactly the concurrent-burst pattern it exists
		// to stop.
		newLimiter := rate.NewLimiter(rate.Limit(cfg.requestsPerSecond), cfg.burst)
		existing, found, _ := buckets.PeekOrAdd(ip, newLimiter)
		limiter := newLimiter
		if found {
			limiter = existing
			// PeekOrAdd's internal peek does not update recency, so a cache
			// hit alone would leave this entry's LRU position untouched —
			// degrading eviction to insertion-order FIFO under sustained
			// traffic from many distinct IPs. Get does update recency, so
			// call it once here to keep genuinely active IPs from being
			// evicted ahead of idle-but-recently-inserted ones.
			buckets.Get(ip)
		}
		if !limiter.Allow() {
			writeXRPCError(w, http.StatusTooManyRequests, "RateLimitExceeded", "too many requests, slow down")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// maxQueueWait bounds how long a request waits for a free concurrency slot
// before being shed with 503 Overloaded. Nothing else in this codebase ever
// puts a deadline on the request context (ReadHeaderTimeout only bounds
// header-reading, not the request lifetime), so without this the semaphore
// wait below is unbounded and load-shedding never fires under real
// saturation. This guards a resource-constrained board (e.g. a Pi Zero 2 W)
// where a slow/stuck worker should shed queued load quickly rather than let
// requests pile up indefinitely, so a short deadline is preferred over a
// generous one.
const maxQueueWait = 3 * time.Second

func withBoundedConcurrency(next http.Handler) http.Handler {
	n := runtime.NumCPU() - 1
	if n < 1 {
		n = 1
	}
	return withBoundedConcurrencyLimits(n, maxQueueWait, next)
}

// withBoundedConcurrencyLimits is withBoundedConcurrency with its worker
// count and queue-wait deadline as parameters, so tests can saturate a
// small semaphore and use a short deadline instead of waiting out the real
// maxQueueWait.
func withBoundedConcurrencyLimits(workers int, queueWait time.Duration, next http.Handler) http.Handler {
	sem := make(chan struct{}, workers)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), queueWait)
		defer cancel()
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
		case <-ctx.Done():
			writeXRPCError(w, http.StatusServiceUnavailable, "Overloaded", "request deadline exceeded while waiting for a free worker")
		}
	})
}

const maxRequestBodyBytes = 4096 // suggestEmoji is a GET with no real body; this is a defensive ceiling, not a real limit

func withMaxBytes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}
