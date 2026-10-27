# Changelog

All notable changes to this project are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project follows
[Semantic Versioning](https://semver.org/). The project is in beta: versions stay at `0.x` for a long while.

## [Unreleased]

### Changed

- The README says plainly that its screenshots' frames of Sintel stay under Creative Commons Attribution 3.0, with
  the app's subtitles and menus drawn over them, and are not covered by the MIT license.

### Fixed

- On a computer whose processors were all busy, the app said FFmpeg was missing while it was there: the check that it
  runs started it at the idle priority, where it waited up to a minute for a turn. The check now runs at the normal
  priority (it answers in a third of a second on two busy processors), and an FFmpeg that still does not answer in
  time is told apart from a missing one: Settings say the computer is busy and offer no installation, and an FFmpeg
  chosen by hand is not called "not a working FFmpeg". Its work, the voice and the seek bar's stills, keeps the idle
  priority, so it never takes the film's turn.
- A computer busy for a moment no longer turns the seek bar's stills off until the film is opened again. The look at
  the film every still waits for now runs at the normal priority (on two busy processors it waited past its time at the
  idle one), after the first seconds of play like the stills themselves, so the player's opening is left alone. What
  the computer, not the film, kept from being done in time (a still that got almost no processor, or a wait for its turn
  behind the voice) is asked again after a rest of 30 seconds, or at once when the pointer comes to the bar. Such a
  still never counts toward giving up on the film; one that worked its whole time still does, so a film too heavy for
  its stills is not tried for ever.

## [0.27.2] - 2026-10-27

### Added

- `lrdev seek-probe <clip>` measures, lap after lap, how long a passage's start takes in VLC from the order to its
  first picture on the screen and to its first sound, and whether the two still go together a second later. A lap is
  sent as one order, as a seek and a play, or with its picture made ready during the gap and a play after, also held
  to fewer processors.
- `lrdev stills --probesize <bytes>` measures the stills with a smaller FFmpeg probe; the app keeps FFmpeg's own.

### Changed

- The developer tool is called `lrdev` (`lrdev.exe` beside the app in `build\bin`).
- The zip's read-me and third-party notices are in English: `README.txt` and `THIRD-PARTY-NOTICES.txt`.

### Fixed

- A video dragged onto the window opens in the app, on the same screen as "Open video", with the subtitles and the
  study mode. It used to play in a browser window of its own, without any of them, that stayed open after the app
  closed. Nothing dropped on the window, a file or a link, is opened by the window's browser anymore.
- Closing the app while the setup assistant is still open no longer writes over the answers it was given a moment
  before.
- The karaoke mode's fill no longer drops halfway when the next line comes before the line goes: while both show,
  the earlier one is all painted and the fill goes on in the next.
- A karaoke line whose time ends before its fill does (VLC's clock jumping forward a little, a computer busy for a
  moment) shows all painted for one frame before it goes, instead of leaving the screen half painted; a seek away, a
  sync step, a subtitle switch or turning the karaoke mode off takes it off at once.
- Going back to a passage's start with VLC (each lap after its gap, or Play at its end) sends the seek and the play
  as one order, so the picture and the sound start together; the sound used to start first, the picture catching up.

## [0.27.1] - 2026-10-27

### Changed

- The message for a file Windows would not let the app write names the file's "Read-only" mark among what to check.
- The executable's details in Windows (its description and copyright) are in English.
- The zip's read-me tells of the setup assistant, the free and optional FFmpeg, and where the projects, the seek
  bar's stills and the logs are kept.

### Fixed

- A settings or profiles file the app could not read is no longer written over with the defaults at the next save.
  One written by a newer version, or one Windows would not open, stays as it is, and the changes last until the app
  closes; a damaged one is copied aside, whole (`config.damaged-<date>-<time>.json`), before the first save over it.
  Settings tells which file and why, on every page.
- A video's project written by a newer version says that nothing changed now is kept, and saving a passage into a
  project the app cannot write is refused, with why, instead of seeming to work.
- Renaming, removing or creating the active profile undoes the profiles' change when the configuration cannot be
  saved, so the app never opens on a profile that is gone.
- A slow disk (an old hard drive, an antivirus scanning the file) no longer holds the app while it saves: the panel,
  the settings and the video's keys go on while a video's project or the settings are written, one write at a time,
  the newest written last. Forgetting a video waits for a write on its way, and nothing writes it back afterwards.
- A video's changes that still cannot be saved as the app closes now ask, in Windows' own box, whether to try again
  (after making room on the disk, say) or to close without them; they used to be lost quietly.
- Temporary files an app that stopped while saving left behind (beside the settings, the profiles, a video's project
  or an exported subtitle) are taken out the next time that file is read or exported, once they are an hour old. A
  package of seek-bar stills left half written is taken out as the app opens, and counts in the size the settings
  show until then.
- Switching the seek bar's stills to "taken again at each opening" no longer writes the stills taken until then to
  the disk.
- When the settings folder cannot take the log (a read-only or full disk), the log goes to the user's temporary
  folder, and the diagnostics name the folder really used.
- An error that keeps the app from opening is told in Windows' own box; it was told nowhere.
- A failed FFmpeg install in Settings shows its message as the app words it, not the raw error.
- Playing, pausing or seeking no longer flips back for a moment when the player's answer, read just before the
  command, arrives after it on a busy computer.

## [0.27.0] - 2026-10-27

### Added

- The whole interface in American English, beside Brazilian Portuguese: every screen, menu, notice, message and tip
  has its English catalog (`internal/presentation/i18n/locales/en-US`), with English plurals and a decimal point.
- Choosing the app language applies it at once, even with a video playing: the screens, the bar and the menu over the
  video, the subtitle line names, the correction box and the Explorer's names for the app all switch without
  reopening the app, and a settings draft not yet saved is kept.
- A setup assistant on the first opening asks four quick questions: the app language (applied at once), the language
  you speak and the one you study, the study mode, and the player and FFmpeg (found, chosen or installed there). Each
  step can be skipped; skipping, or closing the app, keeps what Windows tells: you speak its language and study English,
  or Brazilian Portuguese on an English Windows. A video the app was opened with waits for it.
- On a weak computer (two logical processors or fewer, under 4 GB of memory, or a voice model under 25× real time,
  measured once in the background on made-up sound), the first opening starts with the voice AI off and says why, in
  the setup assistant and in Settings › Study › Actors' voice; it can still be turned on, at the user's own risk.
  Someone who already uses the app keeps their choice.
- Settings › Help › Documentation teaches everything the app can do, in 14 topics (getting started, watching,
  subtitles, sync, study modes, karaoke mode, passages, correcting and exporting, shortcuts, profiles and appearance,
  the voice AI, players, where the app keeps everything, problems and solutions), in both languages. It names the
  profile's own keys, finds topics by any word with or without accents, opens the settings each topic is about, and
  shows the topics as a list in the panel beside the video. Help now has two pages: Documentation and About.
- Settings › General › Setup assistant opens it again when no video plays: it changes the active profile, and another
  language studied gets a profile of its own, named after it, while the old one stays.

### Changed

- Texts that named a fixed key ("Space", "Backspace", the arrows) now show the key the profile gives the command, or
  none when it has no shortcut; the karaoke help no longer quotes the default times, which can be changed.
- The players' reasons to choose them, the Explorer's names for the app, and the messages Windows shows when WebView2
  is missing are worded in the interface's language.
- Numbers on the screens (speeds, passage and editor times) carry the interface's decimal separator; the times are read
  back with either a comma or a point.
- The subtitle ladder's help names the study subtitle and the translation instead of English and Portuguese, and the
  install message for FFmpeg points to the official site and the "Browse…" button.
- The language setting is called "App language".
- A Windows display language the app does not speak now opens the app in English instead of Brazilian Portuguese
  (Portuguese from Portugal still opens it in Brazilian Portuguese, British English in American English).
- The first profile is named after the language studied, in the app language, instead of always the Portuguese name
  for English.
- The settings' subtitle preview speaks each line's language: the sample on top is in the language studied and the one
  below in the translation's (English, Portuguese, Spanish, French, German and Italian; English for the others).
- The README presents the app as it is today, with pictures of it: highlights, features by area, the default
  shortcuts, requirements, privacy and building from source. The guide to translating covers the documentation's
  file, the two catalogs and English as the language of a Windows the app does not speak.

### Fixed

- Turning on "always on top" or the sync notices, or any other choice that applies at once, no longer throws away the
  changes to the profile not yet saved on the same settings screen.

## [0.26.0] - 2026-10-07

The seek bar shows a still of the moment under the pointer, as video sites do: taken in the background without
crowding the film, and kept in the app's own folder, where each video's project now lives too, following a film that
is renamed or moved. When the app cannot keep something on the disk it says why and what to do, and loses nothing.

### Added

- The seek bar shows, above the pointer (and along a drag), the time under it and a still of that moment (240×135 at
  100%, growing with the screen's scale and the bar's size), as video sites do: a still only near the moment shows
  blurred until its own arrives, an empty box waits for the first, and a film with no stills shows the time alone. It
  stays over the subtitles even when they show after it, takes no clicks, and replaces the track's tip.
- While a film plays in the app's window, stills for its seek bar are taken in the background, one at a time: about
  10 s after it starts (at once when the pointer reaches the bar), one each 10 s of film (further apart past 1000
  stills), from coarse to fine so that the first hundred already cover the whole film, and the moment under the
  pointer before any other. Past those first hundred, with nobody at the bar, the system holds the background to a
  quarter of one processor, and on a battery only the moment under the pointer gets a still. Ending the film never
  waits for a still, and a film with no stills to take, or whose stills keep failing, leaves them off.
- The stills are bounded: each FFmpeg that takes one may commit at most 384 MB, a still that reads more than 24 MB of
  the film (or in more than 500 reads) is ended, and a film where that happens twice, or whose index (an MKV's Cues,
  an AVI's idx1 or OpenDML index) leaves out its video, gives no stills, since a seek there reads the film from its
  start. One of the app's FFmpeg runs at a time, the voice's or a still's; the still the pointer waits for never
  queues.
- A film's seek-bar stills are kept in the app's own folder (`%APPDATA%\LingoReel\thumbnails`), never beside the
  film: opened again, even renamed or moved (it is known by its size and its first and last 64 KiB), its stills show at
  once, and a film kept whole runs no FFmpeg at all. One film's stills take up to 64 MB and all of them up to 4 GB, the
  films opened longest ago leaving first. They never leave the disk with less than 1 GB free: older films leave to make
  room, and a disk still too full keeps the stills in memory for the session. A damaged package is refused within bounds
  and the film starts over. Each save at least doubles what is kept, so a long film is written some twice its size in
  all.
- The settings' Appearance page has a card for the seek bar's stills: showing them or not, taking them in the
  background on a battery too, keeping them on the disk or taking them again at each opening (in memory only, the disk
  never read or written), how much the kept stills take with a button that clears them, and the FFmpeg that takes them,
  the same as the voice's. Each choice applies at once, to the video playing too, and a configuration from before keeps
  the stills on and on the disk. The settings folder's note says the projects and the stills live there.
- When the app cannot keep something on the disk, its message says why and what to do: a full disk, Windows refusing
  the folder (its rights, an antivirus, a read-only file), a file another program holds, a write-protected disk, a disk
  or network share that is gone, or a path too long. It shows when saving the settings and profiles, a video's
  corrections and passages, forgetting a video, exporting a subtitle, clearing the kept stills, changing the file
  association or a karaoke mode choice, and when a session cannot start because the disk is full (no longer blamed on a
  slow player). The seek bar's card says when their disk is nearly full and new stills stay in memory.
- `lrdev stills <video>` measures a film's stills as the app takes them: the time, processor and reads of each, paced
  or not, beside VLC playing the film (decoding on the graphics card or on the processor) to count the pictures it
  loses, and held to a few processors as a smaller computer, printing the film's key and never its name.
- Stills of a film taken with the user's own FFmpeg, one short run each, at the lowest priority: the keyframe at or
  just before a moment (about 1 MB read and 150–240 ms here), scaled into a 384×216 box keeping the picture's shape
  (anamorphic DVDs included) as a JPEG. An HD film with no colour tags keeps the BT.709 colours players show,
  interlaced pictures are deinterlaced, and HDR (PQ, HLG) is made small first and tone-mapped after; Dolby Vision
  profile 5 gives no stills. The film is read only as the local file of the container the app opened.

### Changed

- A video's project (its corrections, sync, chosen tracks and mode, saved passages and where it was left) is kept in
  the app's own folder (`%APPDATA%\LingoReel\projects`), named by the film's content, so renaming or moving the
  film keeps it; nothing is written beside the film any more. A project an older version left beside the film, or in
  the app's folder by the film's path, moves in the first time the film opens: written, read back and compared before
  the old file goes, and one that cannot be removed (in use, in a folder without rights) stays, no longer read. A film
  written over in place (a download that ended, a better copy) keeps the project its path last kept. "Forget
  everything" leaves the film with nothing, old files included.
- Following the actors' voice is no longer marked experimental in the settings.

### Fixed

- Exporting over a subtitle that was there (the film's own, say) no longer spoils it when the disk fills up or the app
  stops halfway: the new file is written whole beside it and only then takes its place.
- A video's project that could not be saved is no longer lost quietly: the save stays due and runs again with the next
  one, as the session ends and as the app closes; the panel says why until a save works (the saves that wait a moment,
  such as the sync and the position, included), and one that fails as the session ends is told with its end.
- "Forget everything" that could not remove a video's project no longer left that opening's next changes unsaved.
- With "always use this player" ticked, profiles that could not be saved no longer kept the video from playing.
- Choosing a theme or the app's look now says so when it could not be kept.
- The karaoke mode's fill always ends while its line shows: a line that went before the actor stopped speaking (the
  voice found runs on past it), or a quick one, was left half painted, half in the fill's color and half in the line's.
  Over a line's last 400 ms (never more than 30% of it) what is left is painted, all of it 120 ms before the line goes.
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
- `lrdev voice-check` measures the voice found against a film's lines, printing only numbers, and `lrdev inspect
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
- The developer tool `lrdev` speaks English: its options are now `--subtitles`, `--start`, `--paused` and `--step`.
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
- This changelog.

## [0.16.2] - 2026-10-04

Each subtitle line has its settings page, with the preview beside the fields.

### Fixed

- The settings give each subtitle line its own page (Subtitles › Top and Bottom in the menu), with the preview beside
  the fields marking the line being edited.

## [0.16.1] - 2026-10-03

The passage's save button and training a line again.

### Fixed

- A field's button keeps its whole label on one line
- Training a line again plays it again, also at a passage's end

### Changed

- The README calls the app a beta

## [0.16.0] - 2026-10-03

Correct a line right over the video.

### Added

- Two clicks on a paused line correct it right over the video

### Changed

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

### Changed

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
- Share the Portuguese wording and the composition pieces with lrdev

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
- Sync shortcuts in lrdev play and overlay

### Changed

- Explain that the VLC endpoint file is a Lua page, not JSON

## [0.5.1] - 2026-10-01

Fixes from the audit against the docs, risk mitigations.

### Added

- Emulate the backward frame step with an exact seek
- Open other videos with subtitle files and analyze with a context
- Log VLC events that have no caller
- Add lrdev diag with monitors, scaling and a checklist

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

### Changed

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
- Add lrdev play to follow subtitles with a real player

### Fixed

- Show subtitles only once the player reports its real position

## [0.2.0] - 2026-09-29

Matroska reader, language detection and track selection.

### Added

- Add media tracks, language normalization and track selection
- Add Matroska reader for tracks and text subtitle frames
- Add analysis use cases for media and subtitle loading
- Add lrdev inspect command

## [0.1.0] - 2026-09-29

Subtitle core, text subtitle parsers and settings model.

### Added

- Add lrdev developer tool skeleton with build version
- Add subtitle timeline with interval lookup and offsets
- Add SRT, ASS/SSA and WebVTT parsers with encoding fallback
- Add appearance, study and settings domain models
- Add versioned JSON store with migrations and atomic writes

### Changed

- Add PowerShell check and build scripts
- Move tests into the tests tree and guard the layout

[Unreleased]: https://github.com/ediejohnny/lingoreel/compare/v0.27.2...HEAD
[0.27.2]: https://github.com/ediejohnny/lingoreel/compare/v0.27.1...v0.27.2
[0.27.1]: https://github.com/ediejohnny/lingoreel/compare/v0.27.0...v0.27.1
[0.27.0]: https://github.com/ediejohnny/lingoreel/compare/v0.26.0...v0.27.0
[0.26.0]: https://github.com/ediejohnny/lingoreel/compare/v0.25.0...v0.26.0
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
