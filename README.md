# go-beam-quickstart

A token bucket rate limiter for Go that controls operations-per-second with burst support. The bucket starts full, accumulates tokens continuously at a fixed rate, and caps at the burst ceiling.

```go
import beamquickstart "go-beam-quickstart"

lim := beamquickstart.NewLimiter(10, 5) // 10 ops/sec, burst of 5
if lim.Allow() {
    // proceed
}
```

For deterministic tests, pass a custom clock:

```go
lim := beamquickstart.NewLimiterWithClock(10, 5, myClock)
```

## Exported API

- `Clock` — interface with a single method `Now() time.Time`.
- `SystemClock` — struct implementing `Clock` using `time.Now`. Used as the default.
- `Limiter` — the token bucket.
- `NewLimiter(rate float64, burst int) *Limiter` — construct with real time. Panics if `rate <= 0` or `burst <= 0`.
- `NewLimiterWithClock(rate float64, burst int, clock Clock) *Limiter` — construct with an explicit clock for tests.
- `(*Limiter) Allow() bool` — consume one token, return immediately whether the request is allowed.
- `(*Limiter) AllowN(n int) bool` — consume `n` tokens; returns false if `n > burst` (that request can never succeed).
- `(*Limiter) Tokens() float64` — current token count after refilling against the clock.

## Why this exists

The standard library's `time/rate` covers most rate-limiting needs. This library exists as a small, dependency-free implementation focused on a single design decision: **the bucket starts full**. That means a client can burst up to the full capacity immediately on startup, then settle to the steady-state rate. The trade-off is that a freshly constructed limiter permits an initial burst before any throttling kicks in, which may be undesirable if you want ramp-up behaviour.

The clock is abstracted (`Clock` interface) specifically so tests are deterministic. Passing `NewLimiterWithClock` a fake clock lets you advance time in steps and assert exact token counts without sleeping.

## Edge you will hit

If you ask `AllowN` for more tokens than the burst capacity, it returns `false` immediately — no amount of waiting will ever make that request succeed, so the limiter fails fast rather than blocking or silently under-consuming. Treat a `false` from `AllowN` as either "wait and retry" (if `n <= burst`) or "never going to happen" (if `n > burst`).
