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
	// Appearance is how the controls over the video look, for every
	// profile.
	Appearance AppearanceSettings `json:"appearance"`
	Players    []PlayerSetting    `json:"players"`
	// Voice is how the karaoke mode follows the actors' voice on this
	// computer.
	Voice VoiceSettings `json:"voice"`
	// Thumbnails are the seek bar's stills on this computer.
	Thumbnails ThumbnailSettings `json:"thumbnails"`
	Options    SettingsOptions   `json:"options"`
	// Folder is where the settings and the log are kept.
	Folder string `json:"folder"`
	// Notes tell why a settings file could not be read and what becomes of
	// it; empty when both were read.
	Notes []string `json:"notes"`
}

// GeneralSettings are the choices shared by every profile.
type GeneralSettings struct {
	Theme         string `json:"theme"`
	SyncNotice    bool   `json:"syncNotice"`
	ActiveProfile string `json:"activeProfile"`
	// AlwaysOnTop keeps the app's window in front of the other programs'.
	AlwaysOnTop bool `json:"alwaysOnTop"`
	// Recommendations tells some player install suggestions are hidden.
	RecommendationsHidden bool `json:"recommendationsHidden"`
	// Language is the interface's language as chosen, empty for the
	// Windows display language; Languages are the ones the app speaks,
	// each named in itself.
	Language  string   `json:"language"`
	Languages []Option `json:"languages"`
	// Computer is the computer the app runs on, as a game shows it.
	Computer ComputerView `json:"computer"`
}

// ComputerView is the computer as the app read it: its processor (empty when
// unread), its logical processors, its memory in whole GiB and its level,
// "basic", "medium" or "high"; Adjusted is the level the settings were last
// adjusted to, empty when never.
type ComputerView struct {
	Processor  string `json:"processor"`
	Processors int    `json:"processors"`
	MemoryGiB  int    `json:"memoryGiB"`
	Level      string `json:"level"`
	Adjusted   string `json:"adjusted"`
}

// ComputerChange is the question asked when the app runs on a computer of
// another level than the one the settings were adjusted to: Asked tells it
// waits for an answer.
type ComputerChange struct {
	Asked    bool         `json:"asked"`
	Computer ComputerView `json:"computer"`
}

// SpeechPacksView is every language's speech pack, with the computer's level
// ("basic", "medium" or "high"); Available is false when the app keeps none.
type SpeechPacksView struct {
	Available bool             `json:"available"`
	Level     string           `json:"level"`
	Packs     []SpeechPackView `json:"packs"`
}

