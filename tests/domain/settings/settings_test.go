package settings_test

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func TestDefaultsAreValidAndConsistent(t *testing.T) {
	t.Parallel()
	config, profiles := settings.DefaultConfig(), settings.DefaultProfiles()
	if err := config.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	if err := profiles.Validate(); err != nil {
		t.Fatalf("default profiles invalid: %v", err)
	}
	if _, found := profiles.Find(config.ActiveProfile); !found {
		t.Fatalf("default active profile %q does not exist", config.ActiveProfile)
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(*settings.Config)
		valid  bool
	}{
		{"light theme", func(c *settings.Config) { c.Theme = settings.ThemeLight }, true},
		{"unknown theme", func(c *settings.Config) { c.Theme = "blue" }, false},
		{"empty theme", func(c *settings.Config) { c.Theme = "" }, false},
		{"empty active profile", func(c *settings.Config) { c.ActiveProfile = "" }, false},
		{"valid player path", func(c *settings.Config) { c.PlayerPaths["vlc"] = `C:\Program Files\VideoLAN\VLC\vlc.exe` }, true},
		{"upper case extension", func(c *settings.Config) { c.PlayerPaths["mpv"] = `D:\Tools\MPV.EXE` }, true},
		{"empty player path", func(c *settings.Config) { c.PlayerPaths["vlc"] = "" }, false},
		{"blank player path", func(c *settings.Config) { c.PlayerPaths["vlc"] = "   " }, false},
		{"upper case player id", func(c *settings.Config) { c.PlayerPaths["VLC"] = `C:\vlc.exe` }, false},
		{"path-like player id", func(c *settings.Config) { c.PlayerPaths[`..\x`] = `C:\vlc.exe` }, false},
		{"empty player id", func(c *settings.Config) { c.PlayerPaths[""] = `C:\vlc.exe` }, false},
		{"dismissed valid ids", func(c *settings.Config) { c.InstallRecommendations.Dismissed = []string{"mpv", "potplayer"} }, true},
		{"dismissed invalid id", func(c *settings.Config) { c.InstallRecommendations.Dismissed = []string{"mpv", "Pot Player"} }, false},
		{"interface language", func(c *settings.Config) { c.Language = "fr-CA" }, true},
		{"hostile interface language", func(c *settings.Config) { c.Language = "../fr" }, false},
		{"bar that stays the longest", func(c *settings.Config) { c.Bar.HideDelay = settings.Duration(settings.MaxBarHideDelay) }, true},
		{"bar that never stays", func(c *settings.Config) { c.Bar.HideDelay = 0 }, false},
		{"bar too small", func(c *settings.Config) { c.Bar.Style.Size = 0 }, false},
		{"box hint too small", func(c *settings.Config) { c.Edit.HintSize = 1 }, false},
		{"app text too large", func(c *settings.Config) { c.Interface.TextSize = 3 }, false},
		{"voice heard less readily", func(c *settings.Config) { c.Voice.Sensitivity = speech.SensitivityLow }, true},
		{"unknown voice sensitivity", func(c *settings.Config) { c.Voice.Sensitivity = "loud" }, false},
		{"wireless headphones' delay", func(c *settings.Config) { c.Voice.Delay = settings.Duration(200 * time.Millisecond) }, true},
		{"voice delay past a second", func(c *settings.Config) { c.Voice.Delay = settings.Duration(1500 * time.Millisecond) }, false},
		{"voice heard before it plays", func(c *settings.Config) { c.Voice.Delay = settings.Duration(-time.Millisecond) }, false},
		{"blank FFmpeg path", func(c *settings.Config) { c.Voice.FFmpegPath = "  " }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := settings.DefaultConfig()
			tc.mutate(&config)
			assertValidity(t, config.Validate(), tc.valid)
		})
	}
}

