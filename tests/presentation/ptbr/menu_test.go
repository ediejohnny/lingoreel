package ptbr_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// everything is a menu with every item there is: a line under the click,
// the app, tracks, saved passages.
func everything() []contextmenu.Item {
	return contextmenu.Build(contextmenu.State{
		Bar: controlbar.State{
			Speed: 1, Capabilities: playback.Capabilities{Pause: true, Seek: true, Speed: true, Volume: true, AudioTrackSelection: true},
			Study: controlbar.StudyState{
				HasPrimary: true, HasTranslation: true, Mode: study.ModeDual, Indicator: study.IndicatorBar, Active: true,
				MinGap: 10 * time.Second, Lead: 5 * time.Second, Options: study.PracticePlan{Mode: study.EndLoop},
				Saved: []controlbar.SavedItem{{Start: time.Minute, End: 2 * time.Minute}},
			},
		},
		Tracks: []media.Track{
			{Number: 2, Type: media.TrackAudio, Language: "en"},
			{Number: 4, Type: media.TrackSubtitle, Language: "en", Readable: true},
		},
		Current: media.Selection{Audio: 2, Primary: 4},
		Under:   &contextmenu.Under{Line: contextmenu.LinePrimary, Text: "Hi."},
		App:     true,
	})
}

func eachItem(items []contextmenu.Item, visit func(contextmenu.Item)) {
	for _, item := range items {
		visit(item)
		eachItem(item.Children, visit)
	}
}

// Every item reads as something, and every command shows the key the
// profile gives it: changed, it shows changed; removed, it shows none.
func TestEveryMenuItemHasItsTextAndItsKey(t *testing.T) {
	t.Parallel()
	keys := shortcut.Defaults()
	keys[shortcut.PlayPause] = shortcut.Chord{Key: "P"}
	keys[shortcut.Mute] = shortcut.Chord{}
	eachItem(everything(), func(item contextmenu.Item) {
		if item.Kind == contextmenu.KindSeparator {
			return
		}
		text := ptbr.MenuText(item, keys, nil)
		if strings.TrimSpace(text.Text) == "" {
			t.Errorf("an item without text: %+v", item)
		}
		if item.Action != contextmenu.ActionCommand {
			return
		}
		if want := ptbr.ShortcutLabel(keys.Of(item.Command), nil); text.Key != want {
			t.Errorf("%s shows the key %q, want %q", item.Command, text.Key, want)
		}
		switch item.Command {
		case shortcut.PlayPause:
			if text.Key != "P" {
				t.Errorf("play shows %q, want the changed key P", text.Key)
			}
		case shortcut.Mute:
			if text.Key != "" {
				t.Errorf("the sound shows %q, want no key once removed", text.Key)
			}
		}
	})
}

// The texts the menu shows on its own, with their accents, and the
// system's key on "Fechar".
func TestTheMenuWordsItsOwnItems(t *testing.T) {
	t.Parallel()
	keys := shortcut.Defaults()
	want := map[contextmenu.Action]contextmenu.Text{
		contextmenu.ActionEditHere:  {Text: "Corrigir esta fala…"},
		contextmenu.ActionCopyHere:  {Text: "Copiar esta fala"},
		contextmenu.ActionStopVideo: {Text: "Encerrar o vídeo"},
		contextmenu.ActionOpenVideo: {Text: "Abrir outro vídeo…"},
		contextmenu.ActionClose:     {Text: "Fechar", Key: "Alt+F4"},
	}
	eachItem(everything(), func(item contextmenu.Item) {
		if expected, found := want[item.Action]; found {
			if got := ptbr.MenuText(item, keys, nil); got != expected {
				t.Errorf("action %d reads %+v, want %+v", item.Action, got, expected)
			}
			delete(want, item.Action)
		}
	})
	if len(want) > 0 {
		t.Errorf("items not in the menu: %v", want)
	}
	if got := ptbr.MenuText(contextmenu.Item{Kind: contextmenu.KindItem, Line: contextmenu.LineTranslation, Duration: -150 * time.Millisecond}, keys, nil); got.Text != "Legenda de baixo: −0,150 s" {
		t.Errorf("the offset reads %q", got.Text)
	}
}
