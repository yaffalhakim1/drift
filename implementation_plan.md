# Plan: folder-of-songs mode for drift

## Goal
Pick a folder of MP3s once. drift plays them in order, auto-advances,
next/prev move between songs. The existing single-recording mode with 28
chapters stays.

## Current shape
- `player.go`: one file + one stream; `mix.Chapters` maps position → chapter.
- `main.go`: `choose()` opens one file; `last-recording` remembers it.
- `view.go`: list renders `mix.Chapters`; title from chapter data.

## Changes

### 1. `internal/player/player.go`
- New type `Track{ Title, Artist string }` and `Tracks() []Track`.
- `parseQueueName(filename) (title, artist string)` — strips extension,
  leading track numbers, splits on " - ", drops "(Official ... Video)"
  parentheticals. Pure, unit-tested.
- `FolderTracks(dir string) ([]string, error)` — scans a folder for
  supported extensions, sorts by name. Reuses `supported()`. Unit-tested
  with a temp dir.
- `OpenQueue(paths []string) error` — stores the queue, opens track 0,
  plays. Refactor: extract `openStream(path)` (file + stream setup, no
  persistence); `Open()` = openStream + remember; `openIndex(i)` =
  openStream, no remember (a song change must not corrupt last-recording).
- Auto-advance: `OnEnd` handler checks queue mode and calls `openIndex(i+1)`
  + `Play()` on the listener goroutine (MyGo fires OnEnd on its own
  goroutine; every path locks `p.mu`).
- `Next`/`Prev`/`JumpTo`: queue mode → song index; else existing chapter
  logic. State gains `Title, Artist string; Queue bool; QueueLen int`.

### 2. `main.go`
- `chooseFolder()`: `Dialog.Open{Directory: true}`, `player.FolderTracks`,
  `OpenQueue`, remember folder in `last-folder`.
- Startup: prefer a valid `last-folder`, else the old `last-recording`.
  Choosing a single mix clears `last-folder`; choosing a folder clears
  `last-recording` — one source wins.
- Tray menu: "Choose folder…" item. Model gains `ChooseFolder func()`.

### 3. `internal/view/view.go`
- `plateSection`/`trackMeta`: queue mode uses `st.Title`/`st.Artist`.
- `chapterList`: queue mode renders `player.Tracks()` rows via
  `ui.List` (virtual: builds only visible rows, so 207 rows cost the same
  as 20; checked against docs/ui/list.md). ListState lives in app state
  per docs/ui/views.md; click → JumpTo via the list's Selected + Changed.
- Seek bar unchanged: position/total already scope to the current stream.

### 4. Tests
- `parseQueueName` table test (real sheeran-mix filenames).
- `FolderTracks` on a temp dir: filters junk, sorts.
- `nextIndex` transition table.
- View test gated on `DRIFT_FOLDER` (real folder), like `DRIFT_MIX`.

## Not doing (YAGNI)
Shuffle, reorder, named playlists, volume slider UI, M3U export.

## Risks
1. 207 rows on screen: virtual list keeps the build cheap; verify with
   MYGO_FRAME_STATS=1 anyway.
2. OnEnd goroutine races: all queue mutations under `p.mu`; callback
   released before invoking.
