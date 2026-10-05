package app_test

import (
	"slices"
	"sync"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// watchedStage is the video's place in the app's window; it notes how many
// events the window had heard when it was asked out of the fullscreen.
type watchedStage struct {
	mu     sync.Mutex
	ui     *fakeUI
	asked  []bool
	before []int
}

func (s *watchedStage) Place(bool, layout.Insets) {}

func (s *watchedStage) SetChrome(settings.BarPreferences, appearance.EditStyle, settings.Theme) {}

func (s *watchedStage) SetFullscreen(on bool) {
	s.ui.mu.Lock()
	heard := len(s.ui.events)
	s.ui.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked, s.before = append(s.asked, on), append(s.before, heard)
}

// The menu's "Configurações…" leaves the fullscreen, where the panel does
// not show, and only then opens the panel on the page asked; a page it does
// not know opens the first one.
func TestTheMenuOpensTheSettingsOutOfTheFullscreen(t *testing.T) {
	t.Parallel()
	ui := &fakeUI{}
	stage := &watchedStage{ui: ui}
	r := newRigWith(t, func(deps *app.Deps) { deps.Stage = stage })
	r.ui = ui
	r.attach(t)
	r.lifecycle.OpenSettings("subtitles-primary")
	request, _ := ui.last(app.EventPanel)
	if request != (app.PanelRequest{Section: app.PanelSettings, Page: "subtitles-primary"}) {
		t.Fatalf("panel request = %+v", request)
	}
	stage.mu.Lock()
	asked, before := slices.Clone(stage.asked), slices.Clone(stage.before)
	stage.mu.Unlock()
	if !slices.Equal(asked, []bool{false}) || before[0] != 0 {
		t.Fatalf("fullscreen asked %v with %v events already sent; want out of it before the panel opens", asked, before)
	}
	r.lifecycle.OpenSettings("nowhere")
	if request, _ := ui.last(app.EventPanel); request != (app.PanelRequest{Section: app.PanelSettings, Page: "general"}) {
		t.Fatalf("an unknown page opened %+v, want the first one", request)
	}
}

// Copying a line from the menu goes to the window's clipboard, never the
// real one, and the video says so; a switch the session refuses is told on
// the video too.
func TestTheMenuCopiesAndTellsOnTheVideo(t *testing.T) {
	t.Parallel()
	r := startWatching(t)
	before := r.display.noticeCount()
	r.lifecycle.CopyText("Hello there.")
	r.ui.mu.Lock()
	copied := slices.Clone(r.ui.copied)
	r.ui.mu.Unlock()
	if !slices.Equal(copied, []string{"Hello there."}) {
		t.Fatalf("copied %q", copied)
	}
	testkit.Eventually(t, "the notice of the copy", func() bool { return r.display.noticeCount() == before+1 })
	r.lifecycle.SwitchSubtitle(session.LinePrimary, 99)
	testkit.Eventually(t, "the refusal told on the video", func() bool { return r.display.noticeCount() == before+2 })
}

// "Abrir outro vídeo…" ends this video and opens the one picked, as a drop
// would; cancelling keeps the video playing.
func TestTheMenuOpensAnotherVideo(t *testing.T) {
	t.Parallel()
	r := startWatching(t)
	r.ui.mu.Lock()
	r.ui.choose = ""
	r.ui.mu.Unlock()
	r.lifecycle.OpenVideo()
	if !r.api.PlaybackState().Active {
		t.Fatal("cancelling the choice ended the video")
	}
	r.ui.mu.Lock()
	r.ui.choose = `D:\Filmes\Outro.mkv`
	r.ui.mu.Unlock()
	r.lifecycle.OpenVideo()
	if r.api.PlaybackState().Active {
		t.Fatal("the video goes on after another was picked")
	}
	if dropped, _ := r.ui.last(app.EventVideoDropped); dropped != `D:\Filmes\Outro.mkv` {
		t.Fatalf("the screens were asked to open %v", dropped)
	}
}
