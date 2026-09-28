// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

// Limiter is a simple fixed-window counter per key.
type Limiter struct {
	mu      sync.Mutex
	window  time.Duration
	max     int
	buckets map[string]*bucket
	now     func() time.Time
}

type bucket struct {
	windowStart time.Time
	count       int
}

func New(window time.Duration, max int) *Limiter {
	if window <= 0 {
		window = time.Minute
	}
	if max <= 0 {
		max = 20
	}
	return &Limiter{
		window:  window,
		max:     max,
		buckets: make(map[string]*bucket),
		now:     time.Now,
	}
}

func (l *Limiter) Allow(key string) bool {
	if key == "" {
		key = "unknown"
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b == nil || now.Sub(b.windowStart) >= l.window {
		l.buckets[key] = &bucket{windowStart: now, count: 1}
		return true
	}
	if b.count >= l.max {
		return false
	}
	b.count++
	return true
}

// Middleware rejects with 429 when over limit.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			key = xff
		}
		if !l.Allow(key) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// MiddlewareFunc wraps a raw handler func.
func (l *Limiter) MiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return l.Middleware(next).ServeHTTP
}
