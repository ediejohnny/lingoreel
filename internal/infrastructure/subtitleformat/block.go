package subtitleformat

import (
	"fmt"
	"strings"

	"lingoreel/internal/domain/subtitle"
)

// matroskaASSFields is the field count of an ASS/SSA event stored in a Matroska
// block: ReadOrder, Layer (or Marked), Style, Name, MarginL, MarginR, MarginV,
// Effect, Text. Timing lives in the block itself, not in the payload.
const matroskaASSFields = 9

// BlockText extracts the displayable text of one subtitle block embedded in a
// container, whose payload carries the cue text without timing.
func BlockText(format subtitle.Format, payload []byte) (string, error) {
	if err := checkInputSize(payload); err != nil {
		return "", err
	}
	text := DecodeText(payload)

	switch format {
	case subtitle.FormatSRT:
		return subtitle.NormalizeText(cleanSRTText(text)), nil
	case subtitle.FormatWebVTT:
		return subtitle.NormalizeText(cleanVTTText(text)), nil
	case subtitle.FormatASS, subtitle.FormatSSA:
		fields := strings.SplitN(text, ",", matroskaASSFields)
		if len(fields) != matroskaASSFields {
			return "", fmt.Errorf("%w: ASS block with %d fields", ErrMalformed, len(fields))
		}
		// The WrapStyle lives in the track header, which is not read here;
		// the default style makes \n a soft break.
		return subtitle.NormalizeText(assToPlainText(fields[matroskaASSFields-1], false)), nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnsupportedFormat, format)
}
