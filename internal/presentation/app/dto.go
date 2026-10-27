package app

// The DTOs are what the window receives: plain data, texts already in
// Portuguese, never domain objects. Times are in seconds.

// Video is an analyzed video ready for the session screen.
type Video struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Duration  float64   `json:"duration"`
	Audio     []Track   `json:"audio"`
	Subtitles []Track   `json:"subtitles"`
	Suggested Selection `json:"suggested"`
	Players   Players   `json:"players"`
	// Mode is the study mode to start with: the one last used on this video,
	// or else the profile's. Modes are the choices.
	Mode  string   `json:"mode"`
	Modes []Option `json:"modes"`
	// ProjectNote tells of a problem with the video's corrections file.
	ProjectNote string `json:"projectNote"`
	// LanguageNote tells when no subtitle is in a language the profile
	// asks for.
	LanguageNote string `json:"languageNote"`
	// SavedPassages are the video's passages kept for study, which the
	// session screen can start from.
	SavedPassages []SavedPassage `json:"savedPassages"`
	// Resume offers to go on where the video was left.
	Resume Resume `json:"resume"`
	// Listening is how far hearing the video, to tell which subtitle file
	// fits it, has gone; the video:listening event tells it as it goes.
	Listening Listening `json:"listening"`
}

// Opening is how far opening a video has gone.
type Opening struct {
	Path string `json:"path"`
	// Percent is how much was read, from 0 to 100.
	Percent int `json:"percent"`
}

// Listening is how far hearing a video's voice has gone.
type Listening struct {
	Path   string `json:"path"`
	Active bool   `json:"active"`
	// Percent is how much was heard, from 0 to 100.
	Percent int `json:"percent"`
	// Stretch is the stretch being heard, from 1, of Stretches.
	Stretch   int `json:"stretch"`
	Stretches int `json:"stretches"`
}

// Resume is where the video was left, when it is worth going back to.
type Resume struct {
	Offered bool `json:"offered"`
	// Position is in seconds; Text is "10:38".
	Position float64 `json:"position"`
	Text     string  `json:"text"`
}

// Track is one audio or subtitle choice.
type Track struct {
	Number uint64 `json:"number"`
	// Label is what the list shows: "English · Complete SRT (file)".
	Label    string `json:"label"`
	Language string `json:"language"`
	Readable bool   `json:"readable"`
	// Note explains a track that should not be chosen or was adjusted.
	Note string `json:"note"`
	// Warning marks a note about a problem (another cut, no match).
	Warning bool `json:"warning"`
}

// Selection holds track numbers; zero means none.
type Selection struct {
	Audio       uint64 `json:"audio"`
	Primary     uint64 `json:"primary"`
	Translation uint64 `json:"translation"`
}

// Players is what the app knows about the players on this computer.
type Players struct {
	// Choice is the player that will be used, empty when none can be.
	Choice string `json:"choice"`
	// Preferred is the user's preferred player, empty for automatic.
	Preferred string `json:"preferred"`
	// PreferredMissing tells that the preferred player is gone.
	PreferredMissing bool            `json:"preferredMissing"`
	Installed        []PlayerOption  `json:"installed"`
	Recommendations  []PlayerInstall `json:"recommendations"`
}

// PlayerOption is a player found on the computer.
type PlayerOption struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Supported bool   `json:"supported"`
	// FixedAudio tells this player keeps its own audio track: the session
	// screen offers no choice and says why.
	FixedAudio bool `json:"fixedAudio"`
}

// PlayerInstall is a player the user can install.
type PlayerInstall struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Benefit     string `json:"benefit"`
	AutoInstall bool   `json:"autoInstall"`
}

// WatchRequest is the user's choice on the session screen.
type WatchRequest struct {
	Audio       uint64 `json:"audio"`
	Primary     uint64 `json:"primary"`
	Translation uint64 `json:"translation"`
	Player      string `json:"player"`
	// AlwaysUse makes the player the preferred one.
	AlwaysUse bool `json:"alwaysUse"`
	// Mode is the study mode to watch in; empty is the profile's.
	Mode string `json:"mode"`
	// Resume starts where the video was left, or where a session the player
	// lost had reached.
	Resume bool `json:"resume"`
	// Passage names a saved passage to open the video studying it; Resume
	// is then ignored.
	Passage string `json:"passage"`
}

