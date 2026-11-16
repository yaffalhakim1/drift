package player

import (
	"os"
	"path/filepath"
	"testing"
)

// writeMP3 writes one valid MPEG-1 Layer III frame, enough for the decoder
// to identify the format. What the session tests check is the bookkeeping,
// not the sound.
func writeMP3(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	frame := []byte{0xFF, 0xFB, 0x90, 0x00}
	for len(frame) < 417 {
		frame = append(frame, 0)
	}
	if err := os.WriteFile(p, frame, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestSessionFolderRoundTrip pins that a folder session names the folder and
// the song, and that restoring it opens the same song, paused.
func TestSessionFolderRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeMP3(t, dir, "01 - A - First.mp3")
	writeMP3(t, dir, "02 - B - Second.mp3")

	paths, err := FolderTracks(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := New()
	defer p.Close()
	if err := p.OpenQueue(paths); err != nil {
		t.Fatalf("OpenQueue: %v", err)
	}
	p.JumpTo(1)

	s, ok := p.Session()
	if !ok {
		t.Fatal("Session() reported nothing loaded")
	}
	if s.Kind != "folder" || s.Path != dir || s.Index != 1 {
		t.Fatalf("Session() = %+v, want folder %q index 1", s, dir)
	}

	q := New()
	defer q.Close()
	if err := q.Restore(s); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	st := q.State()
	if !st.Queue || st.QueueLen != 2 {
		t.Fatalf("restored state = %+v, want a queue of 2", st)
	}
	if got := q.CurrentTrack(); got != 1 {
		t.Fatalf("restored current = %d, want 1", got)
	}
	if st.Title != "Second" {
		t.Fatalf("restored title = %q, want Second", st.Title)
	}
	if st.Playing {
		t.Fatal("restored playback is playing, want paused")
	}
}

// TestSessionRecordingRoundTrip pins that a recording session names the file,
// and that restoring it opens the same file.
func TestSessionRecordingRoundTrip(t *testing.T) {
	dir := t.TempDir()
	file := writeMP3(t, dir, "mix.mp3")

	p := New()
	defer p.Close()
	if err := p.Open(file); err != nil {
		t.Fatalf("Open: %v", err)
	}

	s, ok := p.Session()
	if !ok {
		t.Fatal("Session() reported nothing loaded")
	}
	if s.Kind != "recording" || s.Path != file {
		t.Fatalf("Session() = %+v, want recording %q", s, file)
	}

	q := New()
	defer q.Close()
	if err := q.Restore(s); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	st := q.State()
	if !st.Loaded || st.Queue {
		t.Fatalf("restored state = %+v, want a loaded recording", st)
	}
	if st.Path != file {
		t.Fatalf("restored path = %q, want %q", st.Path, file)
	}
}

// TestSessionRestoreRejectsGone pins that a session whose file or folder has
// gone returns an error, so the app starts empty instead of half loaded.
func TestSessionRestoreRejectsGone(t *testing.T) {
	p := New()
	defer p.Close()

	if err := p.Restore(Session{Kind: "recording", Path: filepath.Join(t.TempDir(), "gone.mp3")}); err == nil {
		t.Fatal("Restore of a missing recording returned nil")
	}
	if err := p.Restore(Session{Kind: "folder", Path: filepath.Join(t.TempDir(), "gone")}); err == nil {
		t.Fatal("Restore of a missing folder returned nil")
	}
	if err := p.Restore(Session{Kind: "bogus"}); err != ErrNoFile {
		t.Fatalf("Restore of an unknown kind = %v, want ErrNoFile", err)
	}
}
