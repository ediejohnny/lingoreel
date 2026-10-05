package subtitleformat_test

import (
	"testing"

	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitleformat"
)

func TestDecodeText(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, ""},
		{"plain UTF-8", []byte("Você"), "Você"},
		{"UTF-8 with BOM", append([]byte{0xEF, 0xBB, 0xBF}, "ação"...), "ação"},
		{"only a UTF-8 BOM", []byte{0xEF, 0xBB, 0xBF}, ""},
		{"UTF-16 little endian", []byte{0xFF, 0xFE, 'O', 0, 'l', 0, 0xE1, 0}, "Olá"},
		{"UTF-16 big endian", []byte{0xFE, 0xFF, 0, 'O', 0, 'l', 0, 0xE1}, "Olá"},
		{"UTF-16 with odd trailing byte", []byte{0xFF, 0xFE, 'O', 0, 'k'}, "O"},
		{"Windows-1252 fallback", []byte{'n', 0xE3, 'o', ' ', 0x93, 'x', 0x94, ' ', 0x80}, "não “x” €"},
		{"Windows-1252 undefined byte", []byte{'a', 0x81, 'b'}, "a�b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := subtitleformat.DecodeText(tc.data); got != tc.want {
				t.Fatalf("DecodeText = %q, want %q", got, tc.want)
			}
		})
	}
}
