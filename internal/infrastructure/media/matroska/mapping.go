package matroska

import (
	"time"

	"lingoreel/internal/domain/media"
	"lingoreel/internal/domain/subtitle"
)

// maxPixelDimension rejects absurd frame sizes declared by broken files.
const maxPixelDimension = 1 << 16

// textSubtitleCodecs maps Matroska codec IDs to the text formats the player
// can read. Bitmap codecs such as S_HDMV/PGS and S_VOBSUB are absent: they
// would need OCR.
var textSubtitleCodecs = map[string]subtitle.Format{
	"S_TEXT/UTF8":        subtitle.FormatSRT,
	"S_TEXT/ASCII":       subtitle.FormatSRT,
	"S_TEXT/ASS":         subtitle.FormatASS,
	"S_ASS":              subtitle.FormatASS,
	"S_TEXT/SSA":         subtitle.FormatSSA,
	"S_SSA":              subtitle.FormatSSA,
	"S_TEXT/WEBVTT":      subtitle.FormatWebVTT,
	"D_WEBVTT/SUBTITLES": subtitle.FormatWebVTT,
	"D_WEBVTT/CAPTIONS":  subtitle.FormatWebVTT,
}

// subtitleFormatOf reports the text format of a subtitle track and whether its
// frames can be decoded.
func subtitleFormatOf(entry TrackEntry) (subtitle.Format, bool) {
	format, isText := textSubtitleCodecs[entry.CodecID]
	if !isText || entry.Type != trackTypeSubtitle {
		return "", false
	}
	if _, err := newFrameDecoder(entry.Encodings); err != nil {
		return format, false
	}
	return format, true
}

// MediaInfo converts the container description to the domain model.
func (container Container) MediaInfo() media.Info {
	info := media.Info{Duration: container.Duration, Tracks: make([]media.Track, 0, len(container.Tracks))}
	for _, entry := range container.Tracks {
		language, guessed := trackLanguage(entry)
		track := media.Track{
			Number:          entry.Number,
			UID:             entry.UID,
			Type:            trackTypeOf(entry.Type),
			Codec:           entry.CodecID,
			Language:        language,
			LanguageGuessed: guessed,
			Name:            entry.Name,
			Default:         entry.FlagDefault,
			Forced:          entry.FlagForced,
			HearingImpaired: entry.HearingImpaired,
		}
		switch track.Type {
		case media.TrackVideo:
			if entry.PixelWidth < maxPixelDimension && entry.PixelHeight < maxPixelDimension {
				track.Width, track.Height = int(entry.PixelWidth), int(entry.PixelHeight)
			}
			if entry.DefaultDuration > 0 {
				track.FrameRate = float64(time.Second) / float64(entry.DefaultDuration)
			}
		case media.TrackSubtitle:
			track.SubtitleFormat, track.Readable = subtitleFormatOf(entry)
		}
		info.Tracks = append(info.Tracks, track)
	}
	return info
}

// trackLanguage normalizes a track's language. The spec default "eng" of a
// track without a Language element is only a fallback: the track name comes
// first, and a language taken from the default is reported as guessed.
func trackLanguage(entry TrackEntry) (media.LanguageTag, bool) {
	if entry.LanguageDeclared || entry.LanguageBCP47 != "" {
		return media.NormalizeLanguage(entry.LanguageBCP47, entry.Language, entry.Name), false
	}
	if language := media.NormalizeLanguage("", "", entry.Name); language != "" {
		return language, false
	}
	return media.NormalizeLanguage("", entry.Language, ""), true
}

func trackTypeOf(value uint64) media.TrackType {
	switch value {
	case trackTypeVideo:
		return media.TrackVideo
	case trackTypeAudio:
		return media.TrackAudio
	case trackTypeSubtitle:
		return media.TrackSubtitle
	}
	return media.TrackOther
}

func (container Container) track(number uint64) (TrackEntry, bool) {
	for _, entry := range container.Tracks {
		if entry.Number == number {
			return entry, true
		}
	}
	return TrackEntry{}, false
}
