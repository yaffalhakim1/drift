package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"drift/internal/mix"
	"drift/internal/player"
	"drift/internal/view"
)

// trayIcon is MyGo's default app icon: the blue-violet mark with a ring that
// the installer wears too, at 32 px for the notification area. It is not a
// template image, so neither macOS nor Windows tints it.
//
//go:embed resources/tray.png
var trayIcon []byte

// app holds the state the tray and the popup share. Everything here is read
// and written on the main thread, by tray callbacks and the view.
type app struct {
	player *player.Player
	panel  *mygo.Window
	tray   *mygo.Tray

	scroll ui.ScrollState
	angle  float32
	last   time.Time

	// The queue list's place and its chosen row, kept between frames in
	// app state (docs/ui/views.md: widget state belongs in the app).
	// sel IS the playing track: the list shows one choice, never two.
	list ui.ListState
	sel  int

	// theme is "system", "light" or "dark", as the user last chose.
	theme string

	// settingsPath is where the theme is remembered, and sessionPath where
	// playback is, so the next run picks it up.
	settingsPath string
	sessionPath  string
	savedSession string

	// npKey is the track the system's media controls last showed, so a
	// seek or a pause updates it instead of publishing it again.
	npKey string
}

// model is the state the screen renders from, rebuilt each frame because
// elements do not survive between frames but the callbacks do.
func (a *app) model() *view.Model {
	return &view.Model{
		Player:        a.player,
		Scroll:        &a.scroll,
		Angle:         &a.angle,
		Last:          &a.last,
		List:          &a.list,
		ListSelection: &a.sel, // pointer into app state
		Theme:         a.theme,
		Choose:        a.choose,
		OnTheme:       a.setTheme,
		OnPlay:        a.player.Toggle,
		OnPrev:        a.player.Prev,
		OnNext:        a.player.Next,
		OnSeek: func(f float64) {
			st := a.player.State()
			a.player.Seek(time.Duration(float64(st.Total) * f))
		},
	}
}

// setTheme changes the appearance, and remembers it for the next run.
func (a *app) setTheme(next string) {
	switch next {
	case "light":
		a.theme = "light"
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		a.theme = "dark"
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		a.theme = "system"
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
	a.saveSettings()
}

// render draws the popup.
//
// Themed must be given Fill(): coreThemed builds a plain column, and a column
// that is asked for no height takes its content's, which removes the bound the
// chapter list needs to scroll inside. Without Fill the list measured 1120 px
// in a 585 px window and never scrolled.
func (a *app) render(c *ui.Context) {
	ui.Themed(c, view.ThemeFor(c.Theme().Dark)).Fill().Children(func() {
		c.Root().Background(c.Theme().Background)
		view.Screen(c, a.model())
	})
}

// toggle opens the popup under the tray icon, or closes it.
func (a *app) toggle() {
	if a.panel != nil {
		a.panel.Close()
		return
	}
	a.panel = mygo.NewFlyout(mygo.FlyoutOptions{
		Tray:      a.tray,
		Placement: mygo.PlacementBottom,
		Gap:       4,
		Width:     320,
		Height:    585,
		Focusable: true,
		Shadow:    true,
		Content:   ui.View(a.render),
	})
	a.panel.OnClosed(func() { a.panel = nil })
}

// choose asks for the recording, then opens it.
func (a *app) choose() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title: "Choose your lo-fi mix",
		Filters: []mygo.FileFilter{
			{Name: "Audio", Extensions: []string{"mp3", "ogg", "oga", "wav"}},
		},
	})
	if err != nil || len(paths) == 0 {
		return
	}
	if err := a.player.Open(paths[0]); err != nil {
		mygo.Dialog.Error("Cannot play that file", err.Error())
		return
	}
	a.player.Play()
	// One source wins: the recording replaces any folder, in memory and in
	// the session the next run reads.
	a.saveSession()
}

