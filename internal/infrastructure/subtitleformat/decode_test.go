package subtitleformat

import "testing"

func TestDecodeText(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"plain UTF-8", []byte("Você"), "Você"},
		{"UTF-8 with BOM", append([]byte{0xEF, 0xBB, 0xBF}, "ação"...), "ação"},
		{"UTF-16 little endian", []byte{0xFF, 0xFE, 'O', 0, 'l', 0, 0xE1, 0}, "Olá"},
		{"UTF-16 big endian", []byte{0xFE, 0xFF, 0, 'O', 0, 'l', 0, 0xE1}, "Olá"},
		{"Windows-1252 fallback", []byte{'n', 0xE3, 'o', ' ', 0x93, 'x', 0x94, ' ', 0x80}, "não “x” €"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decodeText(tc.data); got != tc.want {
				t.Fatalf("decodeText = %q, want %q", got, tc.want)
			}
		})
	}
}
