package app_test

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
)

func TestNothingWorksBeforeTheWindow(t *testing.T) {
	r := newRig(t)
	if _, err := r.api.OpenVideo(`D:\Filmes\x.mkv`); !errors.Is(err, app.ErrNotReady) {
		t.Fatalf("OpenVideo before Attach = %v", err)
	}
	if _, err := r.api.ChooseVideo(); !errors.Is(err, app.ErrNotReady) {
		t.Fatalf("ChooseVideo before Attach = %v", err)
	}
}

func TestOpenVideoDescribesTracksAndPlayers(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if _, err := r.api.OpenVideo(`D:\Filmes\notas.txt`); err == nil || !strings.Contains(err.Error(), ".txt") {
		t.Fatalf("OpenVideo of a text file = %v, want a message naming .txt", err)
	}
	video, err := r.api.OpenVideo(`D:\Filmes\Movie.mkv`)
	if err != nil {
		t.Fatal(err)
	}
	if video.Name != "Movie.mkv" || video.Duration != 5400 {
		t.Fatalf("video = %q, %v s", video.Name, video.Duration)
	}
	// The default profile studies English with a Brazilian translation.
	if video.Suggested != (app.Selection{Audio: 3, Primary: 4, Translation: 5}) {
		t.Fatalf("suggested = %+v", video.Suggested)
	}
	if len(video.Audio) != 2 || video.Audio[0].Label != "Português · AC3 · padrão" {
		t.Fatalf("audio = %+v", video.Audio)
	}
	labels := map[uint64]app.Track{}
	for _, track := range video.Subtitles {
		labels[track.Number] = track
	}
	if labels[4].Label != "Inglês — Full" || labels[5].Label != "Português do Brasil" {
		t.Fatalf("subtitle labels = %q / %q", labels[4].Label, labels[5].Label)
	}
	if picture := labels[6]; picture.Readable || !picture.Warning || picture.Note == "" {
		t.Fatalf("an image subtitle must be unreadable with a warning: %+v", picture)
	}
	players := video.Players
	if players.Choice != "fake" || len(players.Installed) != 2 || len(players.Recommendations) != 0 {
		t.Fatalf("players = %+v", players)
	}
	if pot := players.Installed[1]; pot.Supported || pot.Name != "PotPlayer" {
		t.Fatalf("a player without a plugin = %+v, want listed as unsupported", pot)
	}
}

func TestWatchAndControlTheSession(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if err := r.api.Watch(app.WatchRequest{Player: "fake"}); err == nil {
		t.Fatal("watched without a video")
	}
	if _, err := r.api.OpenVideo(`D:\Filmes\Movie.mkv`); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "potplayer", Primary: 4}); err == nil {
		t.Fatal("watched with a player that has no plugin")
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Audio: 3, Primary: 4, Translation: 5, AlwaysUse: true}); err != nil {
		t.Fatal(err)
	}
	if launched := r.plugin.launched; len(launched) != 1 || launched[0].AudioIndex != 1 || launched[0].Window.Width != 800 {
		t.Fatalf("launch = %+v", launched)
	}
	if preferred := r.settings.profiles.Items[0].PreferredPlayer; preferred != "fake" {
		t.Fatalf("'always use' saved %q as the preferred player", preferred)
	}

	eventually(t, "the state of the session", func() bool {
		state := r.api.PlaybackState()
		return state.Active && state.Status == "paused" && state.PositionText == "0:30" && state.DurationText == "1:30:00"
	})
	state := r.api.PlaybackState()
	if state.Lines.Primary != "Legenda de cima (inglês)" || !state.Capabilities.Seek || state.PrimaryOffset != "+0,000 s" {
		t.Fatalf("state = %+v", state)
	}
	if err := r.api.Nudge("primary", true); err != nil {
		t.Fatal(err)
	}
	if got := r.api.PlaybackState().PrimaryOffset; got != "+0,050 s" {
		t.Fatalf("offset after a nudge = %q, want the default 50 ms step", got)
	}
	if err := r.api.Nudge("middle", true); err == nil {
		t.Fatal("nudged an unknown line")
	}
	if err := r.api.SetTranslationVisible(false); err != nil || r.api.PlaybackState().TranslationVisible {
		t.Fatalf("hiding the translation = %v", err)
	}
	if err := r.api.TogglePause(); err != nil {
		t.Fatal(err)
	}

	if err := r.api.StopWatching(); err != nil {
		t.Fatal(err)
	}
	if ended, ok := r.ui.last(app.EventEnded); !ok || ended.(app.Ended).Error != "" {
		t.Fatalf("ended event = %+v, %v", ended, ok)
	}
	if state := r.api.PlaybackState(); state.Active {
		t.Fatal("still active after stopping")
	}
	if err := r.api.StopWatching(); err == nil {
		t.Fatal("stopped twice")
	}
	if err := r.api.TogglePause(); err == nil {
		t.Fatal("a command worked without a session")
	}
}

