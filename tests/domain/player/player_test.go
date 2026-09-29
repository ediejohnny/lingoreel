package player_test

import (
	"testing"

	"lingoreel/internal/domain/player"
)

func supportedOnly(ids ...player.ID) func(player.ID) bool {
	return func(id player.ID) bool {
		for _, supported := range ids {
			if supported == id {
				return true
			}
		}
		return false
	}
}

var everything = supportedOnly(player.MPV, player.VLC, player.PotPlayer, player.MPCHC, player.MPCBE)

func installed(ids ...player.ID) []player.Installation {
	var out []player.Installation
	for _, id := range ids {
		out = append(out, player.Installation{ID: id, Path: `C:\Players\` + string(id) + `.exe`})
	}
	return out
}

func TestChooseRespectsThePreferredPlayer(t *testing.T) {
	choice, ok := player.Choose(installed(player.MPV, player.VLC, player.PotPlayer), everything, player.VLC)
	if !ok || choice.Installation.ID != player.VLC || choice.PreferredMissing {
		t.Fatalf("Choose(prefer VLC) = %+v, %v", choice, ok)
	}
}

func TestChooseFallsBackToTheDefaultPriority(t *testing.T) {
	cases := []struct {
		name      string
		installed []player.Installation
		preferred player.ID
		want      player.ID
		missing   bool
	}{
		{"no preference picks mpv first", installed(player.PotPlayer, player.VLC, player.MPV), "", player.MPV, false},
		{"then VLC", installed(player.PotPlayer, player.VLC), "", player.VLC, false},
		{"then PotPlayer", installed(player.MPCHC, player.PotPlayer), "", player.PotPlayer, false},
		{"preferred uninstalled", installed(player.VLC), player.MPV, player.VLC, true},
		{"preferred unknown", installed(player.PotPlayer), "winamp", player.PotPlayer, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			choice, ok := player.Choose(tc.installed, everything, tc.preferred)
			if !ok || choice.Installation.ID != tc.want || choice.PreferredMissing != tc.missing {
				t.Fatalf("Choose = %+v, %v; want %s (missing=%v)", choice, ok, tc.want, tc.missing)
			}
		})
	}
}

func TestChooseSkipsPlayersWithoutAnAdapter(t *testing.T) {
	onlyVLC := supportedOnly(player.VLC)

	choice, ok := player.Choose(installed(player.MPV, player.VLC), onlyVLC, player.MPV)
	if !ok || choice.Installation.ID != player.VLC || !choice.PreferredMissing {
		t.Fatalf("Choose with mpv unsupported = %+v, %v", choice, ok)
	}
	if _, ok := player.Choose(installed(player.MPCHC, player.MPCBE), everything, ""); ok {
		t.Fatal("MPC players are outside the default priority and must not be picked automatically")
	}
	if choice, ok := player.Choose(installed(player.MPCHC), everything, player.MPCHC); !ok || choice.Installation.ID != player.MPCHC {
		t.Fatalf("an explicitly preferred supported player must be used: %+v, %v", choice, ok)
	}
}

func TestChooseWithNothingInstalled(t *testing.T) {
	if choice, ok := player.Choose(nil, everything, ""); ok || choice.PreferredMissing {
		t.Fatalf("Choose(nothing) = %+v, %v", choice, ok)
	}
	if choice, ok := player.Choose(nil, everything, player.VLC); ok || !choice.PreferredMissing {
		t.Fatalf("Choose(nothing, prefer VLC) = %+v, %v", choice, ok)
	}
}

func TestDisplayNames(t *testing.T) {
	cases := map[player.ID]string{player.MPV: "mpv", player.VLC: "VLC", player.PotPlayer: "PotPlayer", player.MPCHC: "MPC-HC", player.MPCBE: "MPC-BE", "custom": "custom"}
	for id, want := range cases {
		if got := id.DisplayName(); got != want {
			t.Fatalf("%s.DisplayName() = %q, want %q", id, got, want)
		}
	}
}