// chooseFolder asks for a folder of songs and plays it in order.
func (a *app) chooseFolder() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:       "Choose a folder of music",
		ButtonLabel: "Play this folder",
		Directory:   true,
	})
	if err != nil || len(paths) == 0 {
		return
	}
	dir := paths[0]
	files, err := player.FolderTracks(dir)
	if err != nil {
		mygo.Dialog.Error("Cannot use that folder", err.Error())
		return
	}
	if err := a.player.OpenQueue(files); err != nil {
		mygo.Dialog.Error("Cannot play that folder", err.Error())
		return
	}
	// The dialog's button says "Play this folder", so it plays.
	a.player.Play()
	a.saveSession()
}

// saveSettings remembers the theme beside the session.
func (a *app) saveSettings() {
	if a.settingsPath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(a.settingsPath), 0o755)
	_ = os.WriteFile(a.settingsPath, []byte(a.theme+"\n"), 0o644)
}

// saveSession writes where playback is, so the next run picks it up. It
// writes a part file and renames it, so a crash mid-write leaves the old
// session rather than half of a new one.
func (a *app) saveSession() {
	if a.sessionPath == "" || a.player == nil {
		return
	}
	s, ok := a.player.Session()
	if !ok {
		return
	}
	b, err := json.Marshal(s)
	if err != nil || string(b) == a.savedSession {
		return
	}
	_ = os.MkdirAll(filepath.Dir(a.sessionPath), 0o755)
	if os.WriteFile(a.sessionPath+".part", b, 0o644) == nil &&
		os.Rename(a.sessionPath+".part", a.sessionPath) == nil {
		a.savedSession = string(b)
	}
}

// restoreSession opens what played last time, paused. A file or folder that
// has gone is dropped, and the popup starts empty.
func (a *app) restoreSession() {
	if a.sessionPath == "" {
		return
	}
	b, err := os.ReadFile(a.sessionPath)
	if err != nil {
		return
	}
	var s player.Session
	if json.Unmarshal(b, &s) != nil {
		return
	}
	if err := a.player.Restore(s); err != nil {
		log.Println("session from last time could not be opened:", err)
	}
}

// loadSettings reads the theme chosen last time, defaulting to the system.
func (a *app) loadSettings() {
	a.theme = "system"
	if a.settingsPath == "" {
		return
	}
	b, err := os.ReadFile(a.settingsPath)
	if err != nil {
		return
	}
	switch t := string(bytes.TrimSpace(b)); t {
	case "light", "dark", "system":
		a.theme = t
	}
}

// ready builds the tray, picks up what played last time, and wires the menu.
// It runs once the app is up.
func (a *app) ready() {
	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err == nil {
		a.settingsPath = filepath.Join(dataDir, "settings")
		a.sessionPath = filepath.Join(dataDir, "session.json")
	}
	a.loadSettings()
	a.player = player.New()

	// Apply the saved appearance before anything draws.
	a.setTheme(a.theme)

	// Pick up where playback was left, paused, and show it to the system.
	a.restoreSession()
	a.wireNowPlaying()

	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           trayIcon,
		IconIsTemplate: false,
		ToolTip:        "drift",
	})
	if err != nil {
		log.Fatal("tray: ", err)
	}
	a.tray = tray

	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Open drift", Click: func(*mygo.MenuItem, *mygo.Window) { a.toggle() }},
		{Label: "Choose mix…", Click: func(*mygo.MenuItem, *mygo.Window) { a.choose() }},
		{Label: "Choose folder of music…", Click: func(*mygo.MenuItem, *mygo.Window) { a.chooseFolder() }},
		mygo.Separator(),
		{Label: "Play or pause", Click: func(*mygo.MenuItem, *mygo.Window) { a.player.Toggle() }},
		{Label: "Next chapter", Click: func(*mygo.MenuItem, *mygo.Window) { a.player.Next() }},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})

	if runtime.GOOS == "linux" {
		// AppIndicator only shows menus, and tells nothing of clicks.
		tray.SetMenu(menu)
	} else {
		tray.OnClick(a.toggle)
		tray.OnRightClick(func() {
			if a.panel != nil {
				a.panel.Close()
			}
			tray.SetMenu(menu)
			defer tray.SetMenu(nil)
			tray.PopUpMenu()
		})
	}

	// A second launch opens the popup of the one already running.
	mygo.App.OnSecondInstance(func([]string, string) {
		if a.panel == nil {
			a.toggle()
		}
	})

	// Once a second, keep the system's media controls on the chapter that
	// plays, and every tenth second keep the session, in case the app does
	// not get to quit. RunOnMain keeps both on the thread the rest of the
	// app state lives on.
	go func() {
		t := time.NewTicker(time.Second)
		n := 0
		for range t.C {
			n++
			save := n%10 == 0
			mygo.RunOnMain(func() {
				// A new chapter is a new title; the system advances the
				// position itself, so nothing else is due each second.
				if st := a.player.State(); st.Playing && nowPlayingKey(st) != a.npKey {
					a.syncNowPlaying()
				}
				if save {
					a.saveSession()
				}
			})
		}
	}()

	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		a.saveSession()
	})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		a.player.Close()
		tray.Destroy()
	})
}

