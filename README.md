<img src="build/appicon.png" alt="" width="96" align="right">

# LingoReel

A Windows 10/11 app that helps you study languages with movies and series. It opens the video in your favorite
player and draws two synchronized subtitles over it: one in the language you are learning and its translation, which
can stay on screen or show up only when you ask for it.

> **Status:** beta (`0.x` versions). Everything described below already works, and the app keeps getting better.
> The interface is in Brazilian Portuguese for now (the first audience is Brazilian students learning English);
> the app is being prepared to take translations into other languages.

## Features

- Reads the subtitles inside `.mkv` files without ever changing the original file.
- Reads external subtitles (`.srt`, `.ass`, `.ssa`, `.vtt`) from the video's folder and its `Subs`/`Subtitles`
  subfolders, for when the movie does not ship the subtitle you want.
- Two subtitles at once, each with its own synchronization and offset.
- Three study modes: both subtitles, translation on demand (show, hide or hold a key to reveal) and translation always
  visible.
- Configurable shortcuts: 5, 10 and 30 second seeks, a precision step, frame by frame, speed, repeat the line and the
  last few seconds.
- Karaoke mode ("Modo Karaokê"): in long pauses the next line shows up ahead of time, with a bar or a countdown, and
  fills in while the actor speaks, so you can speak along.
- Saved studies: Ctrl+F9 marks where a passage begins and Ctrl+F10 where it ends. The passage is saved with its times as
  its name and repeats until you leave it. When you open the video again, it resumes where you left off, with the speed,
  the karaoke mode, the sync and the list of passages.
- Its own control bar over the video, with frame stepping and menus for the passage, the karaoke mode and the
  subtitles, without going back to the app.
- Works with VLC and PotPlayer, and you choose your favorite player. Other players come in as plugins (see
  [Player plugins](docs/player-plugins.md)).
- Windowed, fullscreen, several monitors and display scaling from 100% to 200%.
- Subtitle corrections while the video is paused: double-click a line to open a box right over the video and fix the
  text; the timing is adjusted in the app's panel. Undo and redo (Ctrl+Z/Ctrl+Y), and the original text is never lost.
- One corrections file per video (`Movie.dualsub.json`, next to the video) that keeps the corrections, the sync, the
  chosen tracks and mode, and the passages saved for study.
- Exports the corrected, synchronized subtitle as `.srt` or `.ass`, to use in another player.
- A single portable `.exe`, no installation. It uses Windows' WebView2, which Windows 11 and up-to-date Windows 10
  already have; a machine without it is offered the download from Microsoft.
- Open videos from the Windows menu ("Assistir com o LingoReel" and "Open with"), if you turn the option on; the
  default program of each video does not change.
- Daily logs for the last 14 days and a diagnostics screen to report problems, without the subtitles' text and without
  sending anything over the internet.

## Development

Requirements: Windows 10/11 64-bit, Go 1.26+ and PowerShell 5.1+. For `go test -race`, a `gcc` on the `PATH`. The
check also runs [staticcheck](https://staticcheck.dev/): `go install honnef.co/go/tools/cmd/staticcheck@v0.8.1`.

| Command | What it does |
|---|---|
| `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\check.ps1` | Formatting, static analysis and tests |
| `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build.ps1` | Builds the executables into `build\bin` |
| `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\package.ps1` | Builds the portable zip into `build\dist` (after the build) |
| `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dev.ps1` | Opens the app in development mode (reloads when the code changes) |

Versions follow [Semantic Versioning](https://semver.org/); see the [changelog](CHANGELOG.md).

## Contributing

Contributions are welcome! Open an issue to talk about an idea or send a pull request. Before sending it, run
`scripts\check.ps1`: it must pass without errors. The interface follows [Prettier](https://prettier.io/): run
`npm run format` in the `frontend` folder to format the code.

Want to use another player? Every player is a plugin that follows the same contract: see
[Player plugins](docs/player-plugins.md).

Want the app in your language? Every text comes from a catalog per language: see [Translating](docs/translating.md).

## License and credits

LingoReel is distributed under the [MIT license](LICENSE): you may use, copy, modify and distribute the project,
in whole or in part, including in commercial projects, as long as you keep the copyright notice and the license text.

The app includes third-party code under permissive licenses (MIT, BSD, ISC and Apache-2.0): Go, Wails, React and a few
small libraries. Their notices go in the zip, in `AVISOS-DE-TERCEIROS.txt`, written by `scripts\package.ps1` from what
the executable actually contains.

If this project is useful to you, **please give credit** by mentioning LingoReel and its author, Edie Johnny,
with a link to the repository. The license does not require it, but it is a kind gesture that helps the project go on.
