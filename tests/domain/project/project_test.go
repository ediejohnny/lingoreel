package project_test

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/project"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }

func ptr[T any](v T) *T { return &v }

func line(id string, start, end int64, text string) subtitle.Event {
	return subtitle.Event{ID: id, Start: ms(start), End: ms(end), OriginalText: text}
}

func TestHashOf(t *testing.T) {
	a, b := project.HashOf("That's sick!"), project.HashOf("That's sick?")
	if len(a) != 16 || strings.Trim(a, "0123456789abcdef") != "" {
		t.Fatalf("HashOf = %q, want 16 hex digits", a)
	}
	if a == b || a != project.HashOf("That's sick!") {
		t.Fatalf("hashes %q %q: not stable or not telling texts apart", a, b)
	}
	if project.HashOf("ação") == project.HashOf("acao") || project.HashOf("") == project.HashOf(" ") {
		t.Fatal("accents or blanks were ignored")
	}
}

func TestMillisRounds(t *testing.T) {
	cases := map[time.Duration]project.Millis{
		1499 * time.Microsecond: 1, 1500 * time.Microsecond: 2, 0: 0,
		-1500 * time.Microsecond: -2, time.Hour: 3_600_000,
	}
	for d, want := range cases {
		if got := project.MillisOf(d); got != want {
			t.Errorf("MillisOf(%v) = %d, want %d", d, got, want)
		}
	}
	if project.Millis(1234).Duration() != ms(1234) {
		t.Fatal("Duration did not convert back")
	}
}

func TestAnEditAppliesOnlyToItsOwnText(t *testing.T) {
	base := line("7", 10_000, 12_000, "Isso está doente!")
	edit := project.Edit{Original: project.HashOf(base.OriginalText), Text: ptr("Que loucura!"), End: ptr(project.Millis(12_500))}
	got, ok := edit.Apply(base)
	if !ok || got.Text() != "Que loucura!" || got.OriginalText != base.OriginalText || got.Start != base.Start || got.End != ms(12_500) {
		t.Fatalf("Apply = %+v %v", got, ok)
	}
	// The file changed under the edit: the line stays as the file has it.
	changed := line("7", 10_000, 12_000, "Isso é doentio!")
	if got, ok := edit.Apply(changed); ok || got != changed {
		t.Fatalf("an edit landed on another text: %+v %v", got, ok)
	}
	// An edit that would end the line before it starts is refused.
	late := project.Edit{Original: edit.Original, Start: ptr(project.Millis(12_000))}
	if got, ok := late.Apply(base); ok || got != base {
		t.Fatalf("start at the end was taken: %+v %v", got, ok)
	}
}

func TestDiffAndApplyAreInverse(t *testing.T) {
	base := line("3", 61_120, 63_800, "I am going to...")
	cases := []subtitle.Event{
		{ID: "3", Start: base.Start, End: base.End, OriginalText: base.OriginalText, OverrideText: "I'm gonna..."},
		{ID: "3", Start: ms(61_000), End: base.End, OriginalText: base.OriginalText},
		{ID: "3", Start: ms(60_999), End: ms(64_001), OriginalText: base.OriginalText, OverrideText: "Eu vou… ✓"},
	}
	for _, edited := range cases {
		edit, changed := project.Diff(base, edited)
		if !changed {
			t.Fatalf("no diff for %+v", edited)
		}
		back, ok := edit.Apply(base)
		if !ok || back.Text() != edited.Text() || back.Start != edited.Start || back.End != edited.End {
			t.Fatalf("Apply(Diff) = %+v %v, want %+v", back, ok, edited)
		}
	}
	// Only what changed is kept.
	edit, _ := project.Diff(base, cases[0])
	if edit.Start != nil || edit.End != nil || edit.Text == nil {
		t.Fatalf("text-only diff = %+v", edit)
	}
	if _, changed := project.Diff(base, base); changed {
		t.Fatal("an unchanged line has a diff")
	}
	// Writing the original text again is no change either.
	same := base
	same.OverrideText = base.OriginalText
	if _, changed := project.Diff(base, same); changed {
		t.Fatal("the original text counted as an edit")
	}
}

func TestATrackAppliesWhatStillFits(t *testing.T) {
	lines := []subtitle.Event{line("1", 0, 1000, "One."), line("2", 2000, 3000, "Two."), line("3", 4000, 5000, "Three.")}
	track := project.Track{Edits: map[string]project.Edit{
		"1": {Original: project.HashOf("One."), Text: ptr("Uno.")},
		"2": {Original: project.HashOf("Deux."), Text: ptr("Dos.")},            // the text changed
		"3": {Original: project.HashOf("Three."), End: ptr(project.Millis(1))}, // would end first
		"9": {Original: project.HashOf("Nine."), Text: ptr("Nueve.")},          // gone
	}}
	got, stale := track.Apply(lines)
	if stale != 3 {
		t.Fatalf("stale = %d, want 3", stale)
	}
	if got[0].Text() != "Uno." || got[1].Text() != "Two." || got[2] != lines[2] || len(got) != 3 {
		t.Fatalf("applied = %+v", got)
	}
	if lines[0].OverrideText != "" {
		t.Fatal("Apply changed the lines it was given")
	}
	if got, stale := (project.Track{}).Apply(lines); stale != 0 || len(got) != 3 {
		t.Fatalf("no edits: %d stale", stale)
	}
}

