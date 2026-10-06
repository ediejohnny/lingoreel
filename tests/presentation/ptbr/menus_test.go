package ptbr_test

import (
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

func menuState() controlbar.State {
	return controlbar.State{Study: controlbar.StudyState{
		Options: study.PracticePlan{Mode: study.EndLoop, Gap: 2 * time.Second, Goal: 5},
		MinGap:  10 * time.Second, Lead: 5 * time.Second, Repeat: 3,
		Saved: []controlbar.SavedItem{
			{Start: 10*time.Minute + 38*time.Second, End: 11*time.Minute + 30*time.Second},
			{Start: 95 * time.Minute, End: 96 * time.Minute, Outside: true},
		},
	}}
}

func TestBarEntryWordsEveryKind(t *testing.T) {
	t.Parallel()
	state, keys := menuState(), shortcut.Defaults()
	cases := []struct {
		entry controlbar.Entry
		want  controlbar.EntryText
	}{
		{controlbar.Entry{Kind: controlbar.EntryCommand, Command: shortcut.PassageMarkStart}, controlbar.EntryText{Text: "Marcar o início do trecho", Key: "Ctrl+F9"}},
		{controlbar.Entry{Kind: controlbar.EntryCommand, Command: shortcut.PassageMarkEnd}, controlbar.EntryText{Text: "Marcar o fim e salvar (marque o início antes)", Key: "Ctrl+F10"}},
		{controlbar.Entry{Kind: controlbar.EntryCommand, Command: shortcut.ReplayLine}, controlbar.EntryText{Text: "Treinar de novo (Modo Karaokê)", Key: "Backspace"}},
		{controlbar.Entry{Kind: controlbar.EntryStepper, Field: controlbar.FieldGap}, controlbar.EntryText{Text: "Intervalo entre as voltas", Value: "2 s"}},
		{controlbar.Entry{Kind: controlbar.EntryStepper, Field: controlbar.FieldGoal}, controlbar.EntryText{Text: "Meta", Value: "5 voltas"}},
		{controlbar.Entry{Kind: controlbar.EntryStepper, Field: controlbar.FieldMinGap}, controlbar.EntryText{Text: "Pausa mínima antes da fala", Value: "10 s"}},
		{controlbar.Entry{Kind: controlbar.EntryStepper, Field: controlbar.FieldRepeat}, controlbar.EntryText{Text: "Repetir cada fala", Value: "3×"}},
		{controlbar.Entry{Kind: controlbar.EntryPassage, Index: 0}, controlbar.EntryText{Text: "Trecho 10:38–11:30"}},
		{controlbar.Entry{Kind: controlbar.EntryPassage, Index: 1}, controlbar.EntryText{Text: "Trecho 1:35:00–1:36:00 (fora deste vídeo)"}},
		{controlbar.Entry{Kind: controlbar.EntryPassage, Index: 9}, controlbar.EntryText{}},
		{controlbar.Entry{Kind: controlbar.EntryLabel, Field: controlbar.FieldSaved}, controlbar.EntryText{Text: "TRECHOS SALVOS"}},
		{controlbar.Entry{Kind: controlbar.EntryChoice, Field: controlbar.FieldMode, Value: string(study.ModeDual)}, controlbar.EntryText{Text: "Modo: Dupla"}},
		{controlbar.Entry{Kind: controlbar.EntryChoice, Field: controlbar.FieldIndicator, Value: "countdown"}, controlbar.EntryText{Text: "Contagem regressiva"}},
		{controlbar.Entry{Kind: controlbar.EntryAction, Field: controlbar.FieldEdit, Value: "translation"}, controlbar.EntryText{Text: "Corrigir a tradução…"}},
	}
	for _, c := range cases {
		if got := ptbr.BarEntry(c.entry, state, keys, nil); got != c.want {
			t.Errorf("BarEntry(%+v) = %+v, want %+v", c.entry, got, c.want)
		}
	}
	// With a start marked, the start mark says it moves it, and the end is
	// plain.
	state.Study.MarkSet, state.Study.MarkStart = true, 10*time.Minute+38*time.Second
	if got := ptbr.BarEntry(controlbar.Entry{Kind: controlbar.EntryCommand, Command: shortcut.PassageMarkStart}, state, keys, nil); got.Text != "Marcar o início de novo (agora em 10:38)" {
		t.Errorf("start mark with a start = %q", got.Text)
	}
	if got := ptbr.BarEntry(controlbar.Entry{Kind: controlbar.EntryCommand, Command: shortcut.PassageMarkEnd}, state, keys, nil); got.Text != "Marcar o fim e salvar o trecho" {
		t.Errorf("end mark with a start = %q", got.Text)
	}
	// Without a key, no key is shown.
	if got := ptbr.BarEntry(controlbar.Entry{Kind: controlbar.EntryCommand, Command: shortcut.PassageStop}, state, keys, nil); got.Key != "" || got.Text != "Sair do trecho" {
		t.Errorf("leave without a key = %+v", got)
	}
}

func TestBarStatus(t *testing.T) {
	t.Parallel()
	passage := study.Segment{Start: 10*time.Minute + 38*time.Second, End: 11*time.Minute + 30*time.Second}
	cases := []struct {
		state controlbar.StudyState
		want  string
	}{
		{controlbar.StudyState{}, ""},
		{controlbar.StudyState{MarkSet: true, MarkStart: 10*time.Minute + 38*time.Second}, "Início do trecho: 10:38"},
		{controlbar.StudyState{Active: true, Passage: passage, Lap: 2}, "Trecho 10:38–11:30 · volta 2"},
		{controlbar.StudyState{Active: true, Passage: passage, Lap: 2, Goal: 5}, "Trecho 10:38–11:30 · volta 2 de 5"},
		{controlbar.StudyState{Active: true, Passage: passage, Suspended: true}, "Trecho 10:38–11:30 · fora do intervalo"},
		{controlbar.StudyState{Active: true, Passage: passage, AtEnd: true}, "Trecho 10:38–11:30 · fim: Reproduzir repete"},
		// A passage being studied wins over a start marked meanwhile.
		{controlbar.StudyState{Active: true, Passage: passage, Lap: 1, MarkSet: true}, "Trecho 10:38–11:30 · volta 1"},
	}
	for _, c := range cases {
		if got := ptbr.BarStatus(c.state); got != c.want {
			t.Errorf("BarStatus(%+v) = %q, want %q", c.state, got, c.want)
		}
	}
}

func TestBarTipsOfTheNewButtons(t *testing.T) {
	t.Parallel()
	keys := shortcut.Defaults()
	cases := []struct {
		item controlbar.Item
		want string
	}{
		{controlbar.Item{Action: controlbar.ActionFrameBack, Enabled: true}, "Quadro anterior (,)"},
		{controlbar.Item{Action: controlbar.ActionFrameForward, Enabled: true}, "Próximo quadro (.)"},
		{controlbar.Item{Action: controlbar.ActionFrameForward}, "Próximo quadro (.) (indisponível neste player)"},
		{controlbar.Item{Action: controlbar.ActionMenu, Menu: controlbar.MenuPassage, Enabled: true}, "Estudar trecho"},
		{controlbar.Item{Action: controlbar.ActionMenu, Menu: controlbar.MenuKaraoke, Enabled: true}, "Modo Karaokê"},
		{controlbar.Item{Action: controlbar.ActionMenu, Menu: controlbar.MenuSubtitles, Enabled: true}, "Legendas"},
	}
	for _, c := range cases {
		if got := ptbr.BarTip(c.item, controlbar.State{}, keys, nil); got != c.want {
			t.Errorf("BarTip(%+v) = %q, want %q", c.item, got, c.want)
		}
	}
}
