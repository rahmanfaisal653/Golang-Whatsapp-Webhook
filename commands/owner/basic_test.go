package owner

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	got := formatDuration(2*time.Hour + 3*time.Minute + 4*time.Second + 500*time.Millisecond)
	want := "2h 3m 5s"
	if got != want {
		t.Fatalf("formatDuration() = %q, want %q", got, want)
	}
}