func valid() project.Project {
	p := project.New()
	p.Tracks["uid:42"] = project.Track{Offset: -120, Edits: map[string]project.Edit{
		"5": {Original: project.HashOf("x"), Text: ptr("y"), Start: ptr(project.Millis(10)), End: ptr(project.Millis(20))},
	}}
	p.Choice = project.Choice{Primary: "uid:42", Translation: "file:Subs/pt.srt", Mode: "dual"}
	p.Passages = []project.Passage{{Name: "Discussão na loja", Start: 1000, End: 5000}}
	return p
}

func TestValidateRefusesWhatAHostileFileCouldHold(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatalf("a good project: %v", err)
	}
	maxOffset := project.MillisOf(subtitle.MaxOffset)
	edit := func(change func(*project.Edit)) func(*project.Project) {
		return func(p *project.Project) {
			e := p.Tracks["uid:42"].Edits["5"]
			change(&e)
			p.Tracks["uid:42"].Edits["5"] = e
		}
	}
	cases := []struct {
		name   string
		change func(*project.Project)
		ok     bool
	}{
		{"offset at the limit", func(p *project.Project) { p.Tracks["uid:42"] = project.Track{Offset: maxOffset} }, true},
		{"offset past the limit", func(p *project.Project) { p.Tracks["uid:42"] = project.Track{Offset: -maxOffset - 1} }, false},
		{"blank key", func(p *project.Project) { p.Tracks[" "] = project.Track{} }, false},
		{"long key", func(p *project.Project) { p.Tracks[strings.Repeat("k", project.MaxKeyRunes+1)] = project.Track{} }, false},
		{"key at the limit", func(p *project.Project) { p.Tracks[strings.Repeat("é", project.MaxKeyRunes)] = project.Track{} }, true},
		{"bad hash", edit(func(e *project.Edit) { e.Original = "XYZ" }), false},
		{"upper-case hash", edit(func(e *project.Edit) { e.Original = strings.ToUpper(project.HashOf("x")) }), false},
		{"empty text", edit(func(e *project.Edit) { e.Text = ptr("") }), false},
		{"blank text", edit(func(e *project.Edit) { e.Text = ptr(" \n ") }), false},
		{"broken UTF-8", edit(func(e *project.Edit) { e.Text = ptr("a\xffb") }), false},
		{"long text", edit(func(e *project.Edit) { e.Text = ptr(strings.Repeat("a", project.MaxTextRunes+1)) }), false},
		{"text at the limit", edit(func(e *project.Edit) { e.Text = ptr(strings.Repeat("ç", project.MaxTextRunes)) }), true},
		{"negative start", edit(func(e *project.Edit) { e.Start = ptr(project.Millis(-1)) }), false},
		{"end past the limit", edit(func(e *project.Edit) { e.End = ptr(project.MillisOf(project.MaxTime) + 1) }), false},
		{"end equal to start", edit(func(e *project.Edit) { e.End = ptr(project.Millis(10)) }), false},
		{"only an end", edit(func(e *project.Edit) { e.Start = nil }), true},
		{"unknown mode", func(p *project.Project) { p.Choice.Mode = "passive" }, false},
		{"no mode", func(p *project.Project) { p.Choice.Mode = "" }, true},
		{"blank choice key", func(p *project.Project) { p.Choice.Audio = "  " }, false},
		{"passage without name", func(p *project.Project) { p.Passages[0].Name = "" }, false},
		{"passage name padded", func(p *project.Project) { p.Passages[0].Name = " Loja" }, false},
		{"passage name at the limit", func(p *project.Project) { p.Passages[0].Name = strings.Repeat("ã", project.MaxPassageNameLen) }, true},
		{"passage name too long", func(p *project.Project) { p.Passages[0].Name = strings.Repeat("a", project.MaxPassageNameLen+1) }, false},
		{"passage ending at its start", func(p *project.Project) { p.Passages[0].End = p.Passages[0].Start }, false},
		{"passage before zero", func(p *project.Project) { p.Passages[0].Start = -5 }, false},
		{"plan at the limits", func(p *project.Project) {
			p.Passages[0].Plan = &project.PassagePlan{Loop: true, Gap: 5000, Goal: 99}
		}, true},
		{"plan gap too long", func(p *project.Project) { p.Passages[0].Plan = &project.PassagePlan{Loop: true, Gap: 5001} }, false},
		{"plan gap negative", func(p *project.Project) { p.Passages[0].Plan = &project.PassagePlan{Gap: -1} }, false},
		{"plan goal too high", func(p *project.Project) { p.Passages[0].Plan = &project.PassagePlan{Goal: 100} }, false},
		{"plan goal negative", func(p *project.Project) { p.Passages[0].Plan = &project.PassagePlan{Goal: -1} }, false},
		{"last position at the limit", func(p *project.Project) { p.Last.Position = project.MillisOf(project.MaxTime) }, true},
		{"last position past the limit", func(p *project.Project) { p.Last.Position = project.MillisOf(project.MaxTime) + 1 }, false},
		{"last position negative", func(p *project.Project) { p.Last.Position = -1 }, false},
		{"last speed of the profile", func(p *project.Project) { p.Last.Speed = 0 }, true},
		{"last speed at the limits", func(p *project.Project) { p.Last.Speed = 2 }, true},
		{"last speed too fast", func(p *project.Project) { p.Last.Speed = 2.5 }, false},
		{"last speed negative", func(p *project.Project) { p.Last.Speed = -1 }, false},
		{"last speed not a number", func(p *project.Project) { p.Last.Speed = math.NaN() }, false},
		{"karaoke at the limits", func(p *project.Project) {
			p.Last.Karaoke = &project.Karaoke{MinGap: 3000, Lead: 2000, Indicator: "bar", Repeat: 10}
		}, true},
		{"karaoke without an indicator", func(p *project.Project) {
			p.Last.Karaoke = &project.Karaoke{MinGap: 10000, Lead: 5000}
		}, false},
		{"karaoke gap too short", func(p *project.Project) {
			p.Last.Karaoke = &project.Karaoke{MinGap: 100, Lead: 5000, Indicator: "bar"}
		}, false},
		{"karaoke repeats too many", func(p *project.Project) {
			p.Last.Karaoke = &project.Karaoke{MinGap: 10000, Lead: 5000, Indicator: "bar", Repeat: 11}
		}, false},
		{"too many passages", func(p *project.Project) {
			p.Passages = make([]project.Passage, project.MaxPassages+1)
			for i := range p.Passages {
				p.Passages[i] = project.Passage{Name: "P", Start: 0, End: 1}
			}
		}, false},
		{"too many tracks", func(p *project.Project) {
			for i := range project.MaxTracks {
				p.Tracks["file:"+strings.Repeat("x", i+1)] = project.Track{}
			}
		}, false},
	}
	for _, c := range cases {
		p := valid()
		c.change(&p)
		err := p.Validate()
		if (err == nil) != c.ok {
			t.Errorf("%s: Validate = %v, want ok %v", c.name, err, c.ok)
		}
		if err != nil && !errors.Is(err, project.ErrInvalid) {
			t.Errorf("%s: %v is not ErrInvalid", c.name, err)
		}
	}
}