// wireNowPlaying shows what plays in the system's media controls and takes
// the media keys, through MyGo's NowPlaying: the volume flyout and the lock
// screen on Windows, Control Center on macOS, MPRIS on Linux.
func (a *app) wireNowPlaying() {
	mygo.NowPlaying.OnCommand(func(cmd mygo.MediaCommand, pos time.Duration) {
		switch cmd {
		case mygo.MediaPlay, mygo.MediaPause:
			if (cmd == mygo.MediaPlay) != a.player.State().Playing {
				a.player.Toggle()
			}
		case mygo.MediaNext:
			a.player.Next()
		case mygo.MediaPrevious:
			a.player.Prev()
		case mygo.MediaSeek:
			a.player.Seek(pos)
		}
		a.syncNowPlaying()
	})
	// The player changes on its own goroutine; the system's controls are
	// updated on the main thread, as the rest of the app state is.
	a.player.OnChange(func() { mygo.RunOnMain(a.syncNowPlaying) })
	a.syncNowPlaying()
}

// nowPlayingKey is what the system shows: the file, and the chapter in a
// recording, so a new chapter is a new title.
func nowPlayingKey(st player.State) string {
	if !st.Queue && st.Chapter >= 0 && st.Chapter < len(mix.Chapters) {
		return fmt.Sprintf("%s#%d", st.Path, st.Chapter)
	}
	return st.Path
}

// syncNowPlaying publishes the current track to the system. A new track is
// set whole; a seek or a pause only moves the state and the position, so
// the system is not handed the same title again on every change.
func (a *app) syncNowPlaying() {
	st := a.player.State()
	if !st.Loaded {
		if a.npKey != "" {
			mygo.NowPlaying.Clear()
			a.npKey = ""
		}
		return
	}

	state := mygo.PlaybackPaused
	if st.Playing {
		state = mygo.PlaybackPlaying
	}
	if key := nowPlayingKey(st); key != a.npKey {
		a.npKey = key
		title, artist := st.Title, st.Artist
		if !st.Queue && st.Chapter >= 0 && st.Chapter < len(mix.Chapters) {
			ch := mix.Chapters[st.Chapter]
			title, artist = ch.Name(), ch.Artist
		}
		mygo.NowPlaying.Set(mygo.NowPlayingInfo{
			Title:    title,
			Artist:   artist,
			Duration: st.Total,
			Position: st.Position,
			State:    state,
			Commands: mygo.MediaPlay | mygo.MediaPause | mygo.MediaNext | mygo.MediaPrevious | mygo.MediaSeek,
		})
		return
	}
	mygo.NowPlaying.SetPlayback(state, st.Position)
}

func main() {
	// A second launch opens the running app's popup instead of starting a
	// second tray icon. Asked before anything else touches the data.
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	a := &app{}

	mygo.App.SetName("drift")
	// A tray app has no Dock icon and no windows of its own until asked.
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	// The popup is the only window: keep running as it closes.
	mygo.App.OnWindowAllClosed(func() {})

	mygo.App.WhenReady(a.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
