package session_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
)

// The karaoke scene: A 1–2 s, then 18 s of silence, B 20–23 s, then C
// 40–42 s. Translations: a 1–2 s, b 20–23 s. With a 10 s gap and a 5 s
// lead, B's preview starts at 15 s.
func karaokeSubtitles(t *testing.T) *session.Subtitles {
	t.Helper()
	primary := timeline(t, cue("A", 1000, 2000), cue("B", 20000, 23000), cue("C", 40000, 42000))
	translation := timeline(t, cue("a", 1000, 2000), cue("b", 20000, 23000))
	subtitles := session.NewSubtitles(&session.Track{Timeline: primary}, &session.Track{Timeline: translation})
	subtitles.SetKaraoke(session.Karaoke{Enabled: true, Rules: study.DefaultCueRules(), Fill: true})
	return subtitles
}

func TestKaraokeShowsTheNextLineWithItsTranslation(t *testing.T) {
	t.Parallel()
	subtitles := karaokeSubtitles(t)
	if got := subtitles.At(ms(14_999)); got.Cue != nil || len(got.Primary) != 0 {
		t.Fatalf("at 14.999 s: %+v, want nothing", got)
	}
	got := subtitles.At(ms(15_000))
	if got.Cue == nil || got.Cue.Event.ID != "B" || ids(got.Primary)[0] != "B" || len(got.Translation) != 1 || got.Translation[0].ID != "b" {
		t.Fatalf("at 15 s: %+v, want B ahead of time with b", got)
	}
	if got.Fill != 0 {
		t.Fatalf("a preview is not filled: %v", got.Fill)
	}
	// Spoken: no preview, a fill that grows.
	early, late := subtitles.At(ms(20_500)), subtitles.At(ms(22_000))
	if early.Cue != nil || !(early.Fill > 0 && early.Fill < late.Fill) {
		t.Fatalf("fill at 20.5 s %v and 22 s %v, want growing", early.Fill, late.Fill)
	}
	// Off: the plain lines only.
	subtitles.SetKaraoke(session.Karaoke{Rules: study.DefaultCueRules(), Fill: true})
	if got := subtitles.At(ms(15_000)); got.Cue != nil || len(got.Primary) != 0 {
		t.Fatalf("karaoke off showed %+v", got)
	}
	if got := subtitles.At(ms(21_000)); got.Fill != 0 {
		t.Fatalf("karaoke off filled %v", got.Fill)
	}
}

func TestKaraokeWithoutFill(t *testing.T) {
	t.Parallel()
	subtitles := karaokeSubtitles(t)
	subtitles.SetKaraoke(session.Karaoke{Enabled: true, Rules: study.DefaultCueRules()})
	if got := subtitles.At(ms(21_000)); got.Fill != 0 {
		t.Fatalf("fill off filled %v", got.Fill)
	}
}

func TestKaraokeWakesUpForThePreview(t *testing.T) {
	t.Parallel()
	subtitles := karaokeSubtitles(t)
	if next, ok := subtitles.NextChangeAfter(ms(3_000)); !ok || next != ms(15_000) {
		t.Fatalf("next change after 3 s = %v %v, want the preview at 15 s", next, ok)
	}
	subtitles.SetKaraoke(session.Karaoke{})
	if next, _ := subtitles.NextChangeAfter(ms(3_000)); next != ms(20_000) {
		t.Fatalf("without karaoke the next change is %v, want 20 s", next)
	}
}

func TestVisibleEqualCountsDrawingSteps(t *testing.T) {
	t.Parallel()
	event := subtitle.Event{ID: "B", Start: ms(20_000), End: ms(23_000), OriginalText: "B"}
	at := func(remaining time.Duration) session.Visible {
		return session.Visible{Primary: []subtitle.Event{event}, Cue: &study.Cue{Event: event, Remaining: remaining, Lead: ms(5000)}}
	}
	if !at(ms(1000)).Equal(at(ms(1000) + ms(39))) {
		t.Error("two times in the same 40 ms step differ")
	}
	if at(ms(1000)).Equal(at(ms(1040))) {
		t.Error("the next 40 ms step looks the same")
	}
	plain := session.Visible{Primary: []subtitle.Event{event}}
	if plain.Equal(at(ms(1000))) || at(ms(1000)).Equal(plain) {
		t.Error("a preview equals a plain line")
	}
	filled := func(f float64) session.Visible {
		return session.Visible{Primary: []subtitle.Event{event}, Fill: f, Filling: true}
	}
	if !filled(0.5).Equal(filled(0.5+0.001)) || filled(0.5).Equal(filled(0.51)) {
		t.Error("fill steps are 1/200")
	}
	if !at(ms(1)).Animating() || !filled(0.3).Animating() || filled(1).Animating() || plain.Animating() {
		t.Error("Animating must hold for previews and lines being filled")
	}
}