func TestTheHistoryUndoesAndRedoesInOrder(t *testing.T) {
	change := func(text string) project.Change {
		return project.Change{Line: 1, After: subtitle.Event{ID: "1", OriginalText: text}}
	}
	h := project.NewHistory(2)
	if h.CanUndo() || h.CanRedo() {
		t.Fatal("a new history has something to undo")
	}
	h.Record(change("a"))
	h.Record(change("b"))
	h.Record(change("c")) // the limit drops "a"
	for _, want := range []string{"c", "b"} {
		got, ok := h.Undo()
		if !ok || got.After.OriginalText != want {
			t.Fatalf("Undo = %+v %v, want %s", got, ok, want)
		}
	}
	if _, ok := h.Undo(); ok {
		t.Fatal("undid past the limit")
	}
	if got, ok := h.Redo(); !ok || got.After.OriginalText != "b" {
		t.Fatalf("Redo = %+v %v", got, ok)
	}
	// A new change drops what was undone.
	h.Record(change("d"))
	if h.CanRedo() {
		t.Fatal("redo survived a new change")
	}
	if got, _ := h.Undo(); got.After.OriginalText != "d" {
		t.Fatalf("Undo after the new change = %+v", got)
	}
	// A limit below one keeps one change.
	one := project.NewHistory(0)
	one.Record(change("x"))
	one.Record(change("y"))
	if got, ok := one.Undo(); !ok || got.After.OriginalText != "y" || one.CanUndo() {
		t.Fatalf("limit 0: %+v %v, can undo %v", got, ok, one.CanUndo())
	}
}

