package view

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"drift/internal/player"
)

// TestNonMixRecordingShowsItsName builds the real screen with a recording
// that is not the 61-minute mix. The popup must show that file's own name
// and none of the mix's chapters, which belong to another file.
func TestNonMixRecordingShowsItsName(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "brown-noise.mp3")
	frame := []byte{0xFF, 0xFB, 0x90, 0x00}
	for len(frame) < 417 {
		frame = append(frame, 0)
	}
	if err := os.WriteFile(file, frame, 0o644); err != nil {
		t.Fatal(err)
	}

	p := player.New()
	if err := p.Open(file); err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(p.Close)

	var scroll ui.ScrollState
	angle := float32(0)
	last := time.Now()
	m := &Model{
		Player: p, Scroll: &scroll, Angle: &angle, Last: &last,
		Choose: func() {}, OnPlay: func() {}, OnPrev: func() {},
		OnNext: func() {}, OnSeek: func(float64) {},
	}
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Themed(c, ThemeFor(c.Theme().Dark)).Fill().Children(func() {
			c.Root().Background(c.Theme().Background)
			ui.Column(c).Fill().Padding(16).Children(func() {
				Screen(c, m)
			})
		})
	}, 320, 585)
	tester.Frame()
	tester.Frame()

	if !tester.HasText("brown-noise") {
		t.Errorf("the recording's own name is not on screen; texts: %v", tester.Texts())
	}
	if tester.HasText("Snowman") {
		t.Errorf("the mix's chapter list is on screen for another recording")
	}
}

// TestRecordingTitle pins the name a recording shows: its file name without
// the extension, and a placeholder when nothing is loaded.
func TestRecordingTitle(t *testing.T) {
	if got := recordingTitle(player.State{Path: `C:\music\brown noise.mp3`}); got != "brown noise" {
		t.Errorf("recordingTitle = %q, want %q", got, "brown noise")
	}
	if got := recordingTitle(player.State{}); got != "Nothing playing" {
		t.Errorf("recordingTitle of nothing = %q, want %q", got, "Nothing playing")
	}
}
