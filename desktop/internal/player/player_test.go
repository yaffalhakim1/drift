package player

import (
	"os"
	"testing"
	"time"

	"drift/internal/mix"
)

// TestSupportedFormats pins which files the decoder can read. An AAC file is
// the one users actually hit, so it gets its own message.
func TestSupportedFormats(t *testing.T) {
	ok := []string{"mix.mp3", "MIX.MP3", "a.ogg", "a.oga", "a.wav"}
	for _, name := range ok {
		if err := supported(name); err != nil {
			t.Errorf("supported(%q) = %v, want nil", name, err)
		}
	}

	bad := []string{"a.m4a", "a.aac", "a.mp4", "a.flac", "a.txt"}
	for _, name := range bad {
		if err := supported(name); err == nil {
			t.Errorf("supported(%q) = nil, want an error", name)
		}
	}

	// The AAC message must name the fix, not just refuse.
	if err := supported("song.m4a"); err == nil {
		t.Fatal("m4a accepted")
	} else if got := err.Error(); !contains(got, "MP3") {
		t.Errorf("m4a error %q should suggest MP3", got)
	}
}

// TestOpenRejectsUnreadable makes sure a bad path fails loudly instead of
// leaving an empty player that looks like a paused one.
func TestOpenRejectsUnreadable(t *testing.T) {
	p := New("")
	if err := p.Open(""); err != ErrNoFile {
		t.Fatalf("Open(\"\") = %v, want ErrNoFile", err)
	}
	if err := p.Open("does-not-exist.mp3"); err == nil {
		t.Fatal("Open of a missing file returned nil")
	}
	if st := p.State(); st.Loaded {
		t.Fatal("State reports Loaded after a failed Open")
	}
}

// TestRememberedPathOnlyReturnsRealFiles stops the popup from opening on a
// path the user moved or deleted.
func TestRememberedPathOnlyReturnsRealFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := dir + "/last"

	p := New(cfg)
	if got := p.RememberedPath(); got != "" {
		t.Fatalf("RememberedPath() = %q on a fresh player, want empty", got)
	}

	real := dir + "/mix.mp3"
	if err := os.WriteFile(real, []byte("not really an mp3"), 0o644); err != nil {
		t.Fatal(err)
	}
	p.remember(real)
	if got := p.RememberedPath(); got != real {
		t.Fatalf("RememberedPath() = %q, want %q", got, real)
	}

	// A file that went away must not be offered again.
	if err := os.Remove(real); err != nil {
		t.Fatal(err)
	}
	if got := p.RememberedPath(); got != "" {
		t.Fatalf("RememberedPath() = %q after the file was deleted, want empty", got)
	}
}

// TestChapterNavigation checks the rule players share: previous restarts the
// chapter first, and only then goes back.
func TestChapterNavigation(t *testing.T) {
	// Chapter 3 starts at 3:25 (205 s) and runs to 5:28 (328 s).
	const third = 2
	start := mix.Chapters[third].At
	end := mix.End(third, 3673*time.Second)

	if got := mix.Length(third, 3673*time.Second); got != end-start {
		t.Fatalf("Length(%d) = %v, want %v", third, got, end-start)
	}
	if end <= start {
		t.Fatal("chapter has no length")
	}

	// The last chapter runs to the end of the recording, not the tracklist:
	// this recording is longer than its list of stops.
	last := len(mix.Chapters) - 1
	total := 3673 * time.Second
	lastLen := mix.Length(last, total)
	listOnly := total - mix.Chapters[last].At
	if lastLen != listOnly {
		t.Fatalf("last chapter length = %v, want the recording's tail %v", lastLen, listOnly)
	}
	if lastLen <= 70*time.Second {
		t.Fatalf("last chapter is %v: it should use the recording's tail, not the list's 1:10", lastLen)
	}
}

// TestIndexAtBoundaries pins the mapping from a position to a chapter.
func TestIndexAtBoundaries(t *testing.T) {
	cases := []struct {
		at   time.Duration
		want int
	}{
		{0, 0},
		{14 * time.Second, 0},
		{15 * time.Second, 1},
		{204 * time.Second, 1},
		{205 * time.Second, 2},
		{2168 * time.Second, 17},
		{3673 * time.Second, 27},
	}
	for _, c := range cases {
		if got := mix.IndexAt(c.at); got != c.want {
			t.Errorf("IndexAt(%v) = %d, want %d", c.at, got, c.want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