// karaokeRunner runs a runner over the karaoke scene and records what it
// shows; the clock starts playing at start.
type karaokeRunner struct {
	time    *fakeTime
	clock   *session.Clock
	samples chan playback.Sample
	shown   chan session.Visible
}

func runKaraoke(t *testing.T, start time.Duration, status playback.Status) *karaokeRunner {
	t.Helper()
	k := &karaokeRunner{time: newFakeTime(), samples: make(chan playback.Sample), shown: make(chan session.Visible, 256)}
	k.clock = session.NewClock(playback.PrecisionExact, k.time.Now)
	k.clock.Observe(playback.Sample{Position: start, Duration: time.Minute, Speed: 1, Status: status, ObservedAt: k.time.Now()})
	subtitles := karaokeSubtitles(t)
	// The sample and the karaoke setup told of changes. Left pending, the
	// runner would wake for them after its first timer and replace it, maybe
	// after the test advanced the time already: the new timer, set from the
	// new time, would never fire.
	for _, changes := range []<-chan struct{}{k.clock.Changes(), subtitles.Changes()} {
		select {
		case <-changes:
		default:
		}
	}
	runner := session.NewRunner(k.clock, subtitles, func(_ time.Duration, v session.Visible) { k.shown <- v }, k.time.NewTimer)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go runner.Run(ctx, k.samples)
	return k
}

func (k *karaokeRunner) next(t *testing.T) session.Visible {
	t.Helper()
	select {
	case v := <-k.shown:
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("nothing shown")
		return session.Visible{}
	}
}

// lastWait is how long the runner's latest timer waits.
func (k *karaokeRunner) lastWait(t *testing.T) time.Duration {
	t.Helper()
	k.time.waitForTimer(t)
	k.time.mu.Lock()
	defer k.time.mu.Unlock()
	last := k.time.timers[len(k.time.timers)-1]
	return last.at.Sub(k.time.now)
}

func TestRunnerAnimatesPreviewsOnlyWhilePlaying(t *testing.T) {
	t.Parallel()
	k := runKaraoke(t, ms(14_000), playback.StatusPlaying)
	if first := k.next(t); first.Cue != nil {
		t.Fatalf("at 14 s: %+v", first)
	}
	// Sleeps until the preview, 1 s away, not 6 s until the line.
	if wait := k.lastWait(t); wait != time.Second {
		t.Fatalf("wait before the preview = %v, want 1 s", wait)
	}
	k.time.Advance(time.Second)
	if v := k.next(t); v.Cue == nil || v.Cue.Remaining != ms(5000) {
		t.Fatalf("at 15 s: %+v, want the preview", v)
	}
	if wait := k.lastWait(t); wait != session.KaraokeFrame {
		t.Fatalf("frame wait = %v, want %v", wait, session.KaraokeFrame)
	}
	k.time.Advance(session.KaraokeFrame)
	if v := k.next(t); v.Cue == nil || v.Cue.Remaining != ms(5000)-session.KaraokeFrame {
		t.Fatalf("one frame later: %+v", v)
	}
}

func TestRunnerDoesNotAnimateWhilePaused(t *testing.T) {
	t.Parallel()
	k := runKaraoke(t, ms(16_000), playback.StatusPaused)
	if v := k.next(t); v.Cue == nil {
		t.Fatalf("paused at 16 s: %+v, want the preview frozen", v)
	}
	if wait := k.lastWait(t); wait != time.Second {
		t.Fatalf("paused wait = %v, want the idle second", wait)
	}
}

