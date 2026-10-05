package watch_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// markingRig is a session that tells its passage marks on the video as
// "start 20s", "no start" and "too short 40s-40.5s".
func markingRig(t *testing.T) (*rig, *watch.Session) {
	t.Helper()
	r := newRig(t)
	r.ports.MarkNotice = func(event watch.MarkEvent, segment study.Segment) string {
		switch event {
		case watch.MarkStarted:
			return fmt.Sprintf("start %v", segment.Start)
		case watch.MarkNoStart:
			return "no start"
		case watch.MarkTooShort:
			return fmt.Sprintf("too short %v-%v", segment.Start, segment.End)
		}
		return "?"
	}
	s := r.start(t)
	r.running.samples <- playback.Sample{Position: 20 * time.Second, Duration: time.Minute, Status: playback.StatusPaused, Speed: 1}
	testkit.Eventually(t, "the first sample", func() bool { return s.State().Duration == time.Minute })
	return r, s
}

// exact turns off the fitting to the lines, for marks checked to the
// millisecond: fitting adds a lead-in before the start.
func exact(t *testing.T, s *watch.Session) {
	t.Helper()
	if err := s.SetPassageOptions(study.PracticePlan{Mode: study.EndPause}, false); err != nil {
		t.Fatal(err)
	}
}

func lastNotice(r *rig) string {
	_, _, notices, _ := r.display.last()
	if len(notices) == 0 {
		return ""
	}
	return notices[len(notices)-1]
}

// Ctrl+F9 marks the start, Ctrl+F10 the end: the passage starts at once and
// the listener hears it, to save it. An end without a start, or too soon
// after it, is told on the video and starts nothing.
func TestMarkingAPassageStudiesItAndTellsTheListener(t *testing.T) {
	t.Parallel()
	r, s := markingRig(t)
	exact(t, s)

	if err := s.Run(shortcut.PassageMarkEnd, false); err != nil {
		t.Fatalf("an end without a start is told, not an error: %v", err)
	}
	if lastNotice(r) != "no start" || s.Passage().Active || len(r.listener.passages()) != 0 {
		t.Fatalf("end without a start: notice %q, active %v, marked %v", lastNotice(r), s.Passage().Active, r.listener.passages())
	}

	at(t, r, s, 20*time.Second)
	if err := s.Run(shortcut.PassageMarkStart, false); err != nil {
		t.Fatal(err)
	}
	if start, set := s.MarkedStart(); !set || start != 20*time.Second || lastNotice(r) != "start 20s" {
		t.Fatalf("after the start mark: %v %v, notice %q", start, set, lastNotice(r))
	}
	if s.Passage().Active {
		t.Fatal("marking the start began a passage")
	}

	at(t, r, s, 35*time.Second)
	if err := s.Run(shortcut.PassageMarkEnd, false); err != nil {
		t.Fatal(err)
	}
	status := s.Passage()
	want := study.Segment{Start: 20 * time.Second, End: 35 * time.Second}
	if !status.Active || status.Segment != want {
		t.Fatalf("after the end mark: %+v, want studying %+v", status, want)
	}
	marked := r.listener.passages()
	if len(marked) != 1 || marked[0].segment != want || marked[0].plan.Mode != study.EndPause {
		t.Fatalf("the listener heard %+v", marked)
	}
	if _, set := s.MarkedStart(); set {
		t.Fatal("the start stayed marked after the passage was closed")
	}
	// The passage starts where it begins and plays its first lap.
	if calls := r.running.took(); !slices.Contains(calls, "seek 20s") || !slices.Contains(calls, "play") {
		t.Fatalf("the player was asked %q", calls)
	}
}

func TestATooShortPassageKeepsItsStart(t *testing.T) {
	t.Parallel()
	r, s := markingRig(t)
	exact(t, s)
	at(t, r, s, 40*time.Second)
	s.MarkPassageStart()
	at(t, r, s, 40*time.Second+500*time.Millisecond)
	_, err := s.MarkPassageEnd()
	if !errors.Is(err, study.ErrInvalidSegment) {
		t.Fatalf("MarkPassageEnd half a second later = %v, want ErrInvalidSegment", err)
	}
	if lastNotice(r) != "too short 40s-40.5s" {
		t.Fatalf("notice %q", lastNotice(r))
	}
	if start, set := s.MarkedStart(); !set || start != 40*time.Second {
		t.Fatalf("the start went away: %v %v", start, set)
	}
	if s.Passage().Active || len(r.listener.passages()) != 0 {
		t.Fatal("a too short passage started or was told")
	}
	// Further on, the same start closes a good passage.
	at(t, r, s, 45*time.Second)
	if segment, err := s.MarkPassageEnd(); err != nil || segment != (study.Segment{Start: 40 * time.Second, End: 45 * time.Second}) {
		t.Fatalf("second try = %+v, %v", segment, err)
	}
}

