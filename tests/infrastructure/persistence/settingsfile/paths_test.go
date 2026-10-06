package settingsfile_test

import (
	"errors"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/settingsfile"
)

func TestCheckPlayerPaths(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		path  string
		valid bool
	}{
		{"absolute program", `C:\Program Files\VideoLAN\VLC\vlc.exe`, true},
		{"upper case extension", `D:\Tools\MPV.EXE`, true},
		{"network share", `\\server\apps\vlc.exe`, true},
		{"relative program", `vlc.exe`, false},
		{"relative with folder", `apps\vlc.exe`, false},
		{"drive without root", `C:vlc.exe`, false},
		{"script", `C:\Tools\vlc.bat`, false},
		{"no extension", `C:\Tools\vlc`, false},
		{"exe inside the name", `C:\Tools\vlc.exe.txt`, false},
		{"empty", ``, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := settings.DefaultConfig()
			config.PlayerPaths["vlc"] = tc.path
			err := settingsfile.CheckPlayerPaths(config)
			if tc.valid && err != nil {
				t.Fatalf("CheckPlayerPaths(%q) = %v, want valid", tc.path, err)
			}
			if !tc.valid && !errors.Is(err, settingsfile.ErrInvalidPlayerPath) {
				t.Fatalf("CheckPlayerPaths(%q) = %v, want ErrInvalidPlayerPath", tc.path, err)
			}
		})
	}
	if err := settingsfile.CheckPlayerPaths(settings.DefaultConfig()); err != nil {
		t.Fatalf("no configured players = %v, want valid", err)
	}
	// The FFmpeg the user chose is held to the same rule.
	config := settings.DefaultConfig()
	config.Voice.FFmpegPath = `ffmpeg.exe`
	if err := settingsfile.CheckPlayerPaths(config); !errors.Is(err, settingsfile.ErrInvalidPlayerPath) {
		t.Fatalf("a relative FFmpeg = %v, want ErrInvalidPlayerPath", err)
	}
	config.Voice.FFmpegPath = `C:\ffmpeg\bin\ffmpeg.exe`
	if err := settingsfile.CheckPlayerPaths(config); err != nil {
		t.Fatalf("an absolute FFmpeg = %v, want valid", err)
	}
}
