package matroska_test

import (
	"bytes"
	"encoding/binary"
	"math"
)

// Element IDs used to build synthetic files, as written in the specification.
const (
	idEBML                 = 0x1A45DFA3
	idEBMLVersion          = 0x4286
	idDocType              = 0x4282
	idSegment              = 0x18538067
	idSeekHead             = 0x114D9B74
	idSeek                 = 0x4DBB
	idSeekID               = 0x53AB
	idSeekPosition         = 0x53AC
	idInfo                 = 0x1549A966
	idTimestampScale       = 0x2AD7B1
	idDuration             = 0x4489
	idTracks               = 0x1654AE6B
	idTrackEntry           = 0xAE
	idTrackNumber          = 0xD7
	idTrackUID             = 0x73C5
	idTrackType            = 0x83
	idFlagDefault          = 0x88
	idFlagForced           = 0x55AA
	idFlagHearingImpaired  = 0x55AB
	idDefaultDuration      = 0x23E383
	idName                 = 0x536E
	idLanguage             = 0x22B59C
	idLanguageBCP47        = 0x22B59D
	idCodecID              = 0x86
	idVideo                = 0xE0
	idPixelWidth           = 0xB0
	idPixelHeight          = 0xBA
	idContentEncodings     = 0x6D80
	idContentEncoding      = 0x6240
	idContentEncodingOrder = 0x5031
	idContentEncodingScope = 0x5032
	idContentEncodingType  = 0x5033
	idContentCompression   = 0x5034
	idContentCompAlgo      = 0x4254
	idContentCompSettings  = 0x4255
	idCluster              = 0x1F43B675
	idClusterTimestamp     = 0xE7
	idSimpleBlock          = 0xA3
	idBlockGroup           = 0xA0
	idBlock                = 0xA1
	idBlockDuration        = 0x9B
	idVoid                 = 0xEC
)

const (
	typeVideo    = 1
	typeAudio    = 2
	typeSubtitle = 0x11
)

// unknownSize is the 8-byte "size unknown" marker of streaming elements.
var unknownSize = []byte{0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

func join(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

func encodeID(id uint32) []byte {
	var out []byte
	for shift := 24; shift >= 0; shift -= 8 {
		if b := byte(id >> shift); b != 0 || len(out) > 0 {
			out = append(out, b)
		}
	}
	return out
}

// encodeVint writes the shortest variable size integer for n.
func encodeVint(n uint64) []byte {
	for length := 1; length <= 8; length++ {
		if n < uint64(1)<<(7*length)-1 {
			out := make([]byte, length)
			value := n | uint64(1)<<(7*length)
			for i := length - 1; i >= 0; i-- {
				out[i] = byte(value)
				value >>= 8
			}
			return out
		}
	}
	panic("value too large for a vint")
}

func el(id uint32, children ...[]byte) []byte {
	payload := join(children...)
	return join(encodeID(id), encodeVint(uint64(len(payload))), payload)
}

func unknownSizeEl(id uint32, children ...[]byte) []byte {
	return join(encodeID(id), unknownSize, join(children...))
}

func uintEl(id uint32, value uint64) []byte {
	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, value)
	for len(raw) > 1 && raw[0] == 0 {
		raw = raw[1:]
	}
	return el(id, raw)
}

// fixedUintEl always uses 8 bytes, so its size does not depend on the value.
func fixedUintEl(id uint32, value uint64) []byte {
	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, value)
	return el(id, raw)
}

func floatEl(id uint32, value float64) []byte {
	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, math.Float64bits(value))
	return el(id, raw)
}

func stringEl(id uint32, value string) []byte {
	return el(id, []byte(value))
}

func ebmlHeader(docType string) []byte {
	return el(idEBML, uintEl(idEBMLVersion, 1), stringEl(idDocType, docType))
}

func segmentInfo(scale uint64, durationTicks float64) []byte {
	return el(idInfo, uintEl(idTimestampScale, scale), floatEl(idDuration, durationTicks))
}

func tracksEl(entries ...[]byte) []byte {
	return el(idTracks, entries...)
}

func trackEntry(number uint64, trackType uint64, codec string, fields ...[]byte) []byte {
	base := [][]byte{uintEl(idTrackNumber, number), uintEl(idTrackUID, 1000+number), uintEl(idTrackType, trackType), stringEl(idCodecID, codec)}
	return el(idTrackEntry, append(base, fields...)...)
}

func compression(order, scope, encodingType, algo uint64, settings []byte) []byte {
	compressionFields := [][]byte{uintEl(idContentCompAlgo, algo)}
	if settings != nil {
		compressionFields = append(compressionFields, el(idContentCompSettings, settings))
	}
	return el(idContentEncodings, el(idContentEncoding,
		uintEl(idContentEncodingOrder, order),
		uintEl(idContentEncodingScope, scope),
		uintEl(idContentEncodingType, encodingType),
		el(idContentCompression, compressionFields...),
	))
}

func blockBody(track uint64, relative int16, flags byte, payload []byte) []byte {
	var header [3]byte
	binary.BigEndian.PutUint16(header[:2], uint16(relative))
	header[2] = flags
	return join(encodeVint(track), header[:], payload)
}

func simpleBlock(track uint64, relative int16, payload string) []byte {
	return el(idSimpleBlock, blockBody(track, relative, 0x80, []byte(payload)))
}

func blockGroup(track uint64, relative int16, duration uint64, payload []byte) []byte {
	return el(idBlockGroup, el(idBlock, blockBody(track, relative, 0, payload)), uintEl(idBlockDuration, duration))
}

func lacedBlockGroup(track uint64, relative int16, duration uint64, flags byte, lacedPayload []byte) []byte {
	return el(idBlockGroup, uintEl(idBlockDuration, duration), el(idBlock, blockBody(track, relative, flags, lacedPayload)))
}

func cluster(timestamp uint64, blocks ...[]byte) []byte {
	return el(idCluster, append([][]byte{uintEl(idClusterTimestamp, timestamp)}, blocks...)...)
}

func matroskaFile(segmentChildren ...[]byte) []byte {
	return join(ebmlHeader("matroska"), el(idSegment, segmentChildren...))
}

// subtitleFile is a minimal file with one SRT-style English subtitle track (2).
func subtitleFile(clusters ...[]byte) []byte {
	parts := [][]byte{
		segmentInfo(1_000_000, 60_000),
		tracksEl(trackEntry(2, typeSubtitle, "S_TEXT/UTF8", stringEl(idLanguage, "eng"))),
	}
	return matroskaFile(append(parts, clusters...)...)
}