// SpeechPackView is a language's pack: its name in the interface's language,
// the bytes fetching it takes now, whether it is ready or being fetched (Done
// of Total bytes), whether it suits the computer's level, and why its last
// fetch stopped, empty when it did not.
type SpeechPackView struct {
	Language string `json:"language"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Ready    bool   `json:"ready"`
	Fetching bool   `json:"fetching"`
	Done     int64  `json:"done"`
	Total    int64  `json:"total"`
	Suits    bool   `json:"suits"`
	Error    string `json:"error"`
}

// VoiceSettings are how the app hears the actors' voice on this computer.
type VoiceSettings struct {
	// Model lets the voice model work on this computer; ModelOnBattery, on
	// its battery too. Battery tells the computer has one (read only).
	Model          bool `json:"model"`
	ModelOnBattery bool `json:"modelOnBattery"`
	Battery        bool `json:"battery"`
	Follow         bool `json:"follow"`
	// Sync keeps the study line on the voice when the film slips.
	Sync bool `json:"sync"`
	// Sensitivity is "low", "normal" or "high".
	Sensitivity string `json:"sensitivity"`
	// DelayMs is the output's delay, in milliseconds (0 to 1000).
	DelayMs      int  `json:"delayMs"`
	EvenWhenBusy bool `json:"evenWhenBusy"`
	// FFmpegPath is the FFmpeg the user chose, empty when it is looked for;
	// FFmpegFound is the one in use, empty when none was found; FFmpegSlow
	// tells one did not answer in time (a busy computer), so it is not
	// missing; CanInstall tells winget can install one.
	FFmpegPath  string `json:"ffmpegPath"`
	FFmpegFound string `json:"ffmpegFound"`
	FFmpegSlow  bool   `json:"ffmpegSlow"`
	CanInstall  bool   `json:"canInstall"`
	// Weak tells the computer seems too weak for the voice AI (read only).
	Weak bool `json:"weak"`
}

// ThumbnailSettings are the seek bar's stills on this computer: taken at
// all, in the background on the battery too, and kept on the disk. Battery
// tells the computer has one, KeptBytes how much the kept stills take and
// Crowded that their disk is nearly full (all read only).
type ThumbnailSettings struct {
	On        bool  `json:"on"`
	OnBattery bool  `json:"onBattery"`
	OnDisk    bool  `json:"onDisk"`
	Battery   bool  `json:"battery"`
	KeptBytes int64 `json:"keptBytes"`
	Crowded   bool  `json:"crowded"`
}

// ProfileSettings are the active profile's choices.
type ProfileSettings struct {
	Name            string           `json:"name"`
	PreferredPlayer string           `json:"preferredPlayer"`
	Languages       LanguageSettings `json:"languages"`
	Primary         StyleSettings    `json:"primary"`
	Translation     StyleSettings    `json:"translation"`
	Layout          LayoutSettings   `json:"layout"`
	Marks           MarkSettings     `json:"marks"`
	Speed           float64          `json:"speed"`
	SyncStep        int              `json:"syncStep"`
	SyncFastStep    int              `json:"syncFastStep"`
	// PrecisionStep is the precision shortcuts' jump, in ms.
	PrecisionStep int `json:"precisionStep"`
	// TranslationOwnTiming shows the translation on its own file's time
	// instead of with the study line it translates.
	TranslationOwnTiming bool `json:"translationOwnTiming"`
	// VolumeStep is how far one touch of a volume key moves the level, in %.
	VolumeStep int             `json:"volumeStep"`
	Karaoke    KaraokeSettings `json:"karaoke"`
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
	// LineSpacing multiplies the font's own distance between lines.
	LineSpacing float64 `json:"lineSpacing"`
}

// LayoutSettings are the margins and the text width, shares of the video,
// the least gap between the two lines, in lines, and their order.
type LayoutSettings struct {
	MarginLeft   float64 `json:"marginLeft"`
	MarginRight  float64 `json:"marginRight"`
	MarginTop    float64 `json:"marginTop"`
	MarginBottom float64 `json:"marginBottom"`
	MaxWidth     float64 `json:"maxWidth"`
	LineGap      float64 `json:"lineGap"`
	// TranslationOnTop puts the translation above the study line.
	TranslationOnTop bool `json:"translationOnTop"`
}

// MarkSettings are the marks over the video besides the subtitles.
type MarkSettings struct {
	Notice NoticeSettings `json:"notice"`
	// NoticeDuration is how long a notice stays, in seconds.
	NoticeDuration float64           `json:"noticeDuration"`
	Selection      SelectionSettings `json:"selection"`
}

// NoticeSettings is how the notices over the video look. The size is in
// pixels at 1080 lines; Side is "left", "center" or "right".
type NoticeSettings struct {
	FontFamily string  `json:"fontFamily"`
	FontSize   float64 `json:"fontSize"`
	Color      string  `json:"color"`
	Background string  `json:"background"`
	Side       string  `json:"side"`
	Margin     float64 `json:"margin"`
	MaxWidth   float64 `json:"maxWidth"`
}

// SelectionSettings is the frame of the line being corrected, in pixels at
// 100% scaling.
type SelectionSettings struct {
	Color  string  `json:"color"`
	Edge   string  `json:"edge"`
	Width  float64 `json:"width"`
	Gap    float64 `json:"gap"`
	Radius float64 `json:"radius"`
}

// KaraokeSettings are the karaoke mode's choices. The bar's thickness is in
// pixels at 1080 lines; CountdownSize is a share of the line's size.
type KaraokeSettings struct {
	Enabled       bool    `json:"enabled"`
	MinGap        float64 `json:"minGap"`
	Lead          float64 `json:"lead"`
	Indicator     string  `json:"indicator"`
	PreviewColor  string  `json:"previewColor"`
	Fill          bool    `json:"fill"`
	FillColor     string  `json:"fillColor"`
	Repeat        int     `json:"repeat"`
	BarThickness  float64 `json:"barThickness"`
	BarColor      string  `json:"barColor"`
	CountdownSize float64 `json:"countdownSize"`
}

// AppearanceSettings are the controls over the video: the bar and its menus,
// and the box that corrects a line.
type AppearanceSettings struct {
	Bar  BarSettings  `json:"bar"`
	Edit EditSettings `json:"edit"`
}

// BarSettings is how the control bar looks. Icon is the icons' and texts'
// color, Loud the volume past 100%; Size multiplies the bar's size;
// HideDelay is in seconds.
type BarSettings struct {
	Panel      string  `json:"panel"`
	Menu       string  `json:"menu"`
	Accent     string  `json:"accent"`
	Icon       string  `json:"icon"`
	Loud       string  `json:"loud"`
	FontFamily string  `json:"fontFamily"`
	Size       float64 `json:"size"`
	HideDelay  float64 `json:"hideDelay"`
}

// EditSettings is how the box that corrects a line looks; HintSize is in
// pixels at 100%.
type EditSettings struct {
	Background string  `json:"background"`
	Text       string  `json:"text"`
	Hint       string  `json:"hint"`
	HintSize   float64 `json:"hintSize"`
}

// InterfaceSettings is the app's own look over its theme: Accent is "" for
// each theme's own blue; FontFamily is "" for the app's font; TextSize
// multiplies every size of the interface.
type InterfaceSettings struct {
	Accent     string  `json:"accent"`
	FontFamily string  `json:"fontFamily"`
	TextSize   float64 `json:"textSize"`
}

// BarPreviewRequest asks for a picture of the bar in a style; Width and
// Height are in screen pixels, Scale is the display scaling.
type BarPreviewRequest struct {
	Bar    BarSettings `json:"bar"`
	Width  int         `json:"width"`
	Height int         `json:"height"`
	Scale  float64     `json:"scale"`
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

// FillCheck is the color the karaoke mode's fill is painted in over the line
// on top: the chosen one, or the one standing in for it when the line would
// hide it (Changed).
type FillCheck struct {
	Used    string `json:"used"`
	Changed bool   `json:"changed"`
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
	// AppearanceDefaults are the controls' default look; EditHint is the
	// box's hint, for its sample.
	AppearanceDefaults AppearanceSettings `json:"appearanceDefaults"`
	EditHint           string             `json:"editHint"`
	// InterfaceDefaults is the app's default look over its theme.
	InterfaceDefaults InterfaceSettings `json:"interfaceDefaults"`
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
	PrecisionStepMin int     `json:"precisionStepMin"`
	PrecisionStepMax int     `json:"precisionStepMax"`
	LineSpacingMin   float64 `json:"lineSpacingMin"`
	LineSpacingMax   float64 `json:"lineSpacingMax"`
	LineGapMax       float64 `json:"lineGapMax"`
	NoticeSizeMax    float64 `json:"noticeSizeMax"`
	NoticeMarginMax  float64 `json:"noticeMarginMax"`
	NoticeWidthMin   float64 `json:"noticeWidthMin"`
	// NoticeDurationMin and NoticeDurationMax are in seconds.
	NoticeDurationMin  float64 `json:"noticeDurationMin"`
	NoticeDurationMax  float64 `json:"noticeDurationMax"`
	SelectionWidthMin  float64 `json:"selectionWidthMin"`
	SelectionWidthMax  float64 `json:"selectionWidthMax"`
	SelectionGapMax    float64 `json:"selectionGapMax"`
	SelectionRadiusMax float64 `json:"selectionRadiusMax"`
	BarThicknessMin    float64 `json:"barThicknessMin"`
	BarThicknessMax    float64 `json:"barThicknessMax"`
	CountdownSizeMin   float64 `json:"countdownSizeMin"`
	CountdownSizeMax   float64 `json:"countdownSizeMax"`
	BarSizeMin         float64 `json:"barSizeMin"`
	BarSizeMax         float64 `json:"barSizeMax"`
	// HideDelayMin and HideDelayMax are in seconds.
	HideDelayMin float64 `json:"hideDelayMin"`
	HideDelayMax float64 `json:"hideDelayMax"`
	HintSizeMin  float64 `json:"hintSizeMin"`
	HintSizeMax  float64 `json:"hintSizeMax"`
	TextSizeMin  float64 `json:"textSizeMin"`
	TextSizeMax  float64 `json:"textSizeMax"`
	// VolumeStepMin and VolumeStepMax bound a touch of a volume key, in %.
	VolumeStepMin int `json:"volumeStepMin"`
	VolumeStepMax int `json:"volumeStepMax"`
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
	// Highlight outlines the line being edited, as Marks' selection says:
	// "primary", "translation", or "" for none.
	Highlight string `json:"highlight"`
	// View is "" for the subtitles as they look between lines, "fill" for
	// the karaoke mode's fill, "cue" for a line shown before its time with
	// the time left, both as Karaoke says, or "notice" for a notice as
	// Marks says. Each part is read only by the view that shows it.
	View    string          `json:"view"`
	Karaoke KaraokeSettings `json:"karaoke"`
	Marks   MarkSettings    `json:"marks"`
	// Languages are the profile's: each sample line speaks its line's.
	Languages LanguageSettings `json:"languages"`
}

// WelcomeView is what the welcome asks, with the answers it suggests.
type WelcomeView struct {
	// Pending tells it is the first run, not answered yet: skipping it keeps
	// what Windows tells.
	Pending bool `json:"pending"`
	// AppLanguage is the app's language as chosen, empty for the Windows
	// one; AppLanguages are the ones the app speaks.
	AppLanguage  string   `json:"appLanguage"`
	AppLanguages []Option `json:"appLanguages"`
	// Spoken is the person's language, on the translation line; Studied is
	// the one studied, on the study line and in the audio.
	Spoken    string   `json:"spoken"`
	Studied   string   `json:"studied"`
	Languages []Option `json:"languages"`
	Mode      string   `json:"mode"`
	Modes     []Option `json:"modes"`
	// PreferredPlayer is the player to watch with, empty for the first one
	// found.
	PreferredPlayer string `json:"preferredPlayer"`
	// VoiceModel lets the voice AI work on this computer; Weak tells the
	// computer seems too weak for it, so it starts off and the screen warns.
	VoiceModel bool `json:"voiceModel"`
	Weak       bool `json:"weak"`
	// Computer is the computer the app runs on, with the level the welcome
	// suggests by.
	Computer ComputerView `json:"computer"`
}

// WelcomeChoices are the welcome's answers.
type WelcomeChoices struct {
	Spoken          string `json:"spoken"`
	Studied         string `json:"studied"`
	Mode            string `json:"mode"`
	PreferredPlayer string `json:"preferredPlayer"`
	VoiceModel      bool   `json:"voiceModel"`
}
