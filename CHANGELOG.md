# Changelog

All notable changes to this project are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project follows
[Semantic Versioning](https://semver.org/). The project is in beta: versions stay at `0.x` for a long while.

## [Unreleased]

### Added

- The seek bar shows, above the pointer (and along a drag), the time under it and a small still of that moment
  (120×68 at 100%, growing with the screen's scale and the bar's size), as video sites do: a still only near the
  moment shows blurred until its own arrives, an empty box waits for the first, and a film with no stills shows the
  time alone. It stands over the subtitles, takes no clicks, and replaces the track's tip.
- While a film plays in the app's window, stills for its seek bar are taken in the background, one at a time: about
  10 s after it starts (at once when the pointer reaches the bar), one each 10 s of film (further apart past 1000
  stills), from coarse to fine so that the first hundred already cover the whole film, and the moment under the
  pointer before any other. Past those first hundred the background works a quarter of the time and rests the rest.
  Ending the film never waits for a still, and a film with no stills to take, or whose stills keep failing, leaves
  them off.
- Stills of a film taken with the user's own FFmpeg, one short run each, at the lowest priority: the keyframe at or
  just before a moment (about 1 MB read and 150–240 ms here), scaled into a 240×135 box keeping the picture's shape
  (anamorphic DVDs included) as a JPEG. An HD film with no colour tags keeps the BT.709 colours players show,
  interlaced pictures are deinterlaced, and HDR (PQ, HLG) is made small first and tone-mapped after; Dolby Vision
  profile 5 gives no stills. The film is read only as the local file of the container the app opened.

### Fixed

- An FFmpeg the app started (the voice's too) could outlive the app if it crashed: every FFmpeg now joins a job that
  ends with the app.

## [0.25.0] - 2026-10-06

The karaoke mode follows the actors' voice with a speech model inside the app, and a video with no subtitle of its
own is heard to tell which subtitle file fits it. A film can be watched with no subtitle at all, and every wait fills
a bar.

### Added

- A speech model inside the app, computed in Go (Silero VAD v6.2.1, MIT): it tells speech from music and effects. The
  karaoke mode follows the voice with it: measured against 49 lines marked by ear in five films, the fill starts a
  median 29 ms from the voice, where the subtitle's own start is 126 ms off.
- Settings › Study › Actors' voice: "Use the voice AI on this computer" (on) and "Also on the battery" (off, shown
  only on a computer with a battery). The karaoke panel says when the AI rests.
- A video with no subtitle inside is heard as it opens, six stretches of 45 s at a quarter of a core, to tell which
  subtitle file beside it fits: that file goes on top, moved onto the voice when it is 250 ms off or more or runs at
  another speed. A bar shows the hearing, and what was found is kept for the next time the video opens.
- Watch, pressed while the film is still heard, asks first: Wait starts the film on its own with the best subtitle
  once the hearing ends; Watch now starts at once with the choice made.
- A subtitle file kept from an earlier opening, chosen before the video was heard, gives way once to the file in its
  language that fits the voice when the hearing finds it does not; a file chosen afterwards stays.
- Both lines can be off, on the session screen and in the panel: a film can be watched with no subtitle at all.
- Opening a video fills a bar from 0 to 100%.
- Settings › Help › About: the app's version and license, and the licenses of the code from others it carries (Go
  and its modules, the screens' packages, the voice model), as their authors ask. The app ships as a single file, so
  they go inside it; `scripts/update-notices.ps1` writes them, the check refuses them out of date and the build makes
  sure they name every module inside the app. The help page will also hold the guide to using the app.
- `spdev voice-check` measures the voice found against a film's lines, printing only numbers, and `spdev inspect
  --hear` hears a video to tell which file fits it.

### Changed

- The loudness detector is gone: without the AI, switched off or on the battery, the lines follow the estimate.
- Following the voice never makes the video wait: the model judges the sound outside the lock, and decoding ahead
  takes at most a quarter of a core, each window starting a second early so the model hears what leads into it.
- With no line on top, the voice is not followed.
- The log tells when a video could not be heard because no FFmpeg was found.

### Fixed

- Following the voice decoded ahead missed seeks: one made during a window, one back after the film's end, and one
  made while paused.
- A subtitle file named after the video but timed for another release is no longer chosen on its own once the video
  is heard.
- The session screen said a language had no subtitle when its files were found not to fit the video; it now says the
  files found do not fit.

## [0.24.0] - 2026-10-06

Settings can always be saved, and every change shows on the video at once. The two subtitle lines keep the order the
user picks, the translation comes and goes with the line it translates, and the volume keys move faster and show a
level.

### Added

- Changes in Settings show on the video at once, before they are saved. Leaving Settings with unsaved changes asks
  whether to save them, throw them away or keep editing; beside the video, the save bar sits at the top of the panel.
- Settings › Subtitles › Subtitle order: the study line on top and the translation below it, or the other way round.
- The translation comes and goes with the study line it translates, so both show and leave together; a translation
  that matches no study line, such as a sign's, keeps its own time. Settings › Subtitles can turn this off.
- Settings › Playback › Volume step: how far one touch of ↑ or ↓ moves the volume, from 1% to 25%.

### Changed

- The two lines are named by their role, the study line and the translation, instead of by their place.
- ↑ and ↓ move the volume 5% a touch (it was 1%) and about 20% a second when held (it was 9%).
- The volume notice is larger, with a bar of the player's whole range under it, marked at 100%.
- Notices sit in the top right corner, where the eye goes first; profiles still in the top left corner move there.
- When the two lines do not fit, the upper one rises; if it would leave the picture, the lower one shrinks and goes
  down to the picture's edge. The order never changes.

### Fixed

- After leaving fullscreen in a maximized window, the bottom of the app's screen sat behind the taskbar, hiding the
  Save button in Settings and the Play button of a video.
- The panel beside the video ran past its edges at narrow widths: colors with an opacity and the style editor's grids.
- The translation sometimes showed above the study line and sometimes below it.
- After Alt+Tab back to the app, Space and the other video keys did nothing until the video was clicked.

## [0.23.1] - 2026-10-06

The app's icon in a neon red and a pink, its play sign larger and centered.

### Changed

- The app's icon and logo: the play sign is a tenth larger and moved so it looks centered, its straight side sharp at
  every size, in a neon red and a pink that stand out on the taskbar.

## [0.23.0] - 2026-10-06

The karaoke mode can follow the actors' voice, as an experiment, and its estimate now weighs each line by its syllables.
Of several subtitle files, the one that fits the video best comes first, judged by what the files hold, and only real
videos and subtitles are opened.

### Added

- Of several subtitle files in one language, the one that fits the video best comes first: a subtitle inside the video,
  then the files lined up with it by the share of their lines that meet it, a file timed for another frame rate a
  little behind. The lists show each line's best choice first, and any track can still be picked by hand.
- A subtitle renamed without an extension, or to .txt, is found by its text.
- The karaoke mode can follow the actors' voice: the fill moves while they speak and holds in their pauses. Following
  the voice comes on with the karaoke mode and can be turned off; it is still an experiment, since music and sound
  effects can pass for a voice and paint a line worse than the estimate. With an FFmpeg on the computer (found, chosen,
  or installed with winget from its authors), the voice is analyzed a little ahead of the video; without one, the app
  hears the player's own sound as it plays. A line trained again is painted from the voice heard the first time. When
  the voice cannot be told (music, a dubbed track playing, a busy computer), the line is painted by the estimate, as
  before.
- Settings › Study › Actors' voice: follow the voice or not, the sensitivity, the FFmpeg, the output's delay for
  wireless headphones, and following on even when the computer is busy. The panel tells how the voice is followed.
- The estimate weighs a line by its syllables: a sound's description ("[DOOR SLAMS]") and a speaker's name take no time,
  a stretched sound ("duuuu") takes longer, and the fill holds at commas and periods.
- The app's own icon and logo, also beside the app's name.
- A third-party notices file in the zip, with the licenses of everything inside the app.

### Changed

- A subtitle file's language, and whether it is for the hearing impaired or forced, come from its text rather than its
  name, which only helps when the text cannot tell.
- The frame around a line picked for a correction is a thin, sharp blue line with a dark edge outside it.
- VLC sends its sound through the Windows mixer, never straight to a receiver, so the voice can be heard.
- The app no longer carries Microsoft's WebView2 installer: a computer without WebView2 is offered the download from
  Microsoft. The app is 1.7 MB smaller.

### Fixed

- The karaoke mode's fill was invisible over a line of a close color (the yellow of the high contrast look): such a
  fill now gives way to a contrasting one, and Settings says which.
- Closing the app (Alt+F4) with a video playing left the window on screen for several seconds: it now goes at once,
  while the player closes behind it.
- The fill started up to a second late at the start of a line.
- The frame around a line picked for a correction never went away: a click beside the lines or playing lets it go.
- A subtitle file whose title began with a word that is also a language code (such as "Ice", Icelandic's) was taken
  for that language.

### Security

- Only real videos and subtitles are opened: a script, a page or a program renamed as a video or a subtitle is refused,
  judged by the start of the file, which is never read whole to find out. FFmpeg only ever reads the film as a local
  file.

## [0.22.0] - 2026-10-05

### Added

- Settings › General › Window: keep the app always on top, on by default. The app's window, with the subtitles and
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

- Each subtitle's line spacing, and the least distance between the two subtitles when they meet.
- Notices on the video: the font, size, colors, side at the top, margin and width of the short notices, and how long
  they stay.
- The selected line: the colors, width, gap and corners of the frame around the line being corrected.
- Karaoke mode: the thickness and color of the bar before a line and the size of the countdown; the preview can show a
  line before its time.
- Settings › Appearance:
  - the control bar and its menus: colors, font, size (75% to 200%) and how long the bar stays, with a picture of it;
  - the correction box: its colors and the hint's size, with a sample;
  - the app's own accent color, font and text size (85% to 150%).
- Restoring the defaults on each of these groups.

### Changed

- The defaults are the looks of 0.20.1, so nothing changes until a choice is made. `profiles.json` moves to version 5:
  every profile keeps its look, and the karaoke bar keeps the color it had (the profile's preview color).

## [0.20.1] - 2026-10-05

A review of 0.20.0: the menu reaches the app, and smaller fixes.

### Fixed

- The menu over the video lacked the app's items in the app (ending the video, opening another one, the settings,
  copying a line, its look, adding and exporting subtitles) and greyed out the track lists.
- A sync step made right as the line's subtitle switched could still be kept on the other file.
- Adding a subtitle file while the video played read the video's saved passages without the lock that guards them.
- The subtitles card offers none on top when the video plays without a subtitle there, instead of naming a track that
  is not on screen.
- The menu's "CC" icon fits its picture; its frame was cut on both sides.
- A pause or a lap of a passage being studied that the player does not carry out is told on the video.
- `scripts\make-fixtures.ps1` runs its last step again.

### Changed

- The menu's item for adding a subtitle file is named as on the session screen and the panel.
- The developer tool `spdev` speaks English: its options are now `--subtitles`, `--start`, `--paused` and `--step`.
- The check runs staticcheck.

## [0.20.0] - 2026-10-05

A right click on the video opens a full menu, and subtitles switch while the video plays.

### Added

- The right button on the video or its bar, the menu key or Shift+F10 opens Windows' own menu over the video: the line
  under the click (correct it in place, copy it, its look), playback, speed, audio, subtitles and sync, study mode,
  karaoke mode, passages, another video, the settings and closing, with icons, submenus and the key the profile gives
  each command.
- Switching the subtitle on top or below, or the audio, while the video plays, from the menu or from the panel's new
  subtitles card; choosing none takes the translation away.
- Adding a subtitle file on the session screen, the panel and the menu: a subtitle file from any folder joins the video
  and comes back the next time it opens.

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
- The interface's language: the Windows display language when the app speaks it, and a field in Settings › General as
  soon as there is more than one.
- Telling a subtitle's language from its text also knows German and Italian; each language's words are a data file.

### Fixed

- One lap and a goal of one repetition now read in the singular.

## [0.18.0] - 2026-10-05

One window: with VLC, the video plays inside the app's window, with the panel beside it.

### Added

- The video plays under the header of the app's window; the panel beside it (▤) holds the playback controls, the
  corrections, the karaoke mode, the passages and the settings, its edge dragged wider and its state remembered.
- Fullscreen (F, double click, the bar's button) leaves only the video; Esc comes back.
- Closing the window, or Alt+F4 on the video, closes everything, the video's work saved first; ending the video goes
  back to the session screen. The window's title names the video.
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

The karaoke mode's fill color on the page of the line on top, and three fixes.

### Added

- Settings › Subtitles › Top edits the karaoke mode's fill color, and its preview can show the line half spoken.

### Fixed

- The spoken part and the bar of a preview follow the line's opacity: at 50% they no longer stay opaque, at 0% they no
  longer show.
- Jobs of a window's thread run while a modal loop is open (a window being dragged or sized, a menu), instead of waiting
  for the next one.
- Saving the settings no longer undoes the karaoke mode of the video being watched, set by its project or the bar.

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

The passage's save button and training a line again.

### Fixed

- A field's button keeps its whole label on one line
- Training a line again plays it again, also at a passage's end

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
- Saved passages and resuming where the video was left, on the session screen
- The bar over the video drives the passage, the karaoke mode and the subtitles

### Fixed

- An edit, undo or redo could leave the old text on screen

### Documentation

- The karaoke mode, saved studies and the bar in the README

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
- Optional "Open with" and context menu entry for videos

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

Karaoke mode and studying a passage.

### Added

- Karaoke mode and studying a passage

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

[Unreleased]: https://github.com/ediejohnny/lingoreel/compare/v0.25.0...HEAD
[0.25.0]: https://github.com/ediejohnny/lingoreel/compare/v0.24.0...v0.25.0
[0.24.0]: https://github.com/ediejohnny/lingoreel/compare/v0.23.1...v0.24.0
[0.23.1]: https://github.com/ediejohnny/lingoreel/compare/v0.23.0...v0.23.1
[0.23.0]: https://github.com/ediejohnny/lingoreel/compare/v0.22.0...v0.23.0
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
