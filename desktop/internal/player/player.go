// Package player owns playback: the recording, where it is, and what is playing.
// It outlives the popup, so the music keeps going when the window closes.
package player

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/audio"
	// MP3 and Ogg register their decoders when imported for effect.
	_ "github.com/egoist/mygo/audio/mp3"
	_ "github.com/egoist/mygo/audio/vorbis"

	"drift/internal/mix"
)

// ErrNoFile means the user has not chosen a recording yet.
var ErrNoFile = errors.New("no recording chosen")

// State is what the popup draws.
type State struct {
	Loaded   bool
	Path     string
	Playing  bool
	Chapter  int
	Position time.Duration
	Total    time.Duration
}

// Player is the transport. Every field is guarded by mu, because the audio
// callbacks and the view run on different goroutines.
type Player struct {
	mu sync.Mutex

	file     *os.File
	stream   *audio.Player
	path     string
	total    time.Duration
	volume   float64
	unsub    func()
	onChange func()

	// The host folder for the remembered path, set once at startup.
	configPath string
}

// New returns a player with a remembered-path file at configPath.
func New(configPath string) *Player {
	return &Player{volume: 0.8, configPath: configPath}
}

// OnChange registers the function called whenever the visible state changes,
// so the popup can redraw. It is called without the lock held.
func (p *Player) OnChange(fn func()) {
	p.mu.Lock()
	p.onChange = fn
	p.mu.Unlock()
}

func (p *Player) changed() {
	p.mu.Lock()
	fn := p.onChange
	p.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// RememberedPath returns the path saved from a previous run, or "".
func (p *Player) RememberedPath() string {
	if p.configPath == "" {
		return ""
	}
	b, err := os.ReadFile(p.configPath)
	if err != nil {
		return ""
	}
	path := strings.TrimSpace(string(b))
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

func (p *Player) remember(path string) {
	if p.configPath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p.configPath), 0o755)
	_ = os.WriteFile(p.configPath, []byte(path), 0o644)
}

// Open loads a recording and prepares to play it. It replaces whatever was
// loaded. Nothing plays until Play is called.
func (p *Player) Open(path string) error {
	if path == "" {
		return ErrNoFile
	}
	if err := supported(path); err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open the recording: %w", err)
	}

	stream, err := audio.OpenStream(f)
	if err != nil {
		f.Close()
		return fmt.Errorf("cannot read the recording: %w", err)
	}

	p.Close()

	p.mu.Lock()
	p.file = f
	p.stream = stream
	p.path = path
	p.total = stream.Duration()
	p.unsub = stream.OnEnd(p.changed)
	stream.SetVolume(p.volume)
	p.mu.Unlock()

	p.remember(path)
	p.changed()
	return nil
}

// supported rejects the formats the decoder cannot read, with the reason,
// instead of failing later as silence.
func supported(path string) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".ogg", ".oga", ".wav":
		return nil
	case ".m4a", ".aac", ".mp4":
		return fmt.Errorf("%s is AAC, which the audio decoder cannot read. Convert it to MP3 or Ogg.",
			filepath.Base(path))
	default:
		return fmt.Errorf("%s is not a format this app can read. Use MP3, Ogg or WAV.",
			filepath.Base(path))
	}
}

// Close stops playback and releases the file.
func (p *Player) Close() {
	p.mu.Lock()
	stream, file, unsub := p.stream, p.file, p.unsub
	p.stream, p.file, p.unsub = nil, nil, nil
	p.mu.Unlock()

	if unsub != nil {
		unsub()
	}
	if stream != nil {
		stream.Close()
	}
	if file != nil {
		file.Close()
	}
}

// Play starts or resumes playback.
func (p *Player) Play() {
	p.mu.Lock()
	s := p.stream
	p.mu.Unlock()
	if s == nil {
		return
	}
	s.Play()
	p.changed()
}

// Pause stops playback where it is.
func (p *Player) Pause() {
	p.mu.Lock()
	s := p.stream
	p.mu.Unlock()
	if s == nil {
		return
	}
	s.Pause()
	p.changed()
}

// Toggle plays or pauses.
func (p *Player) Toggle() {
	if p.State().Playing {
		p.Pause()
		return
	}
	p.Play()
}

// Seek moves to d, clamped to the recording.
func (p *Player) Seek(d time.Duration) {
	p.mu.Lock()
	s, total := p.stream, p.total
	p.mu.Unlock()
	if s == nil {
		return
	}
	if d < 0 {
		d = 0
	}
	if d > total {
		d = total
	}
	s.Seek(d)
	p.changed()
}

// Next jumps to the following chapter, or back to the first after the last.
func (p *Player) Next() {
	st := p.State()
	if !st.Loaded {
		return
	}
	next := st.Chapter + 1
	if next >= len(mix.Chapters) {
		p.Seek(0)
		return
	}
	p.Seek(mix.Chapters[next].At)
}

// Prev rewinds the chapter first, then goes back one, as players do.
func (p *Player) Prev() {
	st := p.State()
	if !st.Loaded {
		return
	}
	// More than three seconds in: restart this chapter instead of leaving it.
	if st.Position-mix.Chapters[st.Chapter].At > 3*time.Second {
		p.Seek(mix.Chapters[st.Chapter].At)
		return
	}
	if st.Chapter == 0 {
		p.Seek(0)
		return
	}
	p.Seek(mix.Chapters[st.Chapter-1].At)
}

// JumpTo plays chapter i from its start.
func (p *Player) JumpTo(i int) {
	if i < 0 || i >= len(mix.Chapters) {
		return
	}
	p.Seek(mix.Chapters[i].At)
	p.Play()
}

// SetVolume takes 0 to 1.
func (p *Player) SetVolume(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.mu.Lock()
	p.volume = v
	s := p.stream
	p.mu.Unlock()
	if s != nil {
		s.SetVolume(v)
	}
}

// Volume returns the current volume, 0 to 1.
func (p *Player) Volume() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

// State reads the transport for the view. It is cheap enough to call on
// every frame.
func (p *Player) State() State {
	p.mu.Lock()
	s, total, path := p.stream, p.total, p.path
	p.mu.Unlock()

	if s == nil {
		return State{}
	}

	pos := s.Position()
	return State{
		Loaded:   true,
		Path:     path,
		Playing:  s.Playing(),
		Chapter:  mix.IndexAt(pos),
		Position: pos,
		Total:    total,
	}
}
