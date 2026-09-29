package subtitleformat

import (
	"bytes"
	"encoding/binary"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

var (
	utf8BOM    = []byte{0xEF, 0xBB, 0xBF}
	utf16LEBOM = []byte{0xFF, 0xFE}
	utf16BEBOM = []byte{0xFE, 0xFF}
)

// DecodeText converts subtitle bytes to a string. UTF-8 (with or without BOM)
// and UTF-16 with BOM are honored; any other input that is not valid UTF-8 is
// read as Windows-1252, the usual encoding of legacy Portuguese subtitle files.
func DecodeText(data []byte) string {
	switch {
	case bytes.HasPrefix(data, utf8BOM):
		data = data[len(utf8BOM):]
	case bytes.HasPrefix(data, utf16LEBOM):
		return decodeUTF16(data[len(utf16LEBOM):], binary.LittleEndian)
	case bytes.HasPrefix(data, utf16BEBOM):
		return decodeUTF16(data[len(utf16BEBOM):], binary.BigEndian)
	}
	if utf8.Valid(data) {
		return string(data)
	}
	return decodeWindows1252(data)
}

func decodeUTF16(data []byte, order binary.ByteOrder) string {
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = order.Uint16(data[2*i:])
	}
	return string(utf16.Decode(units))
}

// windows1252High maps bytes 0x80-0x9F; every other byte matches Latin-1.
var windows1252High = [32]rune{
	0x20AC, 0xFFFD, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0xFFFD, 0x017D, 0xFFFD,
	0xFFFD, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0xFFFD, 0x017E, 0x0178,
}

const (
	windows1252HighStart = 0x80
	windows1252HighEnd   = 0xA0
)

func decodeWindows1252(data []byte) string {
	var builder strings.Builder
	builder.Grow(len(data))
	for _, b := range data {
		switch {
		case b < windows1252HighStart:
			builder.WriteByte(b)
		case b < windows1252HighEnd:
			builder.WriteRune(windows1252High[b-windows1252HighStart])
		default:
			builder.WriteRune(rune(b))
		}
	}
	return builder.String()
}