func TestCueProgressStaysInRange(t *testing.T) {
	t.Parallel()
	subtitles := karaokeSubtitles(t)
	for at := 15_000; at < 20_000; at += 37 {
		v := subtitles.At(ms(at))
		if v.Cue == nil {
			t.Fatalf("no preview at %d ms", at)
		}
		if p := v.Cue.Progress(); p <= 0 || p > 1 || math.IsNaN(p) {
			t.Fatalf("progress %v at %d ms", p, at)
		}
	}
}

// toldVoice is a voice that tells what a test says: the fill of a line on the
// playback clock and when its voice begins; nil tells nothing.
type toldVoice struct {
	fill   func(start, end, position time.Duration) (float64, bool)
	begins func(start time.Duration) (time.Duration, bool)
}

func (v toldVoice) Fill(_ string, start, end, position time.Duration) (float64, bool) {
	if v.fill == nil {
		return 0, false
	}
	return v.fill(start, end, position)
}

func (v toldVoice) Begins(_ string, start, _ time.Duration) (time.Duration, bool) {
	if v.begins == nil {
		return 0, false
	}
	return v.begins(start)
}

// halfSaid tells every line half said.
var halfSaid = toldVoice{fill: func(time.Duration, time.Duration, time.Duration) (float64, bool) { return 0.5, true }}

// An actor who begins before the line's time ends its preview then: B's
// voice begins 400 ms early, so the bar counts to it, and from it the line is
// said, its fill going, with its translation; a voice not known leaves the
// preview to the line's time.
func TestAPreviewEndsAndItsLineIsSaidAsTheVoiceBegins(t *testing.T) {
	t.Parallel()
	subtitles := karaokeSubtitles(t)
	subtitles.ShiftOffset(session.LinePrimary, 500*time.Millisecond) // B from 20.5 s
	var asked time.Duration
	subtitles.SetVoice(toldVoice{
		fill: func(start, _, position time.Duration) (float64, bool) {
			return float64(position-(start-ms(400))) / float64(ms(1000)), true
		},
		begins: func(start time.Duration) (time.Duration, bool) {
			asked = start
			return start - ms(400), true
		},
	})
	if got := subtitles.At(ms(15_500)); got.Cue == nil || got.Cue.Progress() != 1 || asked != ms(20_500) {
		t.Fatalf("at 15.5 s: %+v (the voice asked for the line at %v), want the whole bar counting to 20.1 s", got.Cue, asked)
	}
	if got := subtitles.At(ms(20_000)); got.Cue == nil || got.Cue.Remaining != ms(100) {
		t.Fatalf("at 20 s: %+v, want 100 ms left until the voice", got.Cue)
	}
	said := subtitles.At(ms(20_350))
	if said.Cue != nil || len(said.Primary) != 1 || said.Primary[0].ID != "B" || !said.Filling || said.Fill != 0.25 {
		t.Fatalf("at 20.35 s, the voice begun: %+v, want B said and a quarter painted", said)
	}
	if len(said.Translation) != 1 || said.Translation[0].ID != "b" {
		t.Fatalf("at 20.35 s: translation %+v, want b with its line", said.Translation)
	}

	unknown := karaokeSubtitles(t)
	unknown.SetVoice(halfSaid)
	if got := unknown.At(ms(19_900)); got.Cue == nil || got.Cue.Remaining != ms(100) {
		t.Fatalf("a voice not known: %+v, want the preview counting to the line's time", got.Cue)
	}
}