// Marks may come in either order on the clock: an end before the start
// swaps them.
func TestAnEndBeforeTheStartSwapsThem(t *testing.T) {
	t.Parallel()
	r, s := markingRig(t)
	exact(t, s)
	at(t, r, s, 50*time.Second)
	s.MarkPassageStart()
	at(t, r, s, 44*time.Second)
	segment, err := s.MarkPassageEnd()
	if err != nil || segment != (study.Segment{Start: 44 * time.Second, End: 50 * time.Second}) {
		t.Fatalf("MarkPassageEnd before the start = %+v, %v", segment, err)
	}
}

// By default the marked passage is fitted to the lines, like the panel's
// "Ajustar às falas"; the options last picked (or set) apply to the next.
func TestMarkedPassagesFollowThePassageOptions(t *testing.T) {
	t.Parallel()
	r, s := markingRig(t)
	if plan, snap := s.PassageOptions(); plan.Mode != study.EndPause || !snap {
		t.Fatalf("default options = %+v %v, want pausing at the end, fitted to the lines", plan, snap)
	}
	// The line on top is spoken from 1 s to 3 s: a start inside it moves back
	// to its start, half a second earlier.
	at(t, r, s, 1500*time.Millisecond)
	s.MarkPassageStart()
	at(t, r, s, 10*time.Second)
	if segment, err := s.MarkPassageEnd(); err != nil || segment != (study.Segment{Start: 500 * time.Millisecond, End: 10 * time.Second}) {
		t.Fatalf("fitted passage = %+v, %v", segment, err)
	}

	loop := study.PracticePlan{Mode: study.EndLoop, Gap: time.Second, Goal: 3}
	if err := s.SetPassageOptions(loop, false); err != nil {
		t.Fatal(err)
	}
	at(t, r, s, 1500*time.Millisecond)
	s.MarkPassageStart()
	at(t, r, s, 10*time.Second)
	segment, err := s.MarkPassageEnd()
	if err != nil || segment != (study.Segment{Start: 1500 * time.Millisecond, End: 10 * time.Second}) {
		t.Fatalf("passage without fitting = %+v, %v", segment, err)
	}
	marked := r.listener.passages()
	if last := marked[len(marked)-1]; last.plan != loop || s.Passage().Plan != loop {
		t.Fatalf("plan told %+v, studied %+v, want %+v", last.plan, s.Passage().Plan, loop)
	}
	if err := s.SetPassageOptions(study.PracticePlan{Mode: study.EndLoop, Gap: time.Hour}, true); err == nil {
		t.Fatal("an invalid plan was taken")
	}
	// Studying a passage from the panel makes its options the next ones.
	if err := s.StudyPassage(study.Segment{Start: 30 * time.Second, End: 40 * time.Second}, study.PracticePlan{Mode: study.EndPause, Ladder: true}, true); err != nil {
		t.Fatal(err)
	}
	if plan, snap := s.PassageOptions(); !plan.Ladder || plan.Mode != study.EndPause || !snap {
		t.Fatalf("options after StudyPassage = %+v %v", plan, snap)
	}
}

// The system-wide keys reach the marks; a held Ctrl+F10 closes one passage,
// not one per repeat of the key.
func TestTheGlobalKeysMarkOncePerPress(t *testing.T) {
	t.Parallel()
	r, s := markingRig(t)
	exact(t, s)
	commands := watch.GlobalCommands()
	start, end := slices.Index(commands, shortcut.PassageMarkStart), slices.Index(commands, shortcut.PassageMarkEnd)
	if start < 0 || end < 0 {
		t.Fatalf("the passage marks are not global commands: %v", commands)
	}
	at(t, r, s, 20*time.Second)
	r.shortcuts.presses <- watch.Press{Index: start}
	testkit.Eventually(t, "the start marked by its key", func() bool { _, set := s.MarkedStart(); return set })
	at(t, r, s, 30*time.Second)
	// Held: only repeats arrive after the first press; none of them counts.
	// The keys are handled in order, so a start mark sent after them tells
	// when they were handled.
	r.shortcuts.presses <- watch.Press{Index: end, Repeat: true}
	r.shortcuts.presses <- watch.Press{Index: end, Repeat: true}
	r.shortcuts.presses <- watch.Press{Index: start}
	testkit.Eventually(t, "the keys handled", func() bool { marked, _ := s.MarkedStart(); return marked == 30*time.Second })
	if s.Passage().Active || len(r.listener.passages()) != 0 {
		t.Fatal("a held key's repeats closed a passage")
	}
	at(t, r, s, 40*time.Second)
	r.shortcuts.presses <- watch.Press{Index: end}
	testkit.Eventually(t, "the passage closed by its key", func() bool { return len(r.listener.passages()) == 1 })
}