func TestProfileValidate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(*settings.Profile)
		valid  bool
	}{
		{"empty name", func(p *settings.Profile) { p.Name = "" }, false},
		{"name with surrounding spaces", func(p *settings.Profile) { p.Name = " Inglês" }, false},
		{"name at the limit with accents", func(p *settings.Profile) { p.Name = strings.Repeat("é", settings.MaxProfileNameRunes) }, true},
		{"name above the limit", func(p *settings.Profile) { p.Name = strings.Repeat("é", settings.MaxProfileNameRunes+1) }, false},
		{"automatic player", func(p *settings.Profile) { p.PreferredPlayer = "" }, true},
		{"preferred mpv", func(p *settings.Profile) { p.PreferredPlayer = "mpv" }, true},
		{"preferred player upper case", func(p *settings.Profile) { p.PreferredPlayer = "MPV" }, false},
		{"region tag", func(p *settings.Profile) { p.Languages.Translation = "pt-BR" }, true},
		{"script tag", func(p *settings.Profile) { p.Languages.Primary = "zh-Hant" }, true},
		{"three letter code", func(p *settings.Profile) { p.Languages.Audio = "eng" }, true},
		{"no translation", func(p *settings.Profile) { p.Languages.Translation = "" }, true},
		{"empty audio", func(p *settings.Profile) { p.Languages.Audio = "" }, false},
		{"language name instead of tag", func(p *settings.Profile) { p.Languages.Primary = "english" }, false},
		{"underscore tag", func(p *settings.Profile) { p.Languages.Translation = "pt_BR" }, false},
		{"invalid primary style", func(p *settings.Profile) { p.Primary.FontSize = 0 }, false},
		{"invalid translation style", func(p *settings.Profile) { p.Translation.Opacity = math.NaN() }, false},
		{"invalid layout", func(p *settings.Profile) { p.Layout.MaxWidth = 2 }, false},
		{"unknown study mode", func(p *settings.Profile) { p.Study.Mode = "passive" }, false},
		{"unknown reveal behavior", func(p *settings.Profile) { p.Study.Reveal = "" }, false},
		{"slowest speed", func(p *settings.Profile) { p.Speed = playback.MinSpeed }, true},
		{"zero speed", func(p *settings.Profile) { p.Speed = 0 }, false},
		{"speed NaN", func(p *settings.Profile) { p.Speed = playback.Speed(math.NaN()) }, false},
		{"precision step at minimum", func(p *settings.Profile) { p.Sync.PrecisionSeekStep = settings.Duration(settings.MinPrecisionSeekStep) }, true},
		{"precision step below minimum", func(p *settings.Profile) {
			p.Sync.PrecisionSeekStep = settings.Duration(settings.MinPrecisionSeekStep - time.Millisecond)
		}, false},
		{"precision step above maximum", func(p *settings.Profile) {
			p.Sync.PrecisionSeekStep = settings.Duration(settings.MaxPrecisionSeekStep + time.Millisecond)
		}, false},
		{"zero offset step", func(p *settings.Profile) { p.Sync.OffsetStep = 0 }, false},
		{"negative offset step", func(p *settings.Profile) { p.Sync.OffsetStep = settings.Duration(-100 * time.Millisecond) }, false},
		{"large offset step at maximum", func(p *settings.Profile) { p.Sync.OffsetLargeStep = settings.Duration(subtitle.MaxFastNudgeStep) }, true},
		{"offset step above 1 s", func(p *settings.Profile) {
			p.Sync.OffsetStep = settings.Duration(subtitle.MaxNudgeStep + time.Millisecond)
		}, false},
		{"large step below the step", func(p *settings.Profile) { p.Sync.OffsetLargeStep = p.Sync.OffsetStep - 1 }, false},
		{"large offset step above maximum", func(p *settings.Profile) {
			p.Sync.OffsetLargeStep = settings.Duration(subtitle.MaxFastNudgeStep + time.Millisecond)
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profile := settings.DefaultProfile()
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
	t.Parallel()
	french := settings.DefaultProfile()
	french.Name = "Francês"
	duplicate := settings.DefaultProfile()
	duplicate.Name = "INGLÊS"
	broken := settings.DefaultProfile()
	broken.Name = "Quebrado"
	broken.Speed = 9

	assertValidity(t, settings.Profiles{}.Validate(), false)
	assertValidity(t, settings.Profiles{Items: []settings.Profile{settings.DefaultProfile(), french}}.Validate(), true)
	assertValidity(t, settings.Profiles{Items: []settings.Profile{settings.DefaultProfile(), duplicate}}.Validate(), false)
	assertValidity(t, settings.Profiles{Items: []settings.Profile{settings.DefaultProfile(), broken}}.Validate(), false)

	profiles := settings.Profiles{Items: []settings.Profile{settings.DefaultProfile(), french}}
	if found, ok := profiles.Find("francês"); !ok || found.Name != "Francês" {
		t.Fatalf("Find(francês) = %q, %v", found.Name, ok)
	}
	if _, ok := profiles.Find("Coreano"); ok {
		t.Fatal("Find returned a profile that does not exist")
	}
	if _, ok := profiles.Find(""); ok {
		t.Fatal("Find matched an empty name")
	}
}

func TestDurationTextEncoding(t *testing.T) {
	t.Parallel()
	encoded, err := json.Marshal(settings.Duration(1500 * time.Millisecond))
	if err != nil || string(encoded) != `"1.5s"` {
		t.Fatalf("Marshal = %s, %v; want \"1.5s\"", encoded, err)
	}
	var decoded settings.Duration
	if err := json.Unmarshal([]byte(`"250ms"`), &decoded); err != nil || decoded.Std() != 250*time.Millisecond {
		t.Fatalf("Unmarshal 250ms = %v, %v", decoded.Std(), err)
	}
	for _, bad := range []string{`"abc"`, `"100"`, `""`, `"5 s"`} {
		if err := json.Unmarshal([]byte(bad), &decoded); !errors.Is(err, settings.ErrInvalidSettings) {
			t.Fatalf("Unmarshal %s error = %v, want ErrInvalidSettings", bad, err)
		}
	}
}

func assertValidity(t *testing.T, err error, valid bool) {
	t.Helper()
	if valid && err != nil {
		t.Fatalf("got %v, want valid", err)
	}
	if !valid && !errors.Is(err, settings.ErrInvalidSettings) {
		t.Fatalf("got %v, want ErrInvalidSettings", err)
	}
}
