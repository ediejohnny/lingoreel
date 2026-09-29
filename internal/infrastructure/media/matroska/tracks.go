package matroska

// defaultLanguage is the spec default when a track omits its Language element.
const defaultLanguage = "eng"

// TrackEntry is a track as declared in the file.
type TrackEntry struct {
	Number          uint64
	UID             uint64
	Type            uint64
	CodecID         string
	Name            string
	Language        string
	LanguageBCP47   string
	FlagDefault     bool
	FlagForced      bool
	HearingImpaired bool
	// DefaultDuration is the duration of one frame in nanoseconds, 0 if unknown.
	DefaultDuration uint64
	PixelWidth      uint64
	PixelHeight     uint64
	Encodings       []ContentEncoding
}

// ContentEncoding is one compression or encryption layer applied to frames.
type ContentEncoding struct {
	Order               uint64
	Scope               uint64
	Type                uint64
	CompressionAlgo     uint64
	CompressionSettings []byte
}

// Content encoding values from the specification.
const (
	encodingScopeFrames    = 1
	encodingTypeCompress   = 0
	compressionZlib        = 0
	compressionBzlib       = 1
	compressionLZO         = 2
	compressionHeaderStrip = 3
	maxEncodings           = 8
)

func readTrackEntry(c *cursor, entry element) (TrackEntry, error) {
	track := TrackEntry{Language: defaultLanguage, FlagDefault: true}
	err := c.forEachChild(entry, func(field element) error {
		var err error
		switch field.id {
		case idTrackNumber:
			track.Number, err = c.readUint(field)
		case idTrackUID:
			track.UID, err = c.readUint(field)
		case idTrackType:
			track.Type, err = c.readUint(field)
		case idCodecID:
			track.CodecID, err = c.readString(field, maxStringBytes)
		case idName:
			track.Name, err = c.readString(field, maxStringBytes)
		case idLanguage:
			track.Language, err = c.readString(field, maxStringBytes)
		case idLanguageBCP47:
			track.LanguageBCP47, err = c.readString(field, maxStringBytes)
		case idFlagDefault:
			track.FlagDefault, err = readFlag(c, field)
		case idFlagForced:
			track.FlagForced, err = readFlag(c, field)
		case idFlagHearingImpaired:
			track.HearingImpaired, err = readFlag(c, field)
		case idDefaultDuration:
			track.DefaultDuration, err = c.readUint(field)
		case idVideo:
			err = readVideo(c, field, &track)
		case idContentEncodings:
			track.Encodings, err = readEncodings(c, field)
		}
		return err
	})
	return track, err
}

func readFlag(c *cursor, field element) (bool, error) {
	value, err := c.readUint(field)
	return value != 0, err
}

func readVideo(c *cursor, video element, track *TrackEntry) error {
	return c.forEachChild(video, func(field element) error {
		var err error
		switch field.id {
		case idPixelWidth:
			track.PixelWidth, err = c.readUint(field)
		case idPixelHeight:
			track.PixelHeight, err = c.readUint(field)
		}
		return err
	})
}

func readEncodings(c *cursor, encodings element) ([]ContentEncoding, error) {
	var result []ContentEncoding
	err := c.forEachChild(encodings, func(encoding element) error {
		if encoding.id != idContentEncoding {
			return nil
		}
		if len(result) >= maxEncodings {
			return ErrTooLarge
		}
		parsed := ContentEncoding{Scope: encodingScopeFrames}
		err := c.forEachChild(encoding, func(field element) error {
			var err error
			switch field.id {
			case idContentEncodingOrder:
				parsed.Order, err = c.readUint(field)
			case idContentEncodingScope:
				parsed.Scope, err = c.readUint(field)
			case idContentEncodingType:
				parsed.Type, err = c.readUint(field)
			case idContentCompression:
				err = c.forEachChild(field, func(setting element) error {
					var err error
					switch setting.id {
					case idContentCompAlgo:
						parsed.CompressionAlgo, err = c.readUint(setting)
					case idContentCompSettings:
						parsed.CompressionSettings, err = c.readBytes(setting, maxSettingsBytes)
					}
					return err
				})
			}
			return err
		})
		result = append(result, parsed)
		return err
	})
	return result, err
}
