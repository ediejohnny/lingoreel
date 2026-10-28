package app_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/speechpack"
	"github.com/ediejohnny/lingoreel/internal/domain/capacity"
	"github.com/ediejohnny/lingoreel/internal/presentation/app"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// fakePacks are the packs as the manager tells them, and what the screens
// asked of it.
type fakePacks struct {
	mu      sync.Mutex
	states  []speechpack.State
	fetch   error
	fetched []string
	removed []string
	cancels int
}

func (f *fakePacks) States() []speechpack.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.states
}

func (f *fakePacks) Fetch(language string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetched = append(f.fetched, language)
	return f.fetch
}

func (f *fakePacks) Cancel() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancels++
}

func (f *fakePacks) Remove(language string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, language)
	return nil
}

func packsRig(t *testing.T, machine capacity.Machine, packs *fakePacks) *rig {
	t.Helper()
	r := newRigWith(t, func(deps *app.Deps) {
		deps.Machine = func() capacity.Machine { return machine }
		deps.SpeechPacks = packs
	})
	r.attach(t)
	return r
}

func TestThePacksScreenMarksWhatSuitsTheComputerAndWordsEachFailure(t *testing.T) {
	t.Parallel()
	packs := &fakePacks{states: []speechpack.State{
		{Language: "en", Size: 100, Ready: true},
		{Language: "pt", Size: 300, Fetching: true, Done: 30, Total: 300},
		{Language: "es", Size: 300, Failed: fmt.Errorf("%w: 404", speechpack.ErrNotPublished)},
		{Language: "fr", Size: 300, Failed: fmt.Errorf("%w: other bytes", speechpack.ErrNotAsPublished)},
		{Language: "de", Size: 300, Failed: errors.New("connection reset")},
	}}
	r := packsRig(t, homeComputer, packs)

	view := r.api.SpeechPacks()
	if !view.Available || view.Level != "medium" || len(view.Packs) != 5 {
		t.Fatalf("view = %+v, want the five packs on a medium computer", view)
	}
	byLanguage := map[string]app.SpeechPackView{}
	for _, pack := range view.Packs {
		byLanguage[pack.Language] = pack
	}
	if !byLanguage["en"].Suits || byLanguage["pt"].Suits {
		t.Errorf("on a medium computer: en suits %v, pt suits %v; want only English", byLanguage["en"].Suits, byLanguage["pt"].Suits)
	}
	if pt := byLanguage["pt"]; !pt.Fetching || pt.Done != 30 || pt.Total != 300 || pt.Name == "" {
		t.Errorf("Portuguese under way: %+v", pt)
	}
	for language, key := range map[string]string{
		"es": "error.speechPack.notPublished",
		"fr": "error.speechPack.notAsPublished",
		"de": "error.speechPack.failed",
	} {
		if got := byLanguage[language].Error; got != ptbr.T(key) {
			t.Errorf("%s's failure reads %q, want %q", language, got, ptbr.T(key))
		}
	}
	if byLanguage["en"].Error != "" {
		t.Errorf("a ready pack tells a failure: %q", byLanguage["en"].Error)
	}

	// At the high level every pack suits; at the basic one, none.
	for machine, suits := range map[capacity.Machine]bool{studioComputer: true, schoolComputer: false} {
		other := packsRig(t, machine, &fakePacks{states: []speechpack.State{{Language: "pt"}}})
		if got := other.api.SpeechPacks().Packs[0].Suits; got != suits {
			t.Errorf("%s: Portuguese suits %v, want %v", machine.Processor, got, suits)
		}
	}
}

func TestThePacksScreenAsksTheManagerAndHearsItsChanges(t *testing.T) {
	t.Parallel()
	packs := &fakePacks{states: []speechpack.State{{Language: "en", Size: 100}}}
	r := packsRig(t, homeComputer, packs)

	if err := r.api.FetchSpeechPack("en"); err != nil {
		t.Fatal(err)
	}
	packs.fetch = speechpack.ErrBusy
	if err := r.api.FetchSpeechPack("pt"); err == nil || err.Error() != ptbr.T("error.speechPack.busy") {
		t.Errorf("a fetch while another runs: %v, want the busy message", err)
	}
	r.api.CancelSpeechPack()
	if err := r.api.RemoveSpeechPack("en"); err != nil {
		t.Fatal(err)
	}
	if len(packs.fetched) != 2 || packs.cancels != 1 || len(packs.removed) != 1 {
		t.Errorf("the manager was asked: fetch %v, cancels %d, removed %v", packs.fetched, packs.cancels, packs.removed)
	}

	packs.mu.Lock()
	packs.states = []speechpack.State{{Language: "en", Size: 100, Fetching: true, Done: 50, Total: 100}}
	packs.mu.Unlock()
	r.lifecycle.SpeechPacksChanged()
	data, ok := r.ui.last(app.EventSpeechPacks)
	if !ok {
		t.Fatal("the screens were not told the packs changed")
	}
	if view := data.(app.SpeechPacksView); view.Packs[0].Done != 50 {
		t.Errorf("the event told %+v, want the pack halfway", view)
	}

	// Without a manager the screens show no packs at all.
	bare := newRigWith(t, nil)
	bare.attach(t)
	if view := bare.api.SpeechPacks(); view.Available || view.Packs == nil {
		t.Errorf("without a manager: %+v, want unavailable with an empty list", view)
	}
}
