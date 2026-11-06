package player

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseQueueName pins the filename parsing against real yt-dlp names
// from the sheeran-mix folder. The titles are what the screen shows.
func TestParseQueueName(t *testing.T) {
	cases := []struct {
		in         string
		wantTitle  string
		wantArtist string
	}{
		{"003 - Ed Sheeran - Photograph (Official Video).mp3", "Photograph", "Ed Sheeran"},
		{"001 - Ed Sheeran - Thinking Out Loud (Official Music Video).mp3", "Thinking Out Loud", "Ed Sheeran"},
		{"004 - Bruno Mars - Just The Way You Are (Official Music Video).mp3", "Just The Way You Are", "Bruno Mars"},
		{"012 - Meghan Trainor - Like I'm Gonna Lose You (Official Video) ft. John Legend.mp3", "Like I'm Gonna Lose You  ft. John Legend", "Meghan Trainor"},
		{"011 - Christina Perri - A Thousand Years [Official Music Video].mp3", "A Thousand Years", "Christina Perri"},
		{"009 - Lewis Capaldi - Someone You Loved.mp3", "Someone You Loved", "Lewis Capaldi"},
		{"005 - Passenger - Let Her Go (Lyrics).mp3", "Let Her Go", "Passenger"},
		{"014 - Lady Gaga, Bruno Mars - Die With A Smile (Official Music Video).mp3", "Die With A Smile", "Lady Gaga, Bruno Mars"},
		{"16 - Justin Bieber - Love Yourself (PURPOSE ： The Movement).mp3", "Love Yourself (PURPOSE ： The Movement)", "Justin Bieber"},
		{"song.mp3", "song", ""},
		{"Artist - Title.mp3", "Title", "Artist"},
		{"01 - Intro.mp3", "Intro", ""},
	}
	for _, c := range cases {
		title, artist := parseQueueName(c.in)
		if title != c.wantTitle || artist != c.wantArtist {
			t.Errorf("parseQueueName(%q) = (%q, %q), want (%q, %q)",
				c.in, title, artist, c.wantTitle, c.wantArtist)
		}
	}
}

// TestFolderTracks checks the folder scan: only playable files, sorted,
// directories and junk skipped, empty folder refused.
func TestFolderTracks(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("b - second.mp3")
	write("a - first.mp3")
	write("notes.txt")
	write("clip.m4a")
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FolderTracks(dir)
	if err != nil {
		t.Fatalf("FolderTracks: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("FolderTracks returned %d paths, want 2: %v", len(got), got)
	}
	base := filepath.Base(got[0])
	if base != "a - first.mp3" {
		t.Errorf("first = %q, want the sorted first", base)
	}

	empty := t.TempDir()
	if _, err := FolderTracks(empty); err == nil {
		t.Fatal("empty folder accepted")
	}
	if _, err := FolderTracks(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing folder accepted")
	}
}

// TestTrackTransition pins the wrap-around rules of the queue.
func TestTrackTransition(t *testing.T) {
	if got := nextTrackIndex(0, 3); got != 1 {
		t.Errorf("next(0 of 3) = %d, want 1", got)
	}
	if got := nextTrackIndex(2, 3); got != 0 {
		t.Errorf("next(last of 3) = %d, want wrap to 0", got)
	}
	if got := nextTrackIndex(0, 0); got != -1 {
		t.Errorf("next on empty queue = %d, want -1", got)
	}
	if got := prevTrackIndex(0, 3); got != 2 {
		t.Errorf("prev(first of 3) = %d, want wrap to 2", got)
	}
	if got := prevTrackIndex(2, 3); got != 1 {
		t.Errorf("prev(last of 3) = %d, want 1", got)
	}
}

// TestQueueOpenAndJump drives a queue with tiny real mp3 files. A minimal
// but valid mp3: an MPEG audio frame header plus padding, enough for the
// decoder to identify the format. What is tested is the bookkeeping:
// current index, state fields, JumpTo, and that Open clears the queue.
func TestQueueOpenAndJump(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string) string {
		p := filepath.Join(dir, name)
		// One MPEG-1 Layer III frame at 128 kbps / 44.1 kHz: 417 bytes.
		frame := []byte{0xFF, 0xFB, 0x90, 0x00}
		for len(frame) < 417 {
			frame = append(frame, 0)
		}
		if err := os.WriteFile(p, frame, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// Named so sorting puts first before second.
	first := mk("01 - A - First.mp3")
	second := mk("02 - B - Second.mp3")

	paths, err := FolderTracks(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := New("")
	defer p.Close()
	if err := p.OpenQueue(paths); err != nil {
		t.Fatalf("OpenQueue: %v", err)
	}

	st := p.State()
	if !st.Queue || st.QueueLen != 2 {
		t.Fatalf("state = %+v, want queue of 2", st)
	}
	if st.Title != "First" || st.Artist != "A" {
		t.Fatalf("first track named (%q, %q), want (First, A)", st.Title, st.Artist)
	}
	if got := p.CurrentTrack(); got != 0 {
		t.Fatalf("current = %d, want 0", got)
	}
	tr := p.Tracks()
	if len(tr) != 2 || tr[1].Title != "Second" {
		t.Fatalf("Tracks() = %+v", tr)
	}

	// Jump to the second song.
	p.JumpTo(1)
	st = p.State()
	if st.Chapter != 1 || st.Title != "Second" {
		t.Fatalf("after JumpTo(1): chapter %d title %q, want 1 Second", st.Chapter, st.Title)
	}
	if st.Path != second {
		t.Fatalf("path = %q, want %q", st.Path, second)
	}

	// Out-of-range jumps are ignored, not fatal.
	p.JumpTo(9)
	if got := p.CurrentTrack(); got != 1 {
		t.Fatalf("JumpTo(9) moved to %d", got)
	}

	// Opening a single recording leaves queue mode.
	if err := p.Open(first); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if st := p.State(); st.Queue {
		t.Fatal("still in queue mode after Open of a single file")
	}
}

// TestQueueRealFolder runs against the user's real download folder when
// DRIFT_FOLDER points at it, as DRIFT_MIX does for the recording.
func TestQueueRealFolder(t *testing.T) {
	dir := os.Getenv("DRIFT_FOLDER")
	if dir == "" {
		t.Skip("set DRIFT_FOLDER to a folder of mp3s to run this")
	}
	paths, err := FolderTracks(dir)
	if err != nil {
		t.Fatalf("FolderTracks: %v", err)
	}
	if len(paths) < 2 {
		t.Fatalf("only %d tracks in %s", len(paths), dir)
	}

	p := New("")
	defer p.Close()
	if err := p.OpenQueue(paths); err != nil {
		t.Fatalf("OpenQueue: %v", err)
	}
	st := p.State()
	t.Logf("queue of %d, first: %q by %q, total %v", st.QueueLen, st.Title, st.Artist, st.Total)

	p.JumpTo(st.QueueLen - 1)
	if got := p.State().Chapter; got != st.QueueLen-1 {
		t.Fatalf("jump to last: chapter %d, want %d", got, st.QueueLen-1)
	}
}
