package app

// The settings screen's DTOs. Colors are "#RRGGBB" or "#RRGGBBAA"; sizes are
// pixels at 1080 lines of video; positions and margins are shares of the
// video (0.82 is 82%); times are seconds, except the sync steps, in
// milliseconds.

// SettingsView is everything the settings screen shows.
type SettingsView struct {
	General  GeneralSettings `json:"general"`
	Profiles []string        `json:"profiles"`
	Profile  ProfileSettings `json:"profile"`
	Players  []PlayerSetting `json:"players"`
	Options  SettingsOptions `json:"options"`
	// Folder is where the settings and the log are kept.
	Folder string `json:"folder"`
}

// GeneralSettings are the choices shared by every profile.
type GeneralSettings struct {
	Theme         string `json:"theme"`
	SyncNotice    bool   `json:"syncNotice"`
	ActiveProfile string `json:"activeProfile"`
	// Recommendations tells some player install suggestions are hidden.
	RecommendationsHidden bool `json:"recommendationsHidden"`
	// Language is the interface's language as chosen, empty for the
	// Windows display language; Languages are the ones the app speaks,
	// each named in itself.
	Language  string   `json:"language"`
	Languages []Option `json:"languages"`
}

// ProfileSettings are the active profile's choices.
type ProfileSettings struct {
	Name            string           `json:"name"`
	PreferredPlayer string           `json:"preferredPlayer"`
	Languages       LanguageSettings `json:"languages"`
	Primary         StyleSettings    `json:"primary"`
	Translation     StyleSettings    `json:"translation"`
	Layout          LayoutSettings   `json:"layout"`
	Speed           float64          `json:"speed"`
	SyncStep        int              `json:"syncStep"`
	SyncFastStep    int              `json:"syncFastStep"`
	// PrecisionStep is the precision shortcuts' jump, in ms.
	PrecisionStep int             `json:"precisionStep"`
	Karaoke       KaraokeSettings `json:"karaoke"`
	// Mode is the study mode a video opens with.
	Mode      string            `json:"mode"`
	Reveal    string            `json:"reveal"`
	Shortcuts []ShortcutSetting `json:"shortcuts"`
}

// LanguageSettings are language tags such as "en" or "pt-BR".
type LanguageSettings struct {
	Audio       string `json:"audio"`
	Primary     string `json:"primary"`
	Translation string `json:"translation"`
}

// StyleSettings is how one subtitle line looks.
type StyleSettings struct {
	FontFamily       string  `json:"fontFamily"`
	FontSize         float64 `json:"fontSize"`
	Responsive       bool    `json:"responsive"`
	Bold             bool    `json:"bold"`
	Italic           bool    `json:"italic"`
	Color            string  `json:"color"`
	Opacity          float64 `json:"opacity"`
	Outline          bool    `json:"outline"`
	OutlineColor     string  `json:"outlineColor"`
	OutlineWidth     float64 `json:"outlineWidth"`
	Shadow           bool    `json:"shadow"`
	ShadowColor      string  `json:"shadowColor"`
	ShadowX          float64 `json:"shadowX"`
	ShadowY          float64 `json:"shadowY"`
	ShadowBlur       float64 `json:"shadowBlur"`
	Background       bool    `json:"background"`
	BackgroundColor  string  `json:"backgroundColor"`
	BackgroundMargin float64 `json:"backgroundMargin"`
	Position         float64 `json:"position"`
	Alignment        string  `json:"alignment"`
	MaxLines         int     `json:"maxLines"`
}

// LayoutSettings are the margins and the text width, shares of the video.
type LayoutSettings struct {
	MarginLeft   float64 `json:"marginLeft"`
	MarginRight  float64 `json:"marginRight"`
	MarginTop    float64 `json:"marginTop"`
	MarginBottom float64 `json:"marginBottom"`
	MaxWidth     float64 `json:"maxWidth"`
}

// KaraokeSettings are the Modo Karaokê's choices.
type KaraokeSettings struct {
	Enabled      bool    `json:"enabled"`
	MinGap       float64 `json:"minGap"`
	Lead         float64 `json:"lead"`
	Indicator    string  `json:"indicator"`
	PreviewColor string  `json:"previewColor"`
	Fill         bool    `json:"fill"`
	FillColor    string  `json:"fillColor"`
	Repeat       int     `json:"repeat"`
}

// ShortcutSetting is one command and its keys.
type ShortcutSetting struct {
	Command  string `json:"command"`
	Name     string `json:"name"`
	Category string `json:"category"`
	// Chord is the keys as stored ("Ctrl+Comma"); Label as shown ("Ctrl+,").
	// Both are empty for a command without a shortcut.
	Chord string `json:"chord"`
	Label string `json:"label"`
}

// ShortcutCheck is a captured chord, checked: its stored form and label.
type ShortcutCheck struct {
	Chord string `json:"chord"`
	Label string `json:"label"`
}

// PlayerSetting is a player the app knows.
type PlayerSetting struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Supported bool   `json:"supported"`
	// Path is where it was found or set; Custom tells the user set it.
	Path   string `json:"path"`
	Custom bool   `json:"custom"`
}

// SettingsOptions are the choices and limits of the screen's controls.
type SettingsOptions struct {
	Fonts     []string        `json:"fonts"`
	Presets   []Option        `json:"presets"`
	Languages []Option        `json:"languages"`
	Speeds    []float64       `json:"speeds"`
	Modes     []Option        `json:"modes"`
	Limits    SettingsLimits  `json:"limits"`
	Defaults  ProfileSettings `json:"defaults"`
}

// Option is a value and its name.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// SettingsLimits are the ranges the domain accepts.
type SettingsLimits struct {
	FontSizeMin     float64 `json:"fontSizeMin"`
	FontSizeMax     float64 `json:"fontSizeMax"`
	OutlineMax      float64 `json:"outlineMax"`
	ShadowOffsetMax float64 `json:"shadowOffsetMax"`
	ShadowBlurMax   float64 `json:"shadowBlurMax"`
	PaddingMax      float64 `json:"paddingMax"`
	LinesMin        int     `json:"linesMin"`
	LinesMax        int     `json:"linesMax"`
	MarginMax       float64 `json:"marginMax"`
	GapMin          float64 `json:"gapMin"`
	GapMax          float64 `json:"gapMax"`
	LeadMin         float64 `json:"leadMin"`
	LeadMax         float64 `json:"leadMax"`
	RepeatMax       int     `json:"repeatMax"`
	SyncStepMin     int     `json:"syncStepMin"`
	SyncStepMax     int     `json:"syncStepMax"`
	SyncFastStepMax int     `json:"syncFastStepMax"`
	ProfileNameMax  int     `json:"profileNameMax"`
	// PrecisionStepMin and PrecisionStepMax bound the precision step, in ms.
	PrecisionStepMin int `json:"precisionStepMin"`
	PrecisionStepMax int `json:"precisionStepMax"`
}

// PreviewRequest asks for a picture of two styles on the settings screen.
type PreviewRequest struct {
	Primary     StyleSettings  `json:"primary"`
	Translation StyleSettings  `json:"translation"`
	Layout      LayoutSettings `json:"layout"`
	// Width and Height are the picture's size in screen pixels; Scale is
	// the display scaling (1.5 for 150%).
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Scale  float64 `json:"scale"`
	// Highlight outlines the line being edited: "primary", "translation",
	// or "" for none.
	Highlight string `json:"highlight"`
	// View is "" for the subtitles as they look between lines, or "fill"
	// for the Modo Karaokê's fill, painted in FillColor ("#RRGGBB").
	View      string `json:"view"`
	FillColor string `json:"fillColor"`
}