// A session may open studying a saved passage: the player opens where it
// starts, and the first lap begins with its plan.
func TestARequestedPassageIsStudiedFromTheStart(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	plan := study.PracticePlan{Mode: study.EndLoop, Gap: time.Second, Goal: 3}
	r.request.Passage = &watch.PassageStart{Segment: study.Segment{Start: 10 * time.Second, End: 20 * time.Second}, Plan: plan}
	s := r.start(t)
	if got := r.plugin.requests[0].Start; got != 10*time.Second {
		t.Fatalf("the player opened at %v, want the passage's start", got)
	}
	status := s.Passage()
	if !status.Active || status.Segment != r.request.Passage.Segment || status.Plan != plan {
		t.Fatalf("passage = %+v, want studying %+v with %+v", status, r.request.Passage.Segment, plan)
	}

	// A plan the rules refuse starts nothing.
	bad := newRig(t)
	bad.request.Passage = &watch.PassageStart{Segment: study.Segment{Start: 10 * time.Second, End: 20 * time.Second},
		Plan: study.PracticePlan{Mode: study.EndLoop, Goal: 1000}}
	if _, err := watch.Start(t.Context(), bad.ports, bad.request, bad.listener, nil); err == nil {
		t.Fatal("a session started with an invalid passage plan")
	}
	if len(bad.plugin.requests) != 0 {
		t.Fatal("a player was launched for an invalid passage plan")
	}
}

// The app tells the session the saved passages; the player window's menu
// studies one by its place in that list, as it was kept.
func TestSavedPassagesToldByTheAppAreStudied(t *testing.T) {
	t.Parallel()
	r, s := markingRig(t)
	if len(s.SavedPassages()) != 0 {
		t.Fatal("a new session lists saved passages")
	}
	if err := s.StudySavedAt(0); !errors.Is(err, watch.ErrNoSavedPassage) {
		t.Fatalf("StudySavedAt with none = %v", err)
	}
	loop := study.PracticePlan{Mode: study.EndLoop, Gap: time.Second, Goal: 2}
	list := []watch.SavedPassage{
		{Name: "Loja", Segment: study.Segment{Start: 10 * time.Second, End: 20 * time.Second}, Plan: study.PracticePlan{Mode: study.EndPause}},
		{Name: "Rua", Segment: study.Segment{Start: 30 * time.Second, End: 34 * time.Second}, Plan: loop},
	}
	s.SetSavedPassages(list)
	list[0].Name = "changed after" // the session keeps its own copy
	if got := s.SavedPassages(); len(got) != 2 || got[0].Name != "Loja" {
		t.Fatalf("SavedPassages = %+v", got)
	}
	if err := s.StudySavedAt(1); err != nil {
		t.Fatal(err)
	}
	if status := s.Passage(); !status.Active || status.Segment != list[1].Segment || status.Plan != loop {
		t.Fatalf("studying the second saved passage: %+v", status)
	}
	// Studying a saved passage leaves the mark options as they were.
	if plan, snap := s.PassageOptions(); plan.Mode != study.EndPause || !snap {
		t.Fatalf("options after a saved passage = %+v %v", plan, snap)
	}
	if err := s.StudySavedAt(2); !errors.Is(err, watch.ErrNoSavedPassage) {
		t.Fatalf("StudySavedAt past the list = %v", err)
	}
	_ = r
}