// Playback is the state of the session, sent a few times per second.
type Playback struct {
	Active bool `json:"active"`
	// Embedded tells the video is drawn inside the app's window, beside its
	// panel; otherwise the player shows it in a window of its own.
	Embedded bool    `json:"embedded"`
	Status   string  `json:"status"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Speed    float64 `json:"speed"`
	// PositionText and DurationText are "1:02:03".
	PositionText      string `json:"positionText"`
	DurationText      string `json:"durationText"`
	PrimaryOffset     string `json:"primaryOffset"`
	TranslationOffset string `json:"translationOffset"`
	// PrimaryVoice is what keeping the study line on the voice adds to its own
	// offset ("+0,800 s"); empty when nothing.
	PrimaryVoice       string `json:"primaryVoice"`
	TranslationVisible bool   `json:"translationVisible"`
	// Mode is the study mode; "always" keeps the translation shown.
	Mode         string       `json:"mode"`
	Capabilities Capabilities `json:"capabilities"`
	Lines        LineNames    `json:"lines"`
	Karaoke      Karaoke      `json:"karaoke"`
	Passage      Passage      `json:"passage"`
	Editing      Editing      `json:"editing"`
	// SavedPassages are the video's passages kept for study.
	SavedPassages []SavedPassage `json:"savedPassages"`
	// MarkedStart is the passage start marked by its key ("10:38"), empty
	// when none is.
	MarkedStart string `json:"markedStart"`
	// PassageOptions are how the next passage is studied.
	PassageOptions PassageOptions `json:"passageOptions"`
	// ProjectNote tells of a problem with the video's corrections file.
	ProjectNote string `json:"projectNote"`
	// Health tells of a player being reconnected or frozen; empty when it
	// answers.
	Health string `json:"health"`
	// Tracks are what the panel's subtitle card switches between.
	Tracks PlaybackTracks `json:"tracks"`
}

// PlaybackTracks are the video's tracks and the ones playing.
type PlaybackTracks struct {
	Audio     []Track   `json:"audio"`
	Subtitles []Track   `json:"subtitles"`
	Current   Selection `json:"current"`
	// AudioSwitch tells the player changes the audio track while playing.
	AudioSwitch bool `json:"audioSwitch"`
	// Loading tells a subtitle is being loaded for a line.
	Loading bool `json:"loading"`
}

// PassageOptions are how a passage is studied: fitted to the lines or not,
// and the plan.
type PassageOptions struct {
	Snap bool        `json:"snap"`
	Plan PassagePlan `json:"plan"`
}

// PassagePlan is how a passage is studied: pausing or looping at the end
// (after Gap seconds), with the helps and a goal of laps (0 is none).
type PassagePlan struct {
	Loop        bool    `json:"loop"`
	Gap         float64 `json:"gap"`
	Progressive bool    `json:"progressive"`
	Ladder      bool    `json:"ladder"`
	Goal        int     `json:"goal"`
}

// Editing is the line being corrected and what the editor can do. Times are
// on the playback clock, to the millisecond ("1:02:03,456").
type Editing struct {
	Active bool `json:"active"`
	// Line is "primary" or "translation"; LineName as the panel says it.
	Line          string `json:"line"`
	LineName      string `json:"lineName"`
	Original      string `json:"original"`
	Text          string `json:"text"`
	Start         string `json:"start"`
	End           string `json:"end"`
	OriginalStart string `json:"originalStart"`
	OriginalEnd   string `json:"originalEnd"`
	// Edited tells the line differs from its file.
	Edited  bool `json:"edited"`
	CanUndo bool `json:"canUndo"`
	CanRedo bool `json:"canRedo"`
	// StaleEdits counts saved corrections that no longer fit their file.
	StaleEdits int `json:"staleEdits"`
}

// EditRequest is the editor's text and times for the selected line.
type EditRequest struct {
	Text  string `json:"text"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// SavedPassage is a passage kept for study, with times as typed ("1:10:23").
type SavedPassage struct {
	Name  string `json:"name"`
	Start string `json:"start"`
	End   string `json:"end"`
	// Plan is how it is studied; Summary says it in words.
	Plan    PassagePlan `json:"plan"`
	Summary string      `json:"summary"`
	// Outside tells the passage starts after the end of this video (a file
	// of another cut): it cannot be studied.
	Outside bool `json:"outside"`
}

// Karaoke is the karaoke mode of the session.
type Karaoke struct {
	Enabled bool `json:"enabled"`
	// Indicator is "bar" or "countdown".
	Indicator string `json:"indicator"`
	// Repeat is how many times each line plays again; 0 is off.
	Repeat int `json:"repeat"`
	// Voice is how the line's voice is followed now: "ahead" (decoded with
	// FFmpeg), "live" (the player heard), "estimate", "battery" or "off" (the
	// voice model rests on the battery, or is switched off), or empty when
	// it is not followed.
	Voice string `json:"voice"`
}

// Passage is the passage being studied.
type Passage struct {
	Active    bool   `json:"active"`
	Start     string `json:"start"`
	End       string `json:"end"`
	Lap       int    `json:"lap"`
	Goal      int    `json:"goal"`
	Suspended bool   `json:"suspended"`
	AtEnd     bool   `json:"atEnd"`
}

// PassageRequest starts a passage. Times are typed by the user ("1:10:23").
type PassageRequest struct {
	Start string `json:"start"`
	End   string `json:"end"`
	// Snap widens the passage so no line is cut.
	Snap bool `json:"snap"`
	// Loop goes back by itself after Gap seconds; otherwise it pauses.
	Loop        bool    `json:"loop"`
	Gap         float64 `json:"gap"`
	Progressive bool    `json:"progressive"`
	Ladder      bool    `json:"ladder"`
	Goal        int     `json:"goal"`
}

// PassageTimes are a passage's times as the window shows them.
type PassageTimes struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Capabilities tell which controls work with this player.
type Capabilities struct {
	Pause bool `json:"pause"`
	Seek  bool `json:"seek"`
	Speed bool `json:"speed"`
}

// LineNames name the two subtitle lines for the sync controls.
type LineNames struct {
	Primary     string `json:"primary"`
	Translation string `json:"translation"`
}

// Ended tells why the session ended; Error is empty for a normal end.
type Ended struct {
	Error string `json:"error"`
	// Resume tells the session can start again where it was.
	Resume bool `json:"resume"`
}

// Preferences are the settings the window shows.
type Preferences struct {
	Theme  string    `json:"theme"`
	Speeds []float64 `json:"speeds"`
	// Panel is how the panel beside the video was left.
	Panel PanelState `json:"panel"`
	// Language is the interface's language, the tag of its catalog.
	Language string `json:"language"`
	// Interface is the app's own look over its theme.
	Interface InterfaceSettings `json:"interface"`
}

// InstallProgress reports an installation; Done with an empty Error is a
// success.
type InstallProgress struct {
	ID    string `json:"id"`
	Done  bool   `json:"done"`
	Error string `json:"error"`
}