// Saving the same span twice keeps both: the second gets " (2)", and so on.
// Case and the spaces around do not make a name new.
func TestUniqueNameKeepsEveryPassage(t *testing.T) {
	passages := func(names ...string) []project.Passage {
		list := make([]project.Passage, len(names))
		for i, name := range names {
			list[i] = project.Passage{Name: name, Start: 0, End: 1000}
		}
		return list
	}
	cases := []struct {
		name  string
		taken []project.Passage
		base  string
		want  string
	}{
		{"free", nil, "Trecho 10:38–11:30", "Trecho 10:38–11:30"},
		{"taken once", passages("Trecho 1"), "Trecho 1", "Trecho 1 (2)"},
		{"taken twice", passages("Trecho 1", "Trecho 1 (2)"), "Trecho 1", "Trecho 1 (3)"},
		{"a gap is filled", passages("Trecho 1", "Trecho 1 (3)"), "Trecho 1", "Trecho 1 (2)"},
		{"case", passages("trecho 1"), "Trecho 1", "Trecho 1 (2)"},
		{"spaces around", passages("Trecho 1"), "  Trecho 1  ", "Trecho 1 (2)"},
		{"a name that already ends in (2)", passages("Loja (2)"), "Loja (2)", "Loja (2) (2)"},
	}
	for _, c := range cases {
		if got := project.UniqueName(c.taken, c.base); got != c.want {
			t.Errorf("%s: UniqueName(%q) = %q, want %q", c.name, c.base, got, c.want)
		}
	}
	// Many copies still give a new name each time, always valid.
	var list []project.Passage
	for range 120 {
		name := project.UniqueName(list, "Trecho 0:04–0:09")
		list = append(list, project.Passage{Name: name, Start: 0, End: 1000})
	}
	seen := map[string]bool{}
	for _, passage := range list {
		if seen[strings.ToLower(passage.Name)] {
			t.Fatalf("UniqueName gave %q twice", passage.Name)
		}
		seen[strings.ToLower(passage.Name)] = true
	}
	// A name at the length limit stays inside it with its suffix.
	long := strings.Repeat("á", project.MaxPassageNameLen)
	got := project.UniqueName(passages(long), long)
	if !strings.HasSuffix(got, " (2)") || len([]rune(got)) > project.MaxPassageNameLen {
		t.Fatalf("UniqueName of a %d-letter name = %q (%d letters)", project.MaxPassageNameLen, got, len([]rune(got)))
	}
	if err := (project.Passage{Name: got, Start: 0, End: 1000}).Validate(); err != nil {
		t.Fatalf("the long unique name is not valid: %v", err)
	}
	// Each length near the limit: cut only as much as the suffix needs.
	room := project.MaxPassageNameLen - len([]rune(" (2)"))
	for n := room - 2; n <= project.MaxPassageNameLen; n++ {
		name := strings.Repeat("a", n)
		want := strings.Repeat("a", min(n, room)) + " (2)"
		if got := project.UniqueName(passages(name), name); got != want {
			t.Errorf("UniqueName of %d letters = %d letters %q, want %d", n, len([]rune(got)), got, len([]rune(want)))
		}
	}
	// A cut that ends on a space leaves no double space before the suffix.
	spaced := strings.Repeat("a", room-1) + " bbbb"
	if got, want := project.UniqueName(passages(spaced), spaced), strings.Repeat("a", room-1)+" (2)"; got != want {
		t.Errorf("UniqueName cut on a space = %q, want %q", got, want)
	}
}

func TestIndexOfIgnoresCaseAndSpaces(t *testing.T) {
	list := []project.Passage{{Name: "Loja", End: 1}, {Name: "Rua", End: 1}}
	for name, want := range map[string]int{"Rua": 1, "rua": 1, " LOJA ": 0, "Praça": -1, "": -1} {
		if got := project.IndexOf(list, name); got != want {
			t.Errorf("IndexOf(%q) = %d, want %d", name, got, want)
		}
	}
}

// A passage's plan goes to the file and back unchanged; without a plan it
// pauses at the end with no helps.
func TestAPassagePlanComesBackTheSame(t *testing.T) {
	plans := []project.PassagePlan{
		{},
		{Loop: true, Gap: 2500, Progressive: true, Ladder: true, Goal: 7},
		{Loop: false, Gap: 0, Ladder: true},
	}
	for _, plan := range plans {
		if back := project.PlanOf(plan.Practice()); back != plan {
			t.Errorf("PlanOf(Practice(%+v)) = %+v", plan, back)
		}
	}
	looping := project.PassagePlan{Loop: true, Gap: 1500}.Practice()
	if looping.Mode != "loop" || looping.Gap != 1500*time.Millisecond {
		t.Fatalf("a looping plan = %+v", looping)
	}
	if mode := (project.Passage{Name: "P", End: 1000}).Practice().Mode; mode != "pause" {
		t.Fatalf("a passage without a plan studies with %q, want pause", mode)
	}
}