func TestSeekTakesOnlyPositionsAVideoCanHave(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if _, err := r.api.OpenVideo(`D:\Filmes\Movie.mkv`); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Primary: 4}); err != nil {
		t.Fatal(err)
	}
	if err := r.api.SeekTo(95.5); err != nil {
		t.Fatal(err)
	}
	if at, _ := r.plugin.current().lastSeek(); at != 95500*time.Millisecond {
		t.Fatalf("SeekTo(95.5) reached the player as %v", at)
	}
	// A day or more, infinities and NaN from the window become position 0,
	// never an overflowed duration.
	for _, hostile := range []float64{86400, -86400, math.Inf(1), math.NaN(), 1e300} {
		if err := r.api.SeekTo(hostile); err != nil {
			t.Fatal(err)
		}
		if at, _ := r.plugin.current().lastSeek(); at != 0 {
			t.Errorf("SeekTo(%v) reached the player as %v, want 0", hostile, at)
		}
	}
}

// A player in a window of its own (PotPlayer) is one with the app: the user
// closing it closes the app too, the session's work saved first.
func TestThePlayerClosingClosesTheApp(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if _, err := r.api.OpenVideo(`D:\Filmes\Movie.mkv`); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Primary: 4}); err != nil {
		t.Fatal(err)
	}
	if got := r.ui.titled(); len(got) != 1 || got[0] != "Movie.mkv — LingoReel" {
		t.Fatalf("titles %q: the window does not name the video", got)
	}
	if err := r.api.SeekTo(70); err != nil {
		t.Fatal(err)
	}
	r.plugin.current().Close() // the user closed the player
	eventually(t, "the app to close", func() bool { return r.ui.quitCount() == 1 && !r.api.PlaybackState().Active })
	// Saved before closing: the place reached is kept for "Continuar de".
	if saved, _ := r.projects.stored(t, `D:\Filmes\Movie.mkv`); saved.Last.Position.Duration() == 0 {
		t.Fatal("the app closed without keeping where the video was")
	}
	if got := r.ui.titled(); got[len(got)-1] != "LingoReel" {
		t.Fatalf("titles %q: the window still names the video", got)
	}
	if _, told := r.ui.last(app.EventEnded); told {
		t.Fatal("the screens were told of an end, as if the app went on")
	}
}

// A session the app ends itself never closes the app, whatever the player.
func TestStoppingTheVideoNeverClosesTheApp(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if _, err := r.api.OpenVideo(`D:\Filmes\Movie.mkv`); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Primary: 4}); err != nil {
		t.Fatal(err)
	}
	if err := r.api.StopWatching(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // the reports' goroutine, which must not quit either
	if r.ui.quitCount() != 0 {
		t.Fatal("stopping the video closed the app")
	}
	if _, told := r.ui.last(app.EventEnded); !told {
		t.Fatal("the screens were not told the video ended")
	}
}

// The player lost (a crash) leaves the app open, offering to watch again.
func TestALostPlayerKeepsTheAppOpen(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if _, err := r.api.OpenVideo(`D:\Filmes\Movie.mkv`); err != nil {
		t.Fatal(err)
	}
	if err := r.api.Watch(app.WatchRequest{Player: "fake", Primary: 4}); err != nil {
		t.Fatal(err)
	}
	running := r.plugin.current()
	running.lost.Store(true)
	running.Close()
	eventually(t, "the end of the session", func() bool {
		data, told := r.ui.last(app.EventEnded)
		ended, _ := data.(app.Ended)
		return told && ended.Resume
	})
	if r.ui.quitCount() != 0 {
		t.Fatal("a lost player closed the app")
	}
}

// A player that keeps its own audio track (PotPlayer) says so to the
// session screen, which then offers no choice.
func TestAPlayerThatKeepsItsAudioTrackSaysSo(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	players, err := r.api.Players()
	if err != nil {
		t.Fatal(err)
	}
	if players.Installed[0].ID != "fake" || players.Installed[0].FixedAudio {
		t.Fatalf("players = %+v; the plugin takes an audio track", players.Installed)
	}
	r.plugin.fixedAudio = true
	if players, _ := r.api.Players(); !players.Installed[0].FixedAudio {
		t.Fatalf("players = %+v; the plugin keeps its own audio track", players.Installed)
	}
}

