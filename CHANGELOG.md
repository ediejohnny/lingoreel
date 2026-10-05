# Changelog

All notable changes to this project are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project follows
[Semantic Versioning](https://semver.org/). The project is in beta: versions stay at `0.x` for a long while.

## [Unreleased]

## [0.22.0] - 2026-10-05

### Added

- Configurações › Geral › Janela: "Manter o app sempre no topo", on by default. The app's window, with the subtitles and
  the bar over the video, stays in front of other programs; turned off, it can go behind them.
- The menu over the video shows stop in red, play in blue and pause in amber.

### Fixed

- A white band over the video after maximizing the window or going fullscreen (at the top or the bottom, full width or
  part of it): the app's window painted its white background over the video's black bars.
- The menu over the video's icons were black on the dark menu. The menu now follows the app's theme (dark, light or
  Windows'), and its icons are drawn for it.

## [0.21.0] - 2026-10-05

What shows on the screen can be made one's own: the subtitles' spacing, the marks over the video, the control bar and
its menus, the correction box and the app itself.

### Added

- Each subtitle's line spacing ("Entrelinha"), and the least distance between the two subtitles when they meet.
- "Avisos no vídeo": the font, size, colors, side at the top, margin and width of the short notices, and how long they
  stay.
- "Fala selecionada": the colors, width, gap and corners of the frame around the line being corrected.
- Modo Karaokê: the thickness and color of the bar before a line and the size of the countdown; the preview can show a
  line before its time.
- Settings › Aparência:
  - the control bar and its menus: colors, font, size (75% to 200%) and how long the bar stays, with a picture of it;
  - the correction box: its colors and the hint's size, with a sample;
  - the app's own accent color, font and text size (85% to 150%).
- "Restaurar o padrão" on each of these groups.

### Changed

- The defaults are the looks of 0.20.1, so nothing changes until a choice is made. `profiles.json` moves to version 5:
  every profile keeps its look, and the karaoke bar keeps the color it had (the profile's preview color).

## [0.20.1] - 2026-10-05

A review of 0.20.0: the menu reaches the app, and smaller fixes.

### Fixed

- The menu over the video lacked the app's items in the app ("Encerrar o vídeo", "Abrir outro vídeo…",
  "Configurações…", copying a line, its look, adding and exporting subtitles) and greyed out the track lists.
- A sync step made right as the line's subtitle switched could still be kept on the other file.
- Adding a subtitle file while the video played read the video's saved passages without the lock that guards them.
- The "Legendas" card offers "Nenhuma" on top when the video plays without a subtitle there, instead of naming a track
  that is not on screen.
- The menu's "CC" icon fits its picture; its frame was cut on both sides.
- A pause or a lap of "Estudar trecho" that the player does not carry out is told on the video.
- `scripts\make-fixtures.ps1` runs its last step again.

### Changed

- The menu's item is named "Adicionar arquivo de legenda…", as on the session screen and the panel.
- The developer tool `spdev` speaks English: its options are now `--subtitles`, `--start`, `--paused` and `--step`.
- The check runs staticcheck.

## [0.20.0] - 2026-10-05

A right click on the video opens a full menu, and subtitles switch while the video plays.

### Added

- The right button on the video or its bar, the menu key or Shift+F10 opens Windows' own menu over the video: the line
  under the click (correct it in place, copy it, its look), playback, speed, audio, subtitles and sync, study mode, Modo
  Karaokê, passages, another video, the settings and closing, with icons, submenus and the key the profile gives each
  command.
- Switching the subtitle on top or below, or the audio, while the video plays, from the menu or from the panel's new
  "Legendas" card; "Nenhuma" takes the translation away.
- "Adicionar arquivo de legenda…" on the session screen, the panel and the menu: a subtitle file from any folder joins
  the video and comes back the next time it opens.

### Changed

- A held volume key goes all the way to the player's ceiling (200% on VLC), no longer stopping at 100%.
- Interface texts no longer mix English into Portuguese: the export dialog's formats keep their own names, and the
  diagnostics and track notes read in Portuguese.

### Fixed

- A sync step or a correction made right as a line's subtitle is switched is kept on the track it was made on.

## [0.19.0] - 2026-10-05

The app is ready for translations.

### Added

- Every text of the interface comes from a catalog per language, read by the Go side and the screens alike; Brazilian
  Portuguese is the complete one. Adding a language is adding a folder, with no code changes: see
  [Translating](docs/translating.md).
- The interface's language: the Windows display language when the app speaks it, and a field in Configurações › Geral as
  soon as there is more than one.
- Telling a subtitle's language from its text also knows German and Italian; each language's words are a data file.

### Fixed

- "1 voltas" and "Meta de 1 repetições concluída" now read in the singular.

## [0.18.0] - 2026-10-05

One window: with VLC, the video plays inside the app's window, with the panel beside it.

### Added

- The video plays under the header of the app's window; the panel beside it (▤) holds the playback controls, the
  corrections, the Modo Karaokê, the passages and the settings, its edge dragged wider and its state remembered.
- Fullscreen (F, double click, the bar's button) leaves only the video; Esc comes back.
- Closing the window, or Alt+F4 on the video, closes everything, the video's work saved first; "Encerrar" goes back to
  the session screen. The window's title names the video.
- With PotPlayer, which keeps its own window, closing PotPlayer closes the app too.

### Changed

- Corners of boxes, buttons and the video bar's menus are nearly square (at most 0.1rem).

### Fixed

- A right click on the video no longer sends the app's window a message that waits for it.

## [0.17.0] - 2026-10-05

The volume goes up to 200% on VLC, gradually while a key is held.

### Added

- Volume up to 200% on VLC (100% on PotPlayer, the most it takes), shown on the video at each change; the control bar's
  volume line spans the player's ceiling, past 100% in amber.
- The volume keys move 1% a touch; held, about 9% a second, and going up they stop at 100% until a new touch.

### Fixed

- PotPlayer opened at its own kept level while the bar showed 100%.
- VLC no longer changes the level of the user's own VLC: it keeps no volume between sessions.
## [0.16.3] - 2026-10-05

The Modo Karaokê's fill color on the page of the line on top, and three fixes.

### Added

- Settings › Legendas › De cima edits the Modo Karaokê's fill color, and its preview can show the line half spoken.

### Fixed

- The spoken part and the bar of a preview follow the line's opacity: at 50% they no longer stay opaque, at 0% they no
  longer show.
- Jobs of a window's thread run while a modal loop is open (a window being dragged or sized, a menu), instead of waiting
  for the next one.
- Saving the settings no longer undoes the Modo Karaokê of the video being watched, set by its project or the bar.

### Changed

- The Go module is named after its GitHub home: `github.com/ediejohnny/lingoreel`.
- The repository speaks English: the README, the plugin guide and the scripts' messages. The app's interface stays in
  Brazilian Portuguese for now.

### Documentation

- This changelog.

## [0.16.2] - 2026-10-04

Each subtitle line has its settings page, with the preview beside the fields.

### Fixed

- The settings give each subtitle line its page, with the preview beside

## [0.16.1] - 2026-10-03

The passage's save button and "Treinar de novo".

### Fixed

- A field's button keeps its whole label on one line
- "Treinar de novo" plays the line again, also at a passage's end

### Documentation

- The README calls the app a beta

## [0.16.0] - 2026-10-03

Correct a line right over the video.

### Added

- Two clicks on a paused line correct it right over the video

### Documentation

- Correcting a line right over the video, in the README

## [0.15.1] - 2026-10-03

Subtitles and bar stay over the video in fullscreen.

### Fixed

- Fullscreen no longer hides the subtitles and the bar for good

## [0.15.0] - 2026-10-03

Saved studies and the full control bar.

### Added

- The video's project keeps how it was left and each passage's plan
- Ctrl+F9 and Ctrl+F10 mark a passage, save it and study it
- A video comes back as it was left; passages managed before watching
- Saved passages and "Continuar de" on the session screen
- The bar over the video drives the passage, the Modo Karaokê and the subtitles

### Fixed

- An edit, undo or redo could leave the old text on screen

### Documentation

- The Modo Karaokê, saved studies and the bar in the README

## [0.14.0] - 2026-10-03

PotPlayer plugin.

### Added

- The overlay can follow a plugin's window and its video area
- A plugin can tell the overlay which window holds the picture
- PotPlayer as a player plugin
- The subtitles use the app's own font by default

## [0.13.0] - 2026-10-03

Robustness and packaging.

### Added

- Daily logs with the session's events
- One app at a time, a second launch opens its video here
- Wait for a silent player, and hold the subtitles of a frozen one
- Say what went wrong and what to do
- Diagnostics in the advanced settings
- Optional "Abrir com" and context menu entry for videos

### Changed

- Version in the app's properties, WebView2 inside, portable zip

## [0.12.0] - 2026-10-03

Subtitle corrections, the video project file and export.

### Added

- Subtitle corrections and the video's project file (back end)
- Subtitle editor on the panel, saved passages and export

## [0.11.0] - 2026-10-03

Study modes and the remaining shortcuts.

### Added

- Study modes and the remaining shortcuts

## [0.10.0] - 2026-10-01

Settings, profiles, preview and configurable shortcuts.

### Added

- Settings screen with profiles, live preview and configurable shortcuts

## [0.9.0] - 2026-10-01

Modo Karaokê and Estudar trecho.

### Added

- Modo Karaokê and "Estudar trecho"

## [0.8.0] - 2026-10-01

The app's own player window, with a control bar.

### Added

- The player window is the app's own, with controls over the video

### Fixed

- Keep GDI+ call arguments on the heap

## [0.7.0] - 2026-10-01

The app window, player plugins and assisted installation.

### Added

- Treat every video player as a plugin
- Settings files, winget installer and the shipped plugin list
- The app's API for its window
- The LingoReel window

### Changed

- Move the watching session into the application layer
- Share the Portuguese wording and the composition pieces with spdev

### Fixed

- Let any supported player plugin be chosen
- Keep the embedded frontend folder in fresh clones

## [0.6.0] - 2026-10-01

Downloaded subtitles, automatic and manual sync.

### Added

- Align subtitle files with the video's own subtitles
- Find candidate subtitle files in any subfolder
- Choose and correct subtitle files by their timing
- Show short notices in a corner of the video
- Move each subtitle line earlier or later by steps
- Register shortcuts only while the player is in front
- Sync shortcuts in spdev play and overlay

### Documentation

- Explain that the VLC endpoint file is a Lua page, not JSON

## [0.5.1] - 2026-10-01

Fixes from the audit against the docs, risk mitigations.

### Added

- Emulate the backward frame step with an exact seek
- Open other videos with subtitle files and analyze with a context
- Log VLC events that have no caller
- Add spdev diag with monitors, scaling and a checklist

### Changed

- Move the overlay to infrastructure behind a session display port
- Soften the default translation and name the renderer's shadow numbers
- Keep operating system path rules out of the domain

### Fixed

- Read each indexed subtitle block once and reject indexes that skip clusters
- Let track names beat the default language and prefer full tracks over forced ones
- Harden the VLC control channel and report a lost player
- Redraw at once when an offset changes and cap the lines shown at once
- Keep the primary line fixed and bound the space subtitles take
- Follow the monitor DPI and release the overlay after a hung player
- Treat the lowercase ASS line break as soft unless WrapStyle is 2
- Validate stored documents on load and bound subtitle file reads
- Keep the user's first seek after opening at a start position

## [0.5.0] - 2026-09-29

External subtitle files and a window that fits the screen.

### Added

- Read subtitle files next to the video and in Subs folders

### Fixed

- Open the player window inside the screen

### Documentation

- License the project under MIT and invite contributions

## [0.4.0] - 2026-09-29

Native subtitle overlay over the player.

### Added

- Add the subtitle layout for both lines over the video
- Add the UI thread and the GDI+ subtitle renderer
- Draw the subtitles in an overlay over the player window
- Read subtitle tracks through the cue index

### Fixed

- Smooth small backward corrections of estimated positions
- Keep the overlay hidden over an empty player and never hang on a hung one
- Seek to the start when VLC pauses before applying it
- Never pair two subtitle lines in the same language
- Report VLC positions past 35 minutes

## [0.3.0] - 2026-09-29

Playback session, player detection and the VLC adapter.

### Added

- Add playback clock, controls and boundary-driven subtitle sync
- Detect installed video players from registry, folders and PATH
- Add kill-on-close job object for player processes
- Add VLC adapter over a private HTTP control endpoint
- Add spdev play to follow subtitles with a real player

### Fixed

- Show subtitles only once the player reports its real position

## [0.2.0] - 2026-09-29

Matroska reader, language detection and track selection.

### Added

- Add media tracks, language normalization and track selection
- Add Matroska reader for tracks and text subtitle frames
- Add analysis use cases for media and subtitle loading
- Add spdev inspect command

## [0.1.0] - 2026-09-29

Subtitle core, text subtitle parsers and settings model.

### Added

- Add spdev developer tool skeleton with build version
- Add subtitle timeline with interval lookup and offsets
- Add SRT, ASS/SSA and WebVTT parsers with encoding fallback
- Add appearance, study and settings domain models
- Add versioned JSON store with migrations and atomic writes

### Changed

- Add PowerShell check and build scripts
- Move tests into the tests tree and guard the layout

[Unreleased]: https://github.com/ediejohnny/lingoreel/compare/v0.22.0...HEAD
[0.22.0]: https://github.com/ediejohnny/lingoreel/compare/v0.21.0...v0.22.0
[0.21.0]: https://github.com/ediejohnny/lingoreel/compare/v0.20.1...v0.21.0
[0.20.1]: https://github.com/ediejohnny/lingoreel/compare/v0.20.0...v0.20.1
[0.20.0]: https://github.com/ediejohnny/lingoreel/compare/v0.19.0...v0.20.0
[0.19.0]: https://github.com/ediejohnny/lingoreel/compare/v0.18.0...v0.19.0
[0.18.0]: https://github.com/ediejohnny/lingoreel/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/ediejohnny/lingoreel/compare/v0.16.3...v0.17.0
[0.16.3]: https://github.com/ediejohnny/lingoreel/compare/v0.16.2...v0.16.3
[0.16.2]: https://github.com/ediejohnny/lingoreel/compare/v0.16.1...v0.16.2
[0.16.1]: https://github.com/ediejohnny/lingoreel/compare/v0.16.0...v0.16.1
[0.16.0]: https://github.com/ediejohnny/lingoreel/compare/v0.15.1...v0.16.0
[0.15.1]: https://github.com/ediejohnny/lingoreel/compare/v0.15.0...v0.15.1
[0.15.0]: https://github.com/ediejohnny/lingoreel/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/ediejohnny/lingoreel/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/ediejohnny/lingoreel/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/ediejohnny/lingoreel/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/ediejohnny/lingoreel/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/ediejohnny/lingoreel/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/ediejohnny/lingoreel/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/ediejohnny/lingoreel/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/ediejohnny/lingoreel/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/ediejohnny/lingoreel/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/ediejohnny/lingoreel/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/ediejohnny/lingoreel/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/ediejohnny/lingoreel/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/ediejohnny/lingoreel/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/ediejohnny/lingoreel/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/ediejohnny/lingoreel/releases/tag/v0.1.0
