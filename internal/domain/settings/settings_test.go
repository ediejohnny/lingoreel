package settings

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"lingoreel/internal/domain/playback"
)

func TestDefaultsAreValidAndConsistent(t *testing.T) {
	config, profiles := DefaultConfig(), DefaultProfiles()
	if err := config.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	if err := profiles.Validate(); err != nil {
		t.Fatalf("default profiles invalid: %v", err)
	}
	if _, found := profiles.Find(config.ActiveProfile); !found {
		t.Fatalf("default active profile %q does not exist", config.ActiveProfile)
	}
	profile := DefaultProfile()
	if profile.Languages != (LanguagePreferences{Audio: "en", Primary: "en", Translation: "pt-BR"}) {
		t.Fatalf("default languages = %+v", profile.Languages)
	}
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		valid  bool
	}{
		{"light theme", func(c *Config) { c.Theme = ThemeLight }, true},
		{"unknown theme", func(c *Config) { c.Theme = "blue" }, false},
		{"empty theme", func(c *Config) { c.Theme = "" }, false},
		{"empty active profile", func(c *Config) { c.ActiveProfile = "" }, false},
		{"valid player path", func(c *Config) { c.PlayerPaths["vlc"] = `C:\Program Files\VideoLAN\VLC\vlc.exe` }, true},
		{"upper case extension", func(c *Config) { c.PlayerPaths["mpv"] = `D:\Tools\MPV.EXE` }, true},
		{"relative player path", func(c *Config) { c.PlayerPaths["vlc"] = `vlc.exe` }, false},
		{"not an executable", func(c *Config) { c.PlayerPaths["vlc"] = `C:\Tools\vlc.bat` }, false},
		{"upper case player id", func(c *Config) { c.PlayerPaths["VLC"] = `C:\vlc.exe` }, false},
		{"path-like player id", func(c *Config) { c.PlayerPaths[`..\x`] = `C:\vlc.exe` }, false},
		{"empty player id", func(c *Config) { c.PlayerPaths[""] = `C:\vlc.exe` }, false},
		{"dismissed valid id", func(c *Config) { c.InstallRecommendations.Dismissed = []string{"mpv", "potplayer"} }, true},
		{"dismissed invalid id", func(c *Config) { c.InstallRecommendations.Dismissed = []string{"mpv", "Pot Player"} }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultConfig()
			tc.mutate(&config)
			assertValidity(t, config.Validate(), tc.valid)
		})
	}
}

func TestInstallRecommendationsIsDismissed(t *testing.T) {
	none := InstallRecommendations{}
	some := InstallRecommendations{Dismissed: []string{"mpv"}}
	all := InstallRecommendations{DismissAll: true}

	if none.IsDismissed("mpv") {
		t.Fatal("nothing dismissed, but mpv reported dismissed")
	}
	if !some.IsDismissed("mpv") || some.IsDismissed("vlc") {
		t.Fatal("per-player dismissal leaked to another player or was lost")
	}
	if !all.IsDismissed("vlc") || !all.IsDismissed("anything") {
		t.Fatal("dismiss all did not hide every player")
	}
}

