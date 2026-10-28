package beamquickstart

import (
	"math"
	"testing"
	"time"
)

// fakeClock is a controllable Clock for deterministic tests.
// We advance it manually rather than sleeping, so tests run instantly and
// pass identically on every machine.
type fakeClock struct {
	t time.Time
}

func (f *fakeClock) Now() time.Time { return f.t }

// approx compares two floats with a tolerance.
// Float == comparisons are fragile across architectures, so every test uses this.
func approx(a, b, eps float64) bool { return math.Abs(a-b) < eps }

func TestNewLimiterPanicsOnZeroRate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for rate <= 0")
		}
	}()
	NewLimiter(0, 10)
}

func TestNewLimiterPanicsOnZeroBurst(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for burst <= 0")
		}
	}()
	NewLimiter(1, 0)
}

func TestAllowConsumesFromInitialBurst(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	lim := NewLimiterWithClock(1, 5, clk)
	count := 0
	for i := 0; i < 5; i++ {
		if lim.Allow() {
			count++
		}
	}
	if count != 5 {
		t.Fatalf("expected 5 allowed, got %d", count)
	}
}

func TestAllowReturnsFalseWhenBurstEmpty(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	lim := NewLimiterWithClock(1, 3, clk)
	for i := 0; i < 3; i++ {
		lim.Allow()
	}
	if lim.Allow() {
		t.Fatal("expected Allow() to be false after burst exhausted")
	}
}

func TestRefillAfterOneSecond(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	lim := NewLimiterWithClock(1, 1, clk)
	lim.Allow()
	if lim.Allow() {
		t.Fatal("bucket should be empty")
	}
	clk.t = clk.t.Add(1 * time.Second)
	if !lim.Allow() {
		t.Fatal("expected 1 token to be available after 1 second")
	}
}

func TestRefillCapsAtBurst(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	lim := NewLimiterWithClock(2, 3, clk)
	for i := 0; i < 3; i++ {
		lim.Allow()
	}
	clk.t = clk.t.Add(100 * time.Second)
	// The bucket should cap at 3 even after a long idle period.
	allowed := 0
	for i := 0; i < 3; i++ {
		if lim.Allow() {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("expected exactly 3 tokens after idle, got %d", allowed)
	}
	if lim.Allow() {
		t.Fatal("expected bucket to be empty after draining capped refill")
	}
}

func TestAllowNRejectsOverBurst(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	lim := NewLimiterWithClock(1, 5, clk)
	if lim.AllowN(6) {
		t.Fatal("AllowN exceeding burst must return false")
	}
}

func TestAllowNWithZeroSucceeds(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	lim := NewLimiterWithClock(1, 1, clk)
	if !lim.AllowN(0) {
		t.Fatal("AllowN(0) should always succeed")
	}
	if !lim.AllowN(-1) {
		t.Fatal("AllowN with negative n should succeed without consuming tokens")
	}
}

func TestAllowNConsumesMultiple(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	lim := NewLimiterWithClock(1, 5, clk)
	if !lim.AllowN(3) {
		t.Fatal("AllowN(3) should succeed")
	}
	if !lim.AllowN(2) {
		t.Fatal("AllowN(2) should succeed on remaining tokens")
	}
	if lim.AllowN(1) {
		t.Fatal("AllowN(1) should fail — bucket empty")
	}
}

func TestTokensReturnsCurrentValue(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	lim := NewLimiterWithClock(1, 5, clk)
	lim.Allow()
	got := lim.Tokens()
	if !approx(got, 4, 0.0001) {
		t.Fatalf("expected ~4 tokens after one Allow, got %f", got)
	}
}

func TestFractionalRefill(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	lim := NewLimiterWithClock(4, 4, clk)
	for i := 0; i < 4; i++ {
		lim.Allow()
	}
	if lim.Allow() {
		t.Fatal("bucket should be empty")
	}
	// 0.5 seconds at 4/s yields exactly 2 tokens.
	clk.t = clk.t.Add(500 * time.Millisecond)
	if !lim.AllowN(2) {
		t.Fatal("expected 2 tokens after 0.5s at rate 4/s")
	}
	if lim.Allow() {
		t.Fatal("expected bucket to be empty again")
	}
}

func TestZeroElapsedDoesNotAddTokens(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	lim := NewLimiterWithClock(10, 1, clk)
	lim.Allow()
	if lim.Allow() {
		t.Fatal("should be empty immediately after first Allow")
	}
	// Calling again without advancing the clock must not add tokens.
	if lim.Allow() {
		t.Fatal("calling again without time advancing must not add tokens")
	}
}
