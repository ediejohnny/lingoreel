package player_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/domain/player"
)

func validDescriptor() player.Descriptor {
	return player.Descriptor{
		ID: "vlc", Name: "VLC",
		Install: player.Install{WingetID: "VideoLAN.VLC", DownloadURL: "https://www.videolan.org/vlc/", Benefit: "Abre quase tudo."},
	}
}

func TestDescriptorValidate(t *testing.T) {
	cases := map[string]func(*player.Descriptor){
		"id empty":                 func(d *player.Descriptor) { d.ID = "" },
		"id upper case":            func(d *player.Descriptor) { d.ID = "VLC" },
		"id with a path":           func(d *player.Descriptor) { d.ID = `..\vlc` },
		"id too long":              func(d *player.Descriptor) { d.ID = player.ID(strings.Repeat("a", 33)) },
		"id starting with a dash":  func(d *player.Descriptor) { d.ID = "-vlc" },
		"name empty":               func(d *player.Descriptor) { d.Name = "  " },
		"name with a line break":   func(d *player.Descriptor) { d.Name = "VLC\nmedia" },
		"benefit empty":            func(d *player.Descriptor) { d.Install.Benefit = "" },
		"benefit too long":         func(d *player.Descriptor) { d.Install.Benefit = strings.Repeat("a", 301) },
		"download over http":       func(d *player.Descriptor) { d.Install.DownloadURL = "http://www.videolan.org/" },
		"download without a host":  func(d *player.Descriptor) { d.Install.DownloadURL = "https:///vlc" },
		"download with a password": func(d *player.Descriptor) { d.Install.DownloadURL = "https://user:pw@videolan.org/" },
		"download as a file":       func(d *player.Descriptor) { d.Install.DownloadURL = `file:///C:/vlc.exe` },
		"download empty":           func(d *player.Descriptor) { d.Install.DownloadURL = "" },
		"winget id with a space":   func(d *player.Descriptor) { d.Install.WingetID = "VideoLAN VLC" },
		"winget id as an option":   func(d *player.Descriptor) { d.Install.WingetID = "--silent" },
	}
	for name, spoil := range cases {
		d := validDescriptor()
		spoil(&d)
		if err := d.Validate(); !errors.Is(err, player.ErrInvalidDescriptor) {
			t.Errorf("%s: Validate = %v, want ErrInvalidDescriptor", name, err)
		}
	}
	if err := validDescriptor().Validate(); err != nil {
		t.Fatalf("valid descriptor refused: %v", err)
	}
	noWinget := validDescriptor()
	noWinget.Install.WingetID = ""
	if err := noWinget.Validate(); err != nil {
		t.Fatalf("a player without a winget package must be accepted: %v", err)
	}
	limits := validDescriptor()
	limits.ID, limits.Install.Benefit = player.ID("a"+strings.Repeat("-", 31)), strings.Repeat("a", 300)
	if err := limits.Validate(); err != nil {
		t.Fatalf("a 32-character id and a 300-byte benefit are within the limits: %v", err)
	}
}

func TestIDValidMatchesWhatTheSettingsStore(t *testing.T) {
	for id, want := range map[player.ID]bool{"vlc": true, "mpc-hc": true, "mpv2": true, "": false, "Mpv": false, "a b": false} {
		if got := id.Valid(); got != want {
			t.Errorf("ID(%q).Valid() = %v, want %v", id, got, want)
		}
	}
}
