package idle

import "testing"

func TestIdleSecondsNonNegative(t *testing.T) {
	secs, err := IdleSeconds()
	if err != nil {
		t.Fatalf("IdleSeconds() error: %v", err)
	}
	if secs < 0 {
		t.Errorf("IdleSeconds() = %f, want >= 0", secs)
	}
}

func TestIdleSecondsReasonableRange(t *testing.T) {
	secs, err := IdleSeconds()
	if err != nil {
		t.Fatalf("IdleSeconds() error: %v", err)
	}
	// System uptime-based — should be less than 30 days in seconds.
	const maxReasonable = 30 * 24 * 60 * 60
	if secs > maxReasonable {
		t.Errorf("IdleSeconds() = %f, suspiciously large (> 30 days)", secs)
	}
}
