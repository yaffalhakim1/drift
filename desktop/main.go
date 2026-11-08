package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"drift/internal/player"
	"drift/internal/view"
)

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

	// settingsPath is where the theme is remembered.
	// folderPath remembers the chosen folder of songs.
	settingsPath string
	folderPath   string
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
	a.player.Forget() // one source wins: the recording, not an old folder
	// Clear the folder memory too, on disk: a stale last-folder would
	// otherwise come back over this recording on the next launch.
	a.folderPath = ""
	a.saveFolder()
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
	a.folderPath = dir
	a.saveFolder()
}

// saveSettings remembers the theme beside the recording path.
func (a *app) saveSettings() {
	if a.settingsPath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(a.settingsPath), 0o755)
	_ = os.WriteFile(a.settingsPath, []byte(a.theme+"\n"), 0o644)
}

// saveFolder remembers the folder of songs for the next run.
func (a *app) saveFolder() {
	if a.settingsPath == "" {
		return
	}
	_ = os.WriteFile(filepath.Join(filepath.Dir(a.settingsPath), "last-folder"), []byte(a.folderPath+"\n"), 0o644)
}

// loadFolder reads the folder chosen last time. A folder that went away,
// or that now has no songs, is dropped rather than offered.
func (a *app) loadFolder() string {
	if a.settingsPath == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(a.settingsPath), "last-folder"))
	if err != nil {
		return ""
	}
	dir := strings.TrimSpace(string(b))
	if dir == "" {
		return ""
	}
	if files, err := player.FolderTracks(dir); err != nil || len(files) == 0 {
		return ""
	}
	return dir
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

// ready builds the tray, loads what played last time, and wires the menu.
// It runs once the app is up.
func (a *app) ready() {
	dataDir, err := mygo.App.Path(mygo.PathUserData)
	var lastPath string
	if err == nil {
		lastPath = filepath.Join(dataDir, "last-recording")
		a.settingsPath = filepath.Join(dataDir, "settings")
	}
	a.loadSettings()

	// One source wins: a remembered folder beats a remembered recording.
	folder := a.loadFolder()
	a.folderPath = folder
	a.player = player.New(lastPath)

	// Apply the saved appearance before anything draws.
	a.setTheme(a.theme)

	if folder != "" {
		files, err := player.FolderTracks(folder)
		if err == nil {
			if err := a.player.OpenQueue(files); err != nil {
				log.Println("folder from last time could not be opened:", err)
			}
		}
	} else if p := a.player.RememberedPath(); p != "" {
		// Open the recording from the previous run, so the popup has
		// something to show the first time it opens.
		if err := a.player.Open(p); err != nil {
			log.Println("recording from last time could not be opened:", err)
		}
	}

	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           trayIcon(),
		IconIsTemplate: runtime.GOOS == "darwin",
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

	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		a.player.Close()
		tray.Destroy()
	})
}

// trayIcon draws the tray glyph: a record, black on transparent so macOS can
// tint it for a light or dark menu bar.
func trayIcon() []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	ink := color.NRGBA{A: 255}
	if runtime.GOOS != "darwin" {
		ink = color.NRGBA{R: 0x1b, G: 0x18, B: 0x14, A: 255}
	}

	cx, cy := float64(size)/2, float64(size)/2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := (dx*dx + dy*dy)
			// A filled disc with a punched-out centre: the shape of a record.
			switch {
			case d <= 13*13 && d >= 4*4:
				img.Set(x, y, ink)
			case d < 1.6*1.6:
				img.Set(x, y, ink)
			}
		}
	}

	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		log.Println("tray icon:", err)
		return nil
	}
	return b.Bytes()
}

func main() {
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
