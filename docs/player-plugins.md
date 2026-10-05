# Player plugins

LingoReel does not play the video by itself: it opens the video in an **external player** and draws the
subtitles over it. Each player is a **plugin**, a Go package that follows the same contract.

There are two to learn from: **VLC** (`internal/infrastructure/player/vlc`), which draws inside a window of the app and
is driven through a local channel, and **PotPlayer** (`internal/infrastructure/player/potplayer`), which draws in its own
window and is driven by Windows messages. Follow whichever is closer to your player.

Plugins are compiled into the app, because Go's `plugin` package does not work on Windows. So writing a plugin means
writing a package and adding one line to the list of plugins.

## The contract

The contract lives in `internal/application/players`:

```go
type Plugin interface {
    Descriptor() player.Descriptor
    Launch(ctx context.Context, executable string, request player.LaunchRequest) (Running, error)
}
```

### `Descriptor`

What the user sees about the player:

- **`ID`:** short, lowercase, no spaces (`vlc`, `mpv`, `mpc-hc`). It is stored in the settings.
- **`Name`:** the name shown on screen.
- **`Install.WingetID`:** the winget package used by the automatic installation. Empty when there is no package.
- **`Install.DownloadURL`:** the official download page. It must be `https`.
- **`Install.Benefit`:** one sentence on why to choose this player, in the interface's language (Brazilian Portuguese for
  now). It shows up behind the (?) button.
- **`DrawsInHost`:** `true` when the player can draw the video inside a window of the app (`LaunchRequest.Host`). This is
  the preferred way:
  - the window belongs to LingoReel, with closing (X, Alt+F4), fullscreen, keyboard and the control bar that shows
    up when the mouse is near the bottom;
  - the player must not handle mouse or keyboard inside it;
  - with a zero `Host`, it opens its own window, and the overlay follows that window.

  Examples: VLC uses `--drawable-hwnd` and mpv uses `--wid`.
- **`FixedAudioTrack`:** `true` when the player cannot be told which audio track to play. The session screen then
  disables that choice and explains it stays in the player's own menu. PotPlayer is like this.

### `Launch` and `Running`

`Launch` opens the video and only returns once it is open. The returned `Running` drives the player:

- **Commands:** play, pause, seek, change the speed, step a frame and pick the audio track.
  - `Capabilities()` says which ones the player has.
  - Volume is optional: a player that declares `Volume` implements `SetVolume` (`session.VolumePlayer`), from 0 to 1,
    where 1 is the player's 100%.
  - A command the player does not have **returns an error**. Never pretend it worked: the app shows the button
    disabled.
- **`Samples()`:** delivers observations (position, duration, speed, state). It is closed when the player ends.
- **`ProcessID()`:** the player's process. The overlay follows its window.
- **`Err()`:** once `Samples()` is closed, tells whether the player **crashed** (`playback.ErrPlayerLost`) or the user
  **closed** it (`nil`).
- **`Close()`:** ends the player within a time limit. Calling it again does nothing, and no process may be left behind.
- **`Reconnecting()`** (optional, `players.Reconnector`): `true` while the player is alive but not answering and the
  plugin is trying again. The panel shows a "reconnecting" notice instead of ending the session.
- **`Window()` and `VideoArea()`** (optional, `players.Windowed`): for a player that draws in its own window, they say
  **which** window is its own and **which part** of it holds the picture. Without them, the overlay looks for the
  process's largest window and covers all of it. PotPlayer needs both: its window has a playlist and a control bar next
  to the picture. Either may be zero at any moment (window not open yet, or already closed).

### Where the player is installed

To be found, the plugin also implements `Hints() discovery.Hints` (`internal/infrastructure/player/discovery`), with:

- the executable's names;
- the registry keys;
- the paths under Program Files and `%LOCALAPPDATA%`;
- the name in the list of installed programs.

### Two ways of driving a player

The plugin talks to the player the way the player accepts:

- **Local channel** (VLC, mpv): an HTTP interface on `127.0.0.1` or a named pipe, with a port and a password per session.
- **Window messages** (PotPlayer): `SendMessageTimeout(window, WM_USER, code, value)` with `SMTO_ABORTIFHUNG`, which is
  what tells a hung player from one that answered. This is the way of `internal/infrastructure/player/potplayer`, and
  `win32.SendMessageTimeout` is the helper.

Either way, the plugin **asks again with growing waits** before giving the session up, and reports it through
`Reconnecting()`.

## Rules

- **Process:** the player runs in a Job Object so it is never orphaned (see `internal/infrastructure/windows/job`).
- **Player subtitles off:** LingoReel draws the subtitles.
- **Local control:**
  - only `127.0.0.1`, a named pipe or window messages, never the network;
  - random port and password, per session;
  - arguments passed separately, never a command line built from text;
  - the video's path is absolute, so the player never reads it as an option.
- **`context`:** every piece of work that waits honors the `context` it receives.
- **No global state:** the plugin is a plain value, such as `vlc.Plugin{Logger: ...}`.

## Step by step

1. Create the package `internal/infrastructure/player/<your-player>` with a `Plugin` type.
2. Implement `Descriptor`, `Launch` and `Hints`.
3. Register the plugin with one line in the `All` list of `internal/infrastructure/player/plugins`. The app and the
   development tool use that same list.
4. Write the tests under `tests/infrastructure/player/<your-player>/` and run them through the **conformance kit**:

   ```go
   func TestMyPlayerPassesTheKit(t *testing.T) {
       plugintest.Descriptor(t, myplayer.Plugin{})
       plugintest.Launch(t, myplayer.Plugin{}, executable, video)
   }
   ```

   The kit (`tests/infrastructure/player/plugintest`) checks:
   - that the descriptor is valid;
   - that a cancelled `Launch` gives up quickly and leaves no process;
   - that every declared capability works and the undeclared ones return errors;
   - that `Close` ends everything within a time limit, also when called twice.
5. Run `scripts\check.ps1`. It must pass without errors.
