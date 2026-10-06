package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// newTestGuard returns a guard with a controllable clock and no real sleeping.
func newTestGuard() *guard {
	g := newGuard()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return now }
	g.sleep = func(context.Context, time.Duration) error { return nil }
	return g
}

// A second send to the same recipient with the same text is suppressed.
func TestGuardDedup(t *testing.T) {
	g := newTestGuard()
	ok := func() error { return nil }
	if err := g.Do(context.Background(), "k", "a@s.whatsapp.net", "hi", ok); err != nil {
		t.Fatalf("first send: %v", err)
	}
	err := g.Do(context.Background(), "k", "a@s.whatsapp.net", "hi", ok)
	var rej *rejection
	if !errors.As(err, &rej) || rej.status != http.StatusConflict {
		t.Fatalf("duplicate send = %v, want 409 rejection", err)
	}
	// Different text is allowed.
	if err := g.Do(context.Background(), "k", "a@s.whatsapp.net", "other", ok); err != nil {
		t.Fatalf("different text: %v", err)
	}
}

// The daily limit blocks further sends for the same key.
func TestGuardDailyLimit(t *testing.T) {
	g := newTestGuard()
	g.dailyLimit = 2
	ok := func() error { return nil }
	for i, to := range []string{"a@s", "b@s"} {
		if err := g.Do(context.Background(), "k", to, "m", ok); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	err := g.Do(context.Background(), "k", "c@s", "m", ok)
	var rej *rejection
	if !errors.As(err, &rej) || rej.status != http.StatusTooManyRequests {
		t.Fatalf("over limit = %v, want 429 rejection", err)
	}
	// A different key has its own budget.
	if err := g.Do(context.Background(), "k2", "c@s", "m", ok); err != nil {
		t.Fatalf("other key: %v", err)
	}
}

// The new-recipient limit blocks messages to brand-new numbers.
func TestGuardNewTargetLimit(t *testing.T) {
	g := newTestGuard()
	g.newTargetLimit = 1
	ok := func() error { return nil }
	if err := g.Do(context.Background(), "k", "a@s", "m1", ok); err != nil {
		t.Fatalf("first target: %v", err)
	}
	// Same target again is fine (not a new recipient).
	if err := g.Do(context.Background(), "k", "a@s", "m2", ok); err != nil {
		t.Fatalf("repeat target: %v", err)
	}
	err := g.Do(context.Background(), "k", "b@s", "m3", ok)
	var rej *rejection
	if !errors.As(err, &rej) || rej.status != http.StatusTooManyRequests {
		t.Fatalf("new target over limit = %v, want 429 rejection", err)
	}
}

// A failed send is not counted and does not consume the dedup window.
func TestGuardFailedSendNotCounted(t *testing.T) {
	g := newTestGuard()
	g.dailyLimit = 1
	boom := func() error { return errors.New("boom") }
	if err := g.Do(context.Background(), "k", "a@s", "m", boom); err == nil {
		t.Fatal("expected the send error to propagate")
	}
	// The one allowed send is still available.
	if err := g.Do(context.Background(), "k", "a@s", "m", func() error { return nil }); err != nil {
		t.Fatalf("after failed send: %v", err)
	}
}

// Consecutive sends are spaced by at least minDelay.
func TestGuardSpacing(t *testing.T) {
	g := newTestGuard()
	g.minDelay = 5 * time.Second
	g.maxDelay = 5 * time.Second

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	now := base
	g.now = func() time.Time { return now }
	var slept []time.Duration
	g.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		now = now.Add(d) // advance the fake clock as real sleep would
		return nil
	}
	ok := func() error { return nil }
	if err := g.Do(context.Background(), "k", "a@s", "m1", ok); err != nil {
		t.Fatal(err)
	}
	if err := g.Do(context.Background(), "k", "b@s", "m2", ok); err != nil {
		t.Fatal(err)
	}
	if len(slept) != 1 || slept[0] < 5*time.Second {
		t.Fatalf("slept = %v, want one wait >= 5s", slept)
	}
}

// A day change resets the per-key counters.
func TestGuardDayRollover(t *testing.T) {
	g := newTestGuard()
	g.dailyLimit = 1
	day := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return day }
	ok := func() error { return nil }
	if err := g.Do(context.Background(), "k", "a@s", "m", ok); err != nil {
		t.Fatal(err)
	}
	if err := g.Do(context.Background(), "k", "b@s", "m", ok); err == nil {
		t.Fatal("expected the daily limit to block the second send")
	}
	day = day.Add(24 * time.Hour) // next day
	if err := g.Do(context.Background(), "k", "b@s", "m", ok); err != nil {
		t.Fatalf("after rollover: %v", err)
	}
}

// A realistic burst to many new numbers is stopped by the new-recipient cap.
func TestGuardBurstScenario(t *testing.T) {
	g := newTestGuard()
	g.dailyLimit = 200
	g.newTargetLimit = 50
	ok := func() error { return nil }

	var sent, throttled int
	for i := 0; i < 300; i++ {
		to := fmt.Sprintf("628%09d@s.whatsapp.net", i)
		err := g.Do(context.Background(), "k", to, "promo "+strconv.Itoa(i), ok)
		var rej *rejection
		if errors.As(err, &rej) {
			throttled++
		} else if err == nil {
			sent++
		} else {
			t.Fatalf("send %d: unexpected error %v", i, err)
		}
	}
	if sent != 50 || throttled != 250 {
		t.Fatalf("burst: sent=%d throttled=%d, want sent=50 throttled=250", sent, throttled)
	}
}

// The daily cap stops a caller looping the same recipient with fresh text.
func TestGuardDailyCapOnRepeat(t *testing.T) {
	g := newTestGuard()
	g.dailyLimit = 5
	g.newTargetLimit = 0 // disable the new-recipient cap
	ok := func() error { return nil }

	sent := 0
	for i := 0; i < 10; i++ {
		err := g.Do(context.Background(), "k", "a@s", "msg "+strconv.Itoa(i), ok)
		if err == nil {
			sent++
		}
	}
	if sent != 5 {
		t.Fatalf("sent=%d, want 5 (daily cap)", sent)
	}
}

// Environment variables override the built-in limits.
func TestGuardEnvOverride(t *testing.T) {
	t.Setenv("NOTIFY_DAILY_LIMIT", "7")
	t.Setenv("NOTIFY_NEW_TARGET_LIMIT", "3")
	t.Setenv("NOTIFY_MIN_DELAY_MS", "1000")
	t.Setenv("NOTIFY_MAX_DELAY_MS", "2000")
	g := newGuard()
	if g.dailyLimit != 7 || g.newTargetLimit != 3 {
		t.Fatalf("limits = %d/%d, want 7/3", g.dailyLimit, g.newTargetLimit)
	}
	if g.minDelay != time.Second || g.maxDelay != 2*time.Second {
		t.Fatalf("delays = %v-%v, want 1s-2s", g.minDelay, g.maxDelay)
	}
}