func TestWithoutAPlayerTheCardsOfferOne(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	r.installed = nil
	players, err := r.api.Players()
	if err != nil {
		t.Fatal(err)
	}
	if players.Choice != "" || len(players.Recommendations) != 1 || !players.Recommendations[0].AutoInstall ||
		players.Recommendations[0].Benefit != "Um player de teste." {
		t.Fatalf("players = %+v", players)
	}
	r.installer.available = false
	if players, _ := r.api.Players(); players.Recommendations[0].AutoInstall {
		t.Fatal("automatic installation offered without winget")
	}
	if err := r.api.DismissRecommendation("fake"); err != nil {
		t.Fatal(err)
	}
	if players, _ := r.api.Players(); len(players.Recommendations) != 0 {
		t.Fatal("a dismissed card came back")
	}
	if !r.settings.config.InstallRecommendations.IsDismissed("fake") {
		t.Fatal("the dismissal was not saved")
	}
	if err := r.api.DismissRecommendation("Not An ID"); err == nil {
		t.Fatal("dismissed an invalid id")
	}
	if err := r.api.ShowRecommendations(); err != nil {
		t.Fatal(err)
	}
	if players, _ := r.api.Players(); len(players.Recommendations) != 1 {
		t.Fatal("the cards did not come back")
	}
}

func TestInstallAndDownload(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	if err := r.api.InstallPlayer("fake"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the installation result", func() bool {
		data, ok := r.ui.last(app.EventInstall)
		return ok && data.(app.InstallProgress).Done && data.(app.InstallProgress).Error == ""
	})
	if r.installer.installed[0] != "Fake.Player" {
		t.Fatalf("installed %q, want the descriptor's winget id", r.installer.installed)
	}
	r.installer.err = errors.New("exit 1")
	if err := r.api.InstallPlayer("fake"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the failure", func() bool {
		data, _ := r.ui.last(app.EventInstall)
		return data.(app.InstallProgress).Error != ""
	})
	if err := r.api.InstallPlayer("unknown"); !errors.Is(err, app.ErrUnknownPlayer) {
		t.Fatalf("installing an unknown player = %v", err)
	}
	r.installer.available = false
	if err := r.api.InstallPlayer("fake"); err == nil {
		t.Fatal("installed without winget")
	}

	if err := r.api.OpenDownloadPage("fake"); err != nil || r.ui.urls[0] != "https://example.org/fake" {
		t.Fatalf("download page = %v, %q", err, r.ui.urls)
	}
	if err := r.api.OpenDownloadPage("unknown"); err == nil {
		t.Fatal("opened a page for an unknown player")
	}
}

func TestPreferencesAndTheme(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	preferences := r.api.Preferences()
	if preferences.Theme != "system" || len(preferences.Speeds) < 5 || preferences.Speeds[0] != 0.25 {
		t.Fatalf("preferences = %+v", preferences)
	}
	if err := r.api.SetTheme("dark"); err != nil || r.settings.config.Theme != settings.ThemeDark {
		t.Fatalf("SetTheme = %v, saved %q", err, r.settings.config.Theme)
	}
	if err := r.api.SetTheme("purple"); err == nil {
		t.Fatal("accepted an unknown theme")
	}
	r.settings.saveError = errors.New("disk full")
	if err := r.api.SetTheme("light"); err == nil {
		t.Fatal("a failed save was not reported")
	}
}

func TestDamagedSettingsFallBackToTheDefaults(t *testing.T) {
	r := newRig(t)
	r.settings.config.Theme = settings.ThemeDark
	r.settings.loadErr = errors.New("corrupt")
	r.attach(t)
	if theme := r.api.Preferences().Theme; theme != "system" {
		t.Fatalf("theme = %q, want the default after a load error", theme)
	}
}

func TestDroppedFilesAndTheInitialVideo(t *testing.T) {
	r := newRig(t)
	r.attach(t)
	r.lifecycle.Dropped(nil)
	if _, ok := r.ui.last(app.EventVideoDropped); ok {
		t.Fatal("an empty drop was announced")
	}
	r.lifecycle.Dropped([]string{`D:\a.mkv`, `D:\b.mkv`})
	if path, _ := r.ui.last(app.EventVideoDropped); path != `D:\a.mkv` {
		t.Fatalf("dropped = %v, want the first file", path)
	}
	if first, second := r.api.InitialVideo(), r.api.InitialVideo(); first != `D:\Filmes\Inicial.mkv` || second != "" {
		t.Fatalf("InitialVideo = %q then %q, want it once", first, second)
	}
	r.ui.choose = `D:\c.mkv`
	if path, err := r.api.ChooseVideo(); err != nil || path != `D:\c.mkv` {
		t.Fatalf("ChooseVideo = %q, %v", path, err)
	}
}
