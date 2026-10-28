package beamquickstart

import (
	"sync"
	"time"
)

// Clock abstracts a source of monotonic time so tests can control it.
// We pass it into NewLimiterWithClock instead of reading from time.Now
// directly so that the package is fully deterministic under test.
type Clock interface {
	Now() time.Time
}

// SystemClock reads real wall-clock time.
// It's the default clock so users get time.Now behaviour unless they
// explicitly need a fake.
type SystemClock struct{}

// Now returns the current time using the standard library clock.
func (SystemClock) Now() time.Time { return time.Now() }

// Limiter is a token bucket rate limiter.
//
// It controls how many operations per second are allowed while permitting
// bursts up to a fixed ceiling. Tokens accumulate continuously at a fixed
// per-second rate and can never exceed the burst capacity.
type Limiter struct {
	mu          sync.Mutex
	rate        float64       // tokens added per second
	burst       float64       // maximum tokens the bucket can hold
	tokens      float64       // current token count
	lastRefill  time.Time     // last time tokens were computed
	clock       Clock         // abstract clock for testability
}

// NewLimiter creates a Limiter that adds rate tokens per second and can
// hold at most burst tokens.
//
// rate must be > 0; burst must be > 0. If either is zero, the limiter can
// never allow a request, which would be surprising. We panic so the caller
// fails fast at construction rather than at first use.
func NewLimiter(rate float64, burst int) *Limiter {
	return NewLimiterWithClock(rate, burst, SystemClock{})
}

// NewLimiterWithClock is like NewLimiter but takes an explicit Clock.
// Provided so tests can advance time deterministically without sleeping.
func NewLimiterWithClock(rate float64, burst int, clock Clock) *Limiter {
	if rate <= 0 {
		panic("beamquickstart: rate must be > 0")
	}
	if burst <= 0 {
		panic("beamquickstart: burst must be > 0")
	}
	if clock == nil {
		clock = SystemClock{}
	}
	return &Limiter{
		rate:       rate,
		burst:      float64(burst),
		tokens:     float64(burst), // start full so first burst works
		lastRefill: clock.Now(),
		clock:      clock,
	}
}

// refill recomputes the bucket state. Caller must hold mu.
//
// We add tokens proportional to elapsed seconds since the last refill and
// clamp to burst. If the clock hasn't advanced we add nothing.
func (l *Limiter) refill() {
	now := l.clock.Now()
	elapsed := now.Sub(l.lastRefill)
	if elapsed <= 0 {
		return
	}
	l.tokens += l.rate * elapsed.Seconds()
	if l.tokens > l.burst {
		l.tokens = l.burst
	}
	l.lastRefill = now
}

// Allow consumes one token if available and reports whether the request is
// permitted. Returns true immediately if a token is available, false
// immediately otherwise — this method never blocks.
func (l *Limiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN consumes n tokens if available and reports whether the request is
// permitted. If n exceeds burst, the result is always false: no amount of
// waiting will ever make such a request succeed, so we fail fast.
func (l *Limiter) AllowN(n int) bool {
	if n <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	if float64(n) > l.burst {
		return false
	}
	if l.tokens < float64(n) {
		return false
	}
	l.tokens -= float64(n)
	return true
}

// Tokens returns the current token count after refilling against the clock.
// The value is a float because tokens accumulate continuously.
func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	return l.tokens
}