// The Modo Karaokê's timing changes as a whole, refusing what the rules
// refuse.
func TestTheKaraokeSettingsChangeAsAWhole(t *testing.T) {
	t.Parallel()
	_, s := markingRig(t)
	rules := study.CueRules{MinGap: 4 * time.Second, Lead: 3 * time.Second}
	if err := s.SetKaraokeSettings(session.Karaoke{Enabled: true, Rules: rules, Fill: false}); err != nil {
		t.Fatal(err)
	}
	if got := s.Karaoke(); !got.Enabled || got.Rules != rules || got.Fill {
		t.Fatalf("karaoke = %+v", got)
	}
	if err := s.SetKaraokeSettings(session.Karaoke{Enabled: true, Rules: study.CueRules{MinGap: time.Millisecond, Lead: time.Second}}); err == nil {
		t.Fatal("a gap the rules refuse was taken")
	}
	if got := s.Karaoke(); got.Rules != rules {
		t.Fatalf("a refused change changed the karaoke: %+v", got)
	}
	if !s.HasPrimary() {
		t.Fatal("the session has a line on top")
	}
}

// Correcting a line from the player window pauses, picks the line on screen
// and brings the app's panel up.
func TestOpenEditorPausesPicksAndShowsThePanel(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	shown := make(chan struct{}, 1)
	r.ports.ShowPanel = func() { shown <- struct{}{} }
	s := r.start(t)
	r.running.samples <- playback.Sample{Position: 1500 * time.Millisecond, Duration: time.Minute, Status: playback.StatusPlaying, Speed: 1, ObservedAt: time.Now()}
	testkit.Eventually(t, "the line on screen", func() bool { screen, _, _, _ := r.display.last(); return screen.Primary == "Hello there." })
	if err := s.OpenEditor(session.LinePrimary); err != nil {
		t.Fatalf("OpenEditor: %v", err)
	}
	if selection, ok := s.Selection(); !ok || selection.Line != session.LinePrimary {
		t.Fatalf("selection = %+v %v", selection, ok)
	}
	if s.State().Status == playback.StatusPlaying {
		t.Fatal("the video still plays while a line is corrected")
	}
	select {
	case <-shown:
	case <-time.After(time.Second):
		t.Fatal("the app's panel was not brought up")
	}
	// No line on screen: nothing to correct, and the panel stays where it is.
	if err := s.Controls().SeekTo(40 * time.Second); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenEditor(session.LinePrimary); err == nil {
		t.Fatal("OpenEditor with no line on screen succeeded")
	}
	select {
	case <-shown:
		t.Fatal("the panel came up with nothing to correct")
	default:
	}
}

// Fitted to its lines, a passage can reach past the end of a short video:
// it is cut at the end and studied, not refused.
func TestAMarkedPassageIsCutAtTheEndOfTheVideo(t *testing.T) {
	t.Parallel()
	r, s := markingRig(t)
	sample := func(position time.Duration) {
		r.running.samples <- playback.Sample{Position: position, Duration: 2500 * time.Millisecond, Status: playback.StatusPaused, Speed: 1, ObservedAt: time.Now()}
		testkit.Eventually(t, "the position", func() bool { return s.State().Position == position })
	}
	sample(1200 * time.Millisecond)
	s.MarkPassageStart()
	sample(2 * time.Second)
	segment, err := s.MarkPassageEnd()
	// The line runs 1 s to 3 s: half a second of lead-in before it, and its
	// end cut at the video's 2.5 s.
	want := study.Segment{Start: 500 * time.Millisecond, End: 2500 * time.Millisecond}
	if err != nil || segment != want {
		t.Fatalf("MarkPassageEnd = %+v, %v; want %+v", segment, err, want)
	}
	if status := s.Passage(); !status.Active || status.Segment != want {
		t.Fatalf("studying %+v", status)
	}
}

// A saved passage from a longer cut of the film is studied up to the end of
// this video.
func TestASavedPassagePastTheEndIsCut(t *testing.T) {
	t.Parallel()
	_, s := markingRig(t)
	s.SetSavedPassages([]watch.SavedPassage{
		{Name: "Final", Segment: study.Segment{Start: 50 * time.Second, End: 70 * time.Second}, Plan: study.PracticePlan{Mode: study.EndPause}},
	})
	if err := s.StudySavedAt(0); err != nil {
		t.Fatal(err)
	}
	if status := s.Passage(); !status.Active || status.Segment != (study.Segment{Start: 50 * time.Second, End: time.Minute}) {
		t.Fatalf("studying %+v", status)
	}
}

// Without a line on top there is nothing to fit a passage to or to preview.
func TestASessionWithoutTheLineOnTop(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.request.Tracks.Primary = 0
	s := r.start(t)
	if s.HasPrimary() {
		t.Fatal("a session without a line on top says it has one")
	}
}
