package session_test

import (
	"context"
	"math"
	"testing"
	"time"

	"lingoreel/internal/application/session"
	"lingoreel/internal/domain/playback"
	"lingoreel/internal/domain/study"
	"lingoreel/internal/domain/subtitle"
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
	subtitles := karaokeSubtitles(t)
	subtitles.SetKaraoke(session.Karaoke{Enabled: true, Rules: study.DefaultCueRules()})
	if got := subtitles.At(ms(21_000)); got.Fill != 0 {
		t.Fatalf("fill off filled %v", got.Fill)
	}
}

func TestKaraokeWakesUpForThePreview(t *testing.T) {
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
	filled := func(f float64) session.Visible { return session.Visible{Primary: []subtitle.Event{event}, Fill: f} }
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
	runner := session.NewRunner(k.clock, karaokeSubtitles(t), func(_ time.Duration, v session.Visible) { k.shown <- v }, k.time.NewTimer)
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
	k := runKaraoke(t, ms(16_000), playback.StatusPaused)
	if v := k.next(t); v.Cue == nil {
		t.Fatalf("paused at 16 s: %+v, want the preview frozen", v)
	}
	if wait := k.lastWait(t); wait != time.Second {
		t.Fatalf("paused wait = %v, want the idle second", wait)
	}
}

func TestCueProgressStaysInRange(t *testing.T) {
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
