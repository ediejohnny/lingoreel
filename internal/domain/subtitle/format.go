package subtitle

// Format identifies how a text subtitle source is encoded.
type Format string

// Supported text subtitle formats.
const (
	FormatSRT    Format = "srt"
	FormatASS    Format = "ass"
	FormatSSA    Format = "ssa"
	FormatWebVTT Format = "webvtt"
)
