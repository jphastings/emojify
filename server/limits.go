// server/limits.go
package server

import (
	"net"
	"net/http"
	"runtime"

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
		limiter, ok := buckets.Get(ip)
		if !ok {
			limiter = rate.NewLimiter(rate.Limit(cfg.requestsPerSecond), cfg.burst)
			buckets.Add(ip, limiter)
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

func withBoundedConcurrency(next http.Handler) http.Handler {
	n := runtime.NumCPU() - 1
	if n < 1 {
		n = 1
	}
	sem := make(chan struct{}, n)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
		case <-r.Context().Done():
			writeXRPCError(w, http.StatusServiceUnavailable, "Overloaded", "request deadline exceeded while waiting for a free worker")
		}
	})
}

const maxRequestBodyBytes = 4096 // suggestEmojis is a GET with no real body; this is a defensive ceiling, not a real limit

func withMaxBytes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}