func TestProfileValidate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Profile)
		valid  bool
	}{
		{"empty name", func(p *Profile) { p.Name = "" }, false},
		{"name with surrounding spaces", func(p *Profile) { p.Name = " Inglês" }, false},
		{"name at the limit with accents", func(p *Profile) { p.Name = strings.Repeat("é", maxProfileNameRunes) }, true},
		{"name above the limit", func(p *Profile) { p.Name = strings.Repeat("é", maxProfileNameRunes+1) }, false},
		{"automatic player", func(p *Profile) { p.PreferredPlayer = "" }, true},
		{"preferred mpv", func(p *Profile) { p.PreferredPlayer = "mpv" }, true},
		{"preferred player upper case", func(p *Profile) { p.PreferredPlayer = "MPV" }, false},
		{"region tag", func(p *Profile) { p.Languages.Translation = "pt-BR" }, true},
		{"script tag", func(p *Profile) { p.Languages.Primary = "zh-Hant" }, true},
		{"three letter code", func(p *Profile) { p.Languages.Audio = "eng" }, true},
		{"no translation", func(p *Profile) { p.Languages.Translation = "" }, true},
		{"empty audio", func(p *Profile) { p.Languages.Audio = "" }, false},
		{"language name instead of tag", func(p *Profile) { p.Languages.Primary = "english" }, false},
		{"underscore tag", func(p *Profile) { p.Languages.Translation = "pt_BR" }, false},
		{"invalid primary style", func(p *Profile) { p.Primary.FontSize = 0 }, false},
		{"invalid translation style", func(p *Profile) { p.Translation.Opacity = math.NaN() }, false},
		{"invalid layout", func(p *Profile) { p.Layout.MaxWidth = 2 }, false},
		{"unknown study mode", func(p *Profile) { p.Study.Mode = "passive" }, false},
		{"unknown reveal behavior", func(p *Profile) { p.Study.Reveal = "" }, false},
		{"slowest speed", func(p *Profile) { p.Speed = playback.MinSpeed }, true},
		{"zero speed", func(p *Profile) { p.Speed = 0 }, false},
		{"speed NaN", func(p *Profile) { p.Speed = playback.Speed(math.NaN()) }, false},
		{"precision step at minimum", func(p *Profile) { p.Sync.PrecisionSeekStep = Duration(minPrecisionSeekStep) }, true},
		{"precision step below minimum", func(p *Profile) { p.Sync.PrecisionSeekStep = Duration(minPrecisionSeekStep - time.Millisecond) }, false},
		{"precision step above maximum", func(p *Profile) { p.Sync.PrecisionSeekStep = Duration(maxPrecisionSeekStep + time.Millisecond) }, false},
		{"zero offset step", func(p *Profile) { p.Sync.OffsetStep = 0 }, false},
		{"negative offset step", func(p *Profile) { p.Sync.OffsetStep = Duration(-100 * time.Millisecond) }, false},
		{"large offset step at maximum", func(p *Profile) { p.Sync.OffsetLargeStep = Duration(maxOffsetLargeStep) }, true},
		{"large offset step above maximum", func(p *Profile) { p.Sync.OffsetLargeStep = Duration(maxOffsetLargeStep + time.Second) }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profile := DefaultProfile()
			tc.mutate(&profile)
			err := profile.Validate()
			if tc.valid && err != nil {
				t.Fatalf("Validate() = %v, want valid", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("Validate() accepted an invalid profile")
			}
		})
	}
}

func TestProfilesValidateAndFind(t *testing.T) {
	french := DefaultProfile()
	french.Name = "Francês"
	duplicate := DefaultProfile()
	duplicate.Name = "INGLÊS"
	broken := DefaultProfile()
	broken.Name = "Quebrado"
	broken.Speed = 9

	assertValidity(t, Profiles{}.Validate(), false)
	assertValidity(t, Profiles{Items: []Profile{DefaultProfile(), french}}.Validate(), true)
	assertValidity(t, Profiles{Items: []Profile{DefaultProfile(), duplicate}}.Validate(), false)
	assertValidity(t, Profiles{Items: []Profile{DefaultProfile(), broken}}.Validate(), false)

	profiles := Profiles{Items: []Profile{DefaultProfile(), french}}
	if found, ok := profiles.Find("francês"); !ok || found.Name != "Francês" {
		t.Fatalf("Find(francês) = %q, %v", found.Name, ok)
	}
	if _, ok := profiles.Find("Coreano"); ok {
		t.Fatal("Find returned a profile that does not exist")
	}
}

func TestDurationTextEncoding(t *testing.T) {
	encoded, err := json.Marshal(Duration(1500 * time.Millisecond))
	if err != nil || string(encoded) != `"1.5s"` {
		t.Fatalf("Marshal = %s, %v; want \"1.5s\"", encoded, err)
	}
	var decoded Duration
	if err := json.Unmarshal([]byte(`"250ms"`), &decoded); err != nil || decoded.Std() != 250*time.Millisecond {
		t.Fatalf("Unmarshal 250ms = %v, %v", decoded.Std(), err)
	}
	for _, bad := range []string{`"abc"`, `"100"`, `""`, `"5 s"`} {
		if err := json.Unmarshal([]byte(bad), &decoded); !errors.Is(err, ErrInvalidSettings) {
			t.Fatalf("Unmarshal %s error = %v, want ErrInvalidSettings", bad, err)
		}
	}
}

func TestProfileSurvivesJSONRoundTrip(t *testing.T) {
	original := DefaultProfile()
	original.PreferredPlayer = "vlc"
	original.Translation.Background.Enabled = true

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Profile
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("round trip changed the profile:\n got %+v\nwant %+v", decoded, original)
	}
	if !strings.Contains(string(encoded), `"precisionSeekStep":"100ms"`) || !strings.Contains(string(encoded), `"color":"#FFFFFF"`) {
		t.Fatalf("encoded profile is not human readable: %s", encoded)
	}
}

func assertValidity(t *testing.T, err error, valid bool) {
	t.Helper()
	if valid && err != nil {
		t.Fatalf("got %v, want valid", err)
	}
	if !valid && !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("got %v, want ErrInvalidSettings", err)
	}
}
