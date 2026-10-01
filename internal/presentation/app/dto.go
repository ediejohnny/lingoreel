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
}

// Track is one audio or subtitle choice.
type Track struct {
	Number uint64 `json:"number"`
	// Label is what the list shows: "Inglês · Complet SRT (arquivo)".
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
}

// Playback is the state of the session, sent a few times per second.
type Playback struct {
	Active   bool    `json:"active"`
	Status   string  `json:"status"`
	Position float64 `json:"position"`
	Duration float64 `json:"duration"`
	Speed    float64 `json:"speed"`
	// PositionText and DurationText are "1:02:03".
	PositionText       string       `json:"positionText"`
	DurationText       string       `json:"durationText"`
	PrimaryOffset      string       `json:"primaryOffset"`
	TranslationOffset  string       `json:"translationOffset"`
	TranslationVisible bool         `json:"translationVisible"`
	Capabilities       Capabilities `json:"capabilities"`
	Lines              LineNames    `json:"lines"`
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
}

// Preferences are the settings the window shows.
type Preferences struct {
	Theme  string    `json:"theme"`
	Speeds []float64 `json:"speeds"`
}

// InstallProgress reports an installation; Done with an empty Error is a
// success.
type InstallProgress struct {
	ID    string `json:"id"`
	Done  bool   `json:"done"`
	Error string `json:"error"`
}
