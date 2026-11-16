package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"

	"drift/internal/mix"
	"drift/internal/player"
)

// writeMinimalMP3 writes one valid MPEG-1 Layer III frame: enough for the
// decoder to identify the format. These tests check the session and the
// system's media controls, not the sound.
func writeMinimalMP3(t *testing.T, dir, name string) string {
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

// TestSessionFileRecordingRoundTrip proves the session written for a
// recording opens that recording on the next run.
func TestSessionFileRecordingRoundTrip(t *testing.T) {
	dir := t.TempDir()
	file := writeMinimalMP3(t, dir, "mix.mp3")
	path := filepath.Join(dir, "session.json")

	a := &app{player: player.New(), sessionPath: path}
	defer a.player.Close()
	if err := a.player.Open(file); err != nil {
		t.Fatalf("Open: %v", err)
	}
	a.saveSession()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session not written: %v", err)
	}

	b := &app{player: player.New(), sessionPath: path}
	defer b.player.Close()
	b.restoreSession()
	st := b.player.State()
	if !st.Loaded || st.Queue {
		t.Fatalf("restored state = %+v, want a loaded recording", st)
	}
	if st.Path != file {
		t.Fatalf("restored path = %q, want %q", st.Path, file)
	}
}

// TestSessionFileFolderRoundTrip proves the session written for a folder
// opens that folder on the next run, on the song that played.
func TestSessionFileFolderRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeMinimalMP3(t, dir, "01 - A - First.mp3")
	writeMinimalMP3(t, dir, "02 - B - Second.mp3")
	paths, err := player.FolderTracks(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "session.json")

	a := &app{player: player.New(), sessionPath: path}
	defer a.player.Close()
	if err := a.player.OpenQueue(paths); err != nil {
		t.Fatalf("OpenQueue: %v", err)
	}
	a.player.JumpTo(1)
	a.saveSession()

	b := &app{player: player.New(), sessionPath: path}
	defer b.player.Close()
	b.restoreSession()
	st := b.player.State()
	if !st.Queue || st.QueueLen != 2 {
		t.Fatalf("restored state = %+v, want a queue of 2", st)
	}
	if got := b.player.CurrentTrack(); got != 1 {
		t.Fatalf("restored current = %d, want 1", got)
	}
	if st.Title != "Second" {
		t.Fatalf("restored title = %q, want Second", st.Title)
	}
}

// TestRestoreSessionDropsGone proves a session whose file has gone leaves the
// player empty, rather than half loaded or panicking.
func TestRestoreSessionDropsGone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")
	b, err := json.Marshal(player.Session{Kind: "recording", Path: filepath.Join(dir, "gone.mp3")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	a := &app{player: player.New(), sessionPath: path}
	defer a.player.Close()
	a.restoreSession()
	if a.player.State().Loaded {
		t.Fatal("a gone recording loaded")
	}
}

// TestSyncNowPlayingPublishesRecording proves the chapter's title and artist
// reach the system's media controls.
func TestSyncNowPlayingPublishesRecording(t *testing.T) {
	dir := t.TempDir()
	file := writeMinimalMP3(t, dir, "mix.mp3")
	a := &app{player: player.New()}
	defer a.player.Close()
	if err := a.player.Open(file); err != nil {
		t.Fatalf("Open: %v", err)
	}
	a.syncNowPlaying()

	info, ok := mygo.NowPlaying.Info()
	if !ok {
		t.Fatal("NowPlaying showed nothing")
	}
	ch := mix.Chapters[0]
	if info.Title != ch.Name() || info.Artist != ch.Artist {
		t.Fatalf("NowPlaying = %q by %q, want %q by %q", info.Title, info.Artist, ch.Name(), ch.Artist)
	}
	if info.State != mygo.PlaybackPaused {
		t.Fatalf("NowPlaying state = %v, want paused", info.State)
	}
}

// TestSyncNowPlayingPublishesQueueTrack proves a folder's song reaches the
// system's media controls with the name parsed from its file.
func TestSyncNowPlayingPublishesQueueTrack(t *testing.T) {
	dir := t.TempDir()
	writeMinimalMP3(t, dir, "01 - A - First.mp3")
	writeMinimalMP3(t, dir, "02 - B - Second.mp3")
	paths, err := player.FolderTracks(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &app{player: player.New()}
	defer a.player.Close()
	if err := a.player.OpenQueue(paths); err != nil {
		t.Fatalf("OpenQueue: %v", err)
	}
	a.syncNowPlaying()

	info, ok := mygo.NowPlaying.Info()
	if !ok {
		t.Fatal("NowPlaying showed nothing")
	}
	if info.Title != "First" || info.Artist != "A" {
		t.Fatalf("NowPlaying = %q by %q, want First by A", info.Title, info.Artist)
	}
}

// TestSyncNowPlayingClearsWhenEmpty proves the system's controls stop showing
// the app once nothing is loaded.
func TestSyncNowPlayingClearsWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	file := writeMinimalMP3(t, dir, "mix.mp3")
	a := &app{player: player.New()}
	if err := a.player.Open(file); err != nil {
		t.Fatalf("Open: %v", err)
	}
	a.syncNowPlaying()
	if _, ok := mygo.NowPlaying.Info(); !ok {
		t.Fatal("NowPlaying showed nothing to clear")
	}

	a.player.Close()
	a.syncNowPlaying()
	if _, ok := mygo.NowPlaying.Info(); ok {
		t.Fatal("NowPlaying still shows a track after clearing")
	}
}
