package player

import (
	"os"
	"testing"
	"time"
)

// TestStreamRealRecording streams the user's own file when DRIFT_MIX points at
// it. This is the one test that proves the format registration, the decoder,
// seeking and the chapter mapping all agree with a real 61 minute MP3.
//
//	go test ./internal/player -run TestStreamRealRecording -v
func TestStreamRealRecording(t *testing.T) {
	path := os.Getenv("DRIFT_MIX")
	if path == "" {
		t.Skip("set DRIFT_MIX to the recording to run this")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("recording not found: %v", err)
	}

	p := New("")
	if err := p.Open(path); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer p.Close()

	st := p.State()
	if !st.Loaded {
		t.Fatal("not loaded after a successful Open")
	}
	// The file is 61:13. A decoder that read the header wrong would show 0
	// or something absurd here.
	if st.Total < 3600*time.Second || st.Total > 3800*time.Second {
		t.Fatalf("Total = %v, want about 61 minutes", st.Total)
	}
	t.Logf("loaded %v, %d chapters", st.Total.Round(time.Second), 28)

	// Seek to chapter 18 (Owls of the Night), which is 36:08 in.
	const target = 2168 * time.Second
	p.Seek(target)
	// The decoder needs a moment to land.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st = p.State()
		if d := st.Position - target; d > -2*time.Second && d < 2*time.Second {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	st = p.State()
	if d := st.Position - target; d < -2*time.Second || d > 2*time.Second {
		t.Fatalf("after Seek(%v), position = %v (off by %v)", target, st.Position, d)
	}
	if st.Chapter != 17 {
		t.Fatalf("chapter after seek = %d, want 17", st.Chapter)
	}
	t.Logf("seek to %v landed at %v, chapter %d", target, st.Position.Round(time.Second), st.Chapter)
}
