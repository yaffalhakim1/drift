// Package player owns playback: the recording, where it is, and what is playing.
// It outlives the popup, so the music keeps going when the window closes.
//
// Two modes share one transport. Recording mode plays one long file whose
// chapters come from internal/mix. Queue mode plays a folder of songs in
// order and advances on its own when one ends.
package player

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

	// Queue mode: what is playing came from a folder of songs.
	Queue    bool
	QueueLen int
	Title    string
	Artist   string
}

// Track is one song in the queue, named for the screen.
type Track struct {
	Title  string
	Artist string
	Path   string
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

	// Queue mode. queue holds the paths; tracks the parsed names, same
	// order; current is the index playing now.
	queue   []string
	tracks  []Track
	current int

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

// Forget clears the remembered path. Choosing a folder clears the old
// single-recording memory, so exactly one source wins at startup.
func (p *Player) Forget() {
	p.remember("")
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

// FolderTracks lists the playable files of a folder, sorted by name.
// Unreadable entries are skipped: one stray file must not empty the list.
func FolderTracks(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read the folder: %w", err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if supported(p) == nil {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("%s has no MP3, Ogg or WAV files", filepath.Base(dir))
	}
	sort.Strings(paths)
	return paths, nil
}

// parseQueueName turns a downloaded filename into artist and title.
// "003 - Ed Sheeran - Photograph (Official Video).mp3" becomes
// title "Photograph", artist "Ed Sheeran". Anything it cannot read comes
// back whole, cleaned, as the title.
func parseQueueName(name string) (title, artist string) {
	s := strings.TrimSuffix(name, filepath.Ext(name))
	s = strings.TrimSpace(s)

	// yt-dlp numbers its files: "003 - Artist - Title".
	if i := strings.Index(s, " - "); i > 0 && isDigits(strings.TrimSpace(s[:i])) {
		s = strings.TrimSpace(s[i+3:])
	}

	parts := strings.SplitN(s, " - ", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		artist, title = parts[0], parts[1]
	} else {
		title = s
	}

	title = stripVideoMarkers(title)
	if title == "" {
		title = strings.TrimSpace(s)
	}
	return title, artist
}

// stripVideoMarkers removes the "(Official Music Video)" decorations that
// downloaders put in titles. The name of the file stays untouched.
func stripVideoMarkers(s string) string {
	for {
		open := strings.LastIndexAny(s, "([")
		if open < 0 {
			return strings.TrimSpace(s)
		}
		closeM := strings.IndexAny(s[open:], ")]")
		if closeM < 0 {
			return strings.TrimSpace(s)
		}
		inner := strings.ToLower(s[open+1 : open+closeM])
		if !strings.Contains(inner, "video") && !strings.Contains(inner, "audio") &&
			!strings.Contains(inner, "lyric") && !strings.Contains(inner, "official") &&
			!strings.Contains(inner, "hd") {
			return strings.TrimSpace(s)
		}
		s = strings.TrimSpace(s[:open] + s[open+closeM+1:])
	}
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// openStream loads a file and its decoder without touching memory of it.
// Every open path funnels through here so the bookkeeping lives in one place.
func (p *Player) openStream(path string) (*os.File, *audio.Player, time.Duration, error) {
	if path == "" {
		return nil, nil, 0, ErrNoFile
	}
	if err := supported(path); err != nil {
		return nil, nil, 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("cannot open the recording: %w", err)
	}
	stream, err := audio.OpenStream(f)
	if err != nil {
		f.Close()
		return nil, nil, 0, fmt.Errorf("cannot read the recording: %w", err)
	}
	return f, stream, stream.Duration(), nil
}

// swap replaces the playing stream, releasing the old one. Callers hold mu.
func (p *Player) swap(f *os.File, stream *audio.Player, total time.Duration, path string) {
	if p.unsub != nil {
		p.unsub()
	}
	if p.stream != nil {
		p.stream.Close()
	}
	if p.file != nil {
		p.file.Close()
	}
	p.file, p.stream, p.total, p.path = f, stream, total, path
}

// Open loads a recording and prepares to play it. It replaces whatever was
// loaded. Nothing plays until Play is called.
func (p *Player) Open(path string) error {
	if path == "" {
		return ErrNoFile
	}
	f, stream, total, err := p.openStream(path)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.queue, p.tracks, p.current = nil, nil, 0
	p.swap(f, stream, total, path)
	p.unsub = stream.OnEnd(p.changed)
	stream.SetVolume(p.volume)
	p.mu.Unlock()

	p.remember(path)
	p.changed()
	return nil
}

// OpenQueue loads a folder of songs as the playlist and starts the first.
func (p *Player) OpenQueue(paths []string) error {
	if len(paths) == 0 {
		return ErrNoFile
	}
	tracks := make([]Track, len(paths))
	for i, path := range paths {
		title, artist := parseQueueName(filepath.Base(path))
		tracks[i] = Track{Title: title, Artist: artist, Path: path}
	}

	if err := p.openIndexLockedSetup(paths[0]); err != nil {
		return err
	}

	p.mu.Lock()
	p.queue = append([]string(nil), paths...)
	p.tracks = tracks
	p.current = 0
	p.mu.Unlock()

	p.remember("")
	p.changed()
	return nil
}

// openIndexLockedSetup opens the stream for a queue's first track before the
// queue fields are set, so a failed first song leaves the player untouched.
func (p *Player) openIndexLockedSetup(path string) error {
	f, stream, total, err := p.openStream(path)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.swap(f, stream, total, path)
	p.unsub = stream.OnEnd(p.onTrackEnd)
	stream.SetVolume(p.volume)
	return nil
}

// onTrackEnd advances the queue when a song finishes. MyGo fires OnEnd on
// its own goroutine, so this takes the lock like every other entry point.
func (p *Player) onTrackEnd() {
	p.mu.Lock()
	queue := len(p.queue) > 0
	next := p.current + 1
	ended := !queue || next >= len(p.queue)
	p.mu.Unlock()

	if ended {
		p.changed()
		return
	}
	if err := p.openIndex(next); err != nil {
		// The next file vanished between listing and playing. Stop rather
		// than skip the rest silently; the user sees the error by playing.
		p.changed()
		return
	}
	p.Play()
}

// openIndex starts playing queue song i.
func (p *Player) openIndex(i int) error {
	p.mu.Lock()
	if i < 0 || i >= len(p.queue) {
		p.mu.Unlock()
		return ErrNoFile
	}
	path := p.queue[i]
	p.mu.Unlock()

	f, stream, total, err := p.openStream(path)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.swap(f, stream, total, path)
	p.current = i
	p.unsub = stream.OnEnd(p.onTrackEnd)
	stream.SetVolume(p.volume)
	p.mu.Unlock()

	p.changed()
	return nil
}

// Tracks returns the queue's songs for the screen. Empty in recording mode.
func (p *Player) Tracks() []Track {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Track(nil), p.tracks...)
}

// CurrentTrack returns the index of the playing song in the queue.
func (p *Player) CurrentTrack() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.current
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

// nextTrackIndex returns where Next should go in queue mode, or -1 to
// stay. The last song wraps to the first, as the chapter list does.
func nextTrackIndex(current, n int) int {
	if n == 0 {
		return -1
	}
	return (current + 1) % n
}

// prevTrackIndex returns where Prev should go in queue mode. Restarting the
// current song when it is partway in is the view's job, as with chapters.
func prevTrackIndex(current, n int) int {
	if n == 0 {
		return -1
	}
	if current <= 0 {
		return n - 1
	}
	return current - 1
}

// Next jumps to the following chapter or song, or back to the first after
// the last.
func (p *Player) Next() {
	st := p.State()
	if !st.Loaded {
		return
	}
	if st.Queue {
		if i := nextTrackIndex(p.CurrentTrack(), st.QueueLen); i >= 0 {
			if err := p.openIndex(i); err == nil {
				p.Play()
			}
		}
		return
	}
	next := st.Chapter + 1
	if next >= len(mix.Chapters) {
		p.Seek(0)
		return
	}
	p.Seek(mix.Chapters[next].At)
}

// Prev rewinds the chapter or song first, then goes back one, as players do.
func (p *Player) Prev() {
	st := p.State()
	if !st.Loaded {
		return
	}
	if st.Queue {
		// More than three seconds in: restart this song instead of leaving it.
		if st.Position > 3*time.Second {
			p.Seek(0)
			return
		}
		if i := prevTrackIndex(p.CurrentTrack(), st.QueueLen); i >= 0 {
			if err := p.openIndex(i); err == nil {
				p.Play()
			}
		}
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

// JumpTo plays queue song i from its start.
func (p *Player) JumpTo(i int) {
	p.mu.Lock()
	inQueue := i >= 0 && i < len(p.queue)
	p.mu.Unlock()
	if !inQueue {
		return
	}
	if err := p.openIndex(i); err == nil {
		p.Play()
	}
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
	queue, tracks, current := len(p.queue) > 0, len(p.tracks), p.current
	var track Track
	if queue && current < tracks {
		track = p.tracks[current]
	}
	p.mu.Unlock()

	if s == nil {
		return State{}
	}

	pos := s.Position()
	st := State{
		Loaded:   true,
		Path:     path,
		Playing:  s.Playing(),
		Position: pos,
		Total:    total,
		Queue:    queue,
		QueueLen: tracks,
		Title:    track.Title,
		Artist:   track.Artist,
	}
	if queue {
		st.Chapter = current
	} else {
		st.Chapter = mix.IndexAt(pos)
	}
	return st
}