// A line is painted from the moment it is said, even while its fill is
// still zero (before the voice begins, or exactly at its start), so the
// screen keeps moving; the voice, when it is followed, decides the fill,
// with the line on the playback clock, and leaves the estimate when it
// cannot tell.
func TestKaraokeFillFollowsTheVoice(t *testing.T) {
	t.Parallel()
	subtitles := karaokeSubtitles(t)
	if start := subtitles.At(ms(20_000)); !start.Filling || start.Fill != 0 || !start.Animating() {
		t.Fatalf("at the line's start: %+v, want filling from zero and animating", start)
	}
	subtitles.ShiftOffset(session.LinePrimary, 500*time.Millisecond)
	var asked []time.Duration
	subtitles.SetVoice(toldVoice{fill: func(start, end, position time.Duration) (float64, bool) {
		asked = []time.Duration{start, end}
		return 0.25, position < ms(22_000)
	}})
	if got := subtitles.At(ms(21_000)); got.Fill != 0.25 || asked[0] != ms(20_500) || asked[1] != ms(23_500) {
		t.Fatalf("voice fill %v for the line at %v, want 0.25 at 20.5-23.5 s", got.Fill, asked)
	}
	if got := subtitles.At(ms(22_500)); got.Fill == 0.25 || got.Fill == 0 {
		t.Fatalf("the estimate did not take over: fill %v", got.Fill)
	}
	described := session.NewSubtitles(&session.Track{Timeline: timeline(t, subtitle.Event{ID: "D", Start: ms(1000), End: ms(3000), OriginalText: "[DOOR SLAMS]"})}, nil)
	described.SetKaraoke(session.Karaoke{Enabled: true, Rules: study.DefaultCueRules(), Fill: true})
	if got := described.At(ms(2_000)); got.Filling || got.Animating() {
		t.Fatalf("a line with nothing said: %+v", got)
	}
}

// A line's fill always ends while it shows (half a line was seen painted
// as it went): a voice still at half when the line goes, the line moved by
// its offset, is caught up and all painted by the line's last pictures,
// with nothing left to draw; a quick line painted by the estimate too.
func TestKaraokeFillAlwaysEndsWhileTheLineShows(t *testing.T) {
	t.Parallel()
	subtitles := karaokeSubtitles(t)
	subtitles.ShiftOffset(session.LinePrimary, 500*time.Millisecond) // B shows from 20.5 s to 23.5 s
	subtitles.SetVoice(halfSaid)
	if got := subtitles.At(ms(22_000)); got.Fill != 0.5 {
		t.Fatalf("well before the line goes: fill %v, want the voice's 0.5", got.Fill)
	}
	if got := subtitles.At(ms(23_400)); got.Fill != 1 || got.Animating() {
		t.Fatalf("the line's last pictures: %+v, want it all painted, nothing left to draw", got)
	}
	quick := session.NewSubtitles(&session.Track{Timeline: timeline(t, subtitle.Event{ID: "Q", Start: ms(1000), End: ms(1300),
		OriginalText: "Well, I really do not think that is what happened here."})}, nil)
	quick.SetKaraoke(session.Karaoke{Enabled: true, Rules: study.DefaultCueRules(), Fill: true})
	// The estimate alone is still on "happened" 100 ms before the line goes.
	if got := quick.At(ms(1_200)); got.Fill != 1 {
		t.Fatalf("a quick line's last pictures: fill %v, want it all painted", got.Fill)
	}
}

// A line the next one overlaps (it comes before the line goes) is all
// painted while both show, and the fill goes on in the next one: a line never
// drops its fill halfway as the next comes. The share counts the visible
// characters, so the drawing paints the first line to its end.
func TestKaraokeFillOfALineTheNextOverlaps(t *testing.T) {
	t.Parallel()
	first := subtitle.Event{ID: "A", Start: ms(1000), End: ms(4000), OriginalText: "This line is first."}
	second := subtitle.Event{ID: "B", Start: ms(3000), End: ms(6000), OriginalText: "And this one next."}
	overlapping := session.NewSubtitles(&session.Track{Timeline: timeline(t, first, second)}, nil)
	overlapping.SetKaraoke(session.Karaoke{Enabled: true, Rules: study.DefaultCueRules(), Fill: true})
	overlapping.SetVoice(halfSaid)
	if got := overlapping.At(ms(2_000)); len(got.Primary) != 1 || got.Fill != 0.5 {
		t.Fatalf("the first line alone: %d lines, fill %v; want 0.5", len(got.Primary), got.Fill)
	}
	got := overlapping.At(ms(3_200))
	// 16 visible characters in the first line, whole, and half of the next's 15.
	if want := (16 + 0.5*15) / 31.0; len(got.Primary) != 2 || !got.Filling || math.Abs(got.Fill-want) > 1e-9 {
		t.Fatalf("both lines: %d lines, filling %v, fill %v; want %v", len(got.Primary), got.Filling, got.Fill, want)
	}
}
