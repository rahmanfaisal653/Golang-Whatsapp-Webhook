package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// Default anti-spam limits for the /notify endpoint. They are deliberately
// conservative: WhatsApp bans for volume and repetition far more than speed.
const (
	defaultDailyLimit     = 200
	defaultNewTargetLimit = 50
	defaultDedupWindow    = 60 * time.Second
	defaultMinDelay       = 5 * time.Second
	defaultMaxDelay       = 15 * time.Second
)

// guard throttles /notify so a looping or misbehaving caller cannot burst
// messages, which is the main cause of WhatsApp bans. Its state is in-memory:
// counters reset when the process restarts, which is fine for abuse prevention
// (the goal is to stop bursts, not to bill usage).
type guard struct {
	dailyLimit     int           // max messages per API key per day
	newTargetLimit int           // max first-time recipients per API key per day
	dedupWindow    time.Duration // identical (key, recipient, text) suppressed within this window
	minDelay       time.Duration // minimum gap between two sends
	maxDelay       time.Duration // maximum gap; each gap is random in [minDelay, maxDelay]

	mu       sync.Mutex
	day      string
	keys     map[string]*keyUsage
	nextSend time.Time
	sendMu   sync.Mutex

	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

// keyUsage holds the per-API-key counters for the current day.
type keyUsage struct {
	sent    int
	targets map[string]struct{}
	recent  map[string]time.Time
}

// rejection is a guard refusal carrying the HTTP status to return.
type rejection struct {
	status int
	msg    string
}

func (e *rejection) Error() string { return e.msg }

func newGuard() *guard {
	g := &guard{
		dailyLimit:     defaultDailyLimit,
		newTargetLimit: defaultNewTargetLimit,
		dedupWindow:    defaultDedupWindow,
		minDelay:       defaultMinDelay,
		maxDelay:       defaultMaxDelay,
		keys:           map[string]*keyUsage{},
		now:            time.Now,
		sleep:          sleepCtx,
	}
	if v, ok := envInt("NOTIFY_DAILY_LIMIT"); ok && v > 0 {
		g.dailyLimit = v
	}
	if v, ok := envInt("NOTIFY_NEW_TARGET_LIMIT"); ok && v >= 0 {
		g.newTargetLimit = v
	}
	if v, ok := envInt("NOTIFY_MIN_DELAY_MS"); ok && v >= 0 {
		g.minDelay = time.Duration(v) * time.Millisecond
	}
	if v, ok := envInt("NOTIFY_MAX_DELAY_MS"); ok && v >= 0 {
		g.maxDelay = time.Duration(v) * time.Millisecond
	}
	if g.maxDelay < g.minDelay {
		g.maxDelay = g.minDelay
	}
	log.Printf("web: notify guard: %d msg/key/day, %d new recipients/day, %s dedup, %s-%s spacing",
		g.dailyLimit, g.newTargetLimit, g.dedupWindow, g.minDelay, g.maxDelay)
	return g
}

// Do runs send under the guard: it paces the call, applies the abuse checks and
// records the send on success. A *rejection is returned when the call is
// refused; any other error comes from send itself (and is not counted).
func (g *guard) Do(ctx context.Context, key, to, message string, send func() error) error {
	g.sendMu.Lock()
	defer g.sendMu.Unlock()

	if now := g.now(); now.Before(g.nextSend) {
		wait := g.nextSend.Sub(now)
		if wait > g.maxDelay*2 {
			return &rejection{status: http.StatusTooManyRequests, msg: "too many requests, slow down"}
		}
		if err := g.sleep(ctx, wait); err != nil {
			return err
		}
	}

	if err := g.check(key, to, message); err != nil {
		return err
	}

	// Reserve the next slot before sending so failures are paced too.
	g.nextSend = g.now().Add(g.randomDelay())
	if err := send(); err != nil {
		return err
	}
	g.commit(key, to, message)
	return nil
}

// check reports whether a send is allowed, without changing any state.
func (g *guard) check(key, to, message string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rollDayLocked()

	u := g.usageLocked(key)
	if g.dailyLimit > 0 && u.sent >= g.dailyLimit {
		return &rejection{status: http.StatusTooManyRequests,
			msg: fmt.Sprintf("daily limit reached (%d messages)", g.dailyLimit)}
	}
	if _, seen := u.targets[to]; !seen && g.newTargetLimit > 0 && len(u.targets) >= g.newTargetLimit {
		return &rejection{status: http.StatusTooManyRequests,
			msg: fmt.Sprintf("daily new-recipient limit reached (%d)", g.newTargetLimit)}
	}
	if t, ok := u.recent[dedupHash(to, message)]; ok && g.now().Sub(t) < g.dedupWindow {
		return &rejection{status: http.StatusConflict,
			msg: "duplicate message suppressed (same text to same recipient)"}
	}
	return nil
}

// commit records a successful send so the limits and dedup advance.
func (g *guard) commit(key, to, message string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rollDayLocked()

	u := g.usageLocked(key)
	u.sent++
	u.targets[to] = struct{}{}
	u.recent[dedupHash(to, message)] = g.now()
}

// rollDayLocked clears the per-key counters when the calendar day changes.
func (g *guard) rollDayLocked() {
	day := g.now().Format("2006-01-02")
	if day != g.day {
		g.day = day
		g.keys = map[string]*keyUsage{}
	}
}

func (g *guard) usageLocked(key string) *keyUsage {
	u := g.keys[key]
	if u == nil {
		u = &keyUsage{targets: map[string]struct{}{}, recent: map[string]time.Time{}}
		g.keys[key] = u
	}
	return u
}

func (g *guard) randomDelay() time.Duration {
	if g.maxDelay <= g.minDelay {
		return g.minDelay
	}
	return g.minDelay + time.Duration(rand.Int63n(int64(g.maxDelay-g.minDelay)))
}

// dedupHash keys the duplicate-message window on recipient + text.
func dedupHash(to, message string) string {
	sum := sha256.Sum256([]byte(to + "\x00" + message))
	return hex.EncodeToString(sum[:16])
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func envInt(name string) (int, bool) {
	v := os.Getenv(name)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}
