package mix

import (
	"testing"
	"time"
)

// TestIsMix pins which recordings get the chapters: the 61-minute file the
// offsets were measured against, not another long file.
func TestIsMix(t *testing.T) {
	for _, total := range []time.Duration{3674 * time.Second, 3673 * time.Second} {
		if !IsMix(total) {
			t.Errorf("IsMix(%v) = false, want true", total)
		}
	}
	for _, total := range []time.Duration{0, 8 * time.Hour, 3 * time.Minute} {
		if IsMix(total) {
			t.Errorf("IsMix(%v) = true, want false", total)
		}
	}
}
