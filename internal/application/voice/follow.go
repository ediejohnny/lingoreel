package voice

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/domain/playback"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
)

// Clock is the playback clock the sound is placed by (session.Clock).
type Clock interface {
	PositionAt(at time.Time) time.Duration
	State() playback.Sample
}

// Settings are the user's choices for following the voice.
type Settings struct {
	Sensitivity speech.Sensitivity
	// Delay is how long after the system takes the sound it is heard: the
	// output's own delay (about 0.2 s on Bluetooth headphones).
	Delay time.Duration
	// EvenWhenBusy keeps following the voice on a computer that cannot keep
	// up, where it would fall back to the estimate.
	EvenWhenBusy bool
}

// Sources are the ways to the film's sound; either may be missing.
type Sources struct {
	// Decoder decodes the voice track ahead (the user's FFmpeg); Voice says
	// which track, its From and Length set as needed.
	Decoder Decoder
	Voice   Stretch
	// Tap hears the player's process PID as it plays.
	Tap Tap
	PID int
}

// Mode is how the voice is followed now.
type Mode int

const (
	// ModeEstimate: the voice cannot be followed; the line is painted by
	// the estimate from its letters.
	ModeEstimate Mode = iota
	// ModeAhead: the voice is decoded ahead of the video.
	ModeAhead
	// ModeLive: the player is heard as it plays.
	ModeLive
)

var modeNames = [...]string{"estimate", "ahead", "live"}

func (m Mode) String() string {
	if m < 0 || int(m) >= len(modeNames) {
		return "unknown"
	}
	return modeNames[m]
}

// Line is a subtitle line on the playback clock.
type Line struct {
	Text       string
	Start, End time.Duration
}

// How far ahead the voice is decoded: a window at a time, the next one when
// less than aheadLow is left, from a moment before the position so the
// first line has its margin. A window decoded slower than twice the speed
// of the video means the computer cannot keep up.
const (
	aheadWindow  = time.Minute
	aheadLow     = 30 * time.Second
	aheadLead    = 2 * time.Second
	aheadPoll    = 250 * time.Millisecond
	aheadSlowest = aheadWindow / 2
	aheadErrors  = 3
	// AheadMemory keeps the window decoded and the sound just played.
	aheadMemory = 3 * time.Minute
)

// How live hearing is judged: a player silent for silentPlayer while it
// plays (muted, or its sound going straight to a receiver) cannot be
// followed; packets later than busyLate for busyFor mean a busy computer.
const (
	silentPlayer = 3 * time.Second
	// jumpTolerance is how far the clock may move between two packets beyond
	// the time between them before it counts as a seek.
	jumpTolerance = time.Second
	staleSound    = 300 * time.Millisecond
	busyLate      = 500 * time.Millisecond
	busyFor       = 2 * time.Second
)

// A film where under leastTrusted of the lines (once triedEnough were
// tried) have a voice that can be trusted is painted by the estimate.
const (
	triedEnough  = 20
	leastTrusted = 0.4
)

// Follower finds the voice of each line and paints it along. It is safe
// for concurrent use: the sound arrives on its own goroutines while the
// lines are drawn on another.
type Follower struct {
	clock    Clock
	settings Settings
	sources  Sources
	memory   *Memory
	cancel   context.CancelFunc
	done     sync.WaitGroup
	seeks    chan struct{}

	mu             sync.Mutex
	mode           Mode
	lines          map[Line]*painting
	live           []*painting // lines painted live, fed every frame
	tried, trusted int
	// Live hearing.
	analyzer    *speech.Analyzer
	listener    *speech.Listener
	frames      []speech.Frame
	sure        bool
	lastSound   time.Time
	silentSince time.Time
	lateSince   time.Time
	// Where the last packet heard while playing went, to notice a seek.
	heardAt       time.Time
	heardPosition time.Duration
}

// painting is how one line is painted: by a plan, live, or by the estimate.
type painting struct {
	plan *study.FillPlan
	live *study.LiveFill
	end  time.Duration
}

func (p *painting) fraction(position time.Duration) (float64, bool) {
	switch {
	case p.plan != nil:
		return p.plan.Fraction(position), true
	case p.live != nil:
		return p.live.Fraction(), true
	}
	return 0, false
}

// Follow starts following the voice with the best source there is.
func Follow(ctx context.Context, sources Sources, clock Clock, settings Settings) *Follower {
	ctx, cancel := context.WithCancel(ctx)
	f := &Follower{
		clock: clock, settings: settings, sources: sources, cancel: cancel,
		seeks: make(chan struct{}, 1), lines: make(map[Line]*painting),
		listener: speech.NewListener(settings.Sensitivity),
	}
	switch {
	case sources.Decoder != nil && sources.Voice.File != "":
		f.mode, f.memory = ModeAhead, NewMemory(aheadMemory)
		f.run(func() { f.decodeAhead(ctx) })
	case sources.Tap != nil && sources.PID > 0:
		f.mode, f.memory = ModeLive, NewMemory(RecentSound)
		f.run(func() { f.hearLive(ctx) })
	default:
		f.memory = NewMemory(RecentSound)
	}
	return f
}

func (f *Follower) run(work func()) {
	f.done.Add(1)
	go func() {
		defer f.done.Done()
		work()
	}()
}

// Close stops following; it returns once the sound stopped coming.
func (f *Follower) Close() {
	f.cancel()
	f.done.Wait()
}

// Mode tells how the voice is followed now.
func (f *Follower) Mode() Mode {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mode
}

func (f *Follower) setMode(mode Mode) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mode = mode
}

// Seeked tells the video jumped: the sound heard before says nothing about
// what comes, and lines painted live are painted anew. A jump while the
// player is heard is noticed without it.
func (f *Follower) Seeked() {
	f.mu.Lock()
	f.forget()
	f.mu.Unlock()
}

// forget drops what a seek makes stale; callers hold the lock.
func (f *Follower) forget() {
	for line, p := range f.lines {
		if p.plan == nil {
			delete(f.lines, line)
		}
	}
	f.live = nil
	f.listener.Reset()
	select {
	case f.seeks <- struct{}{}:
	default:
	}
}

// Fill is the share of a line painted at a position of the playback clock,
// following its voice; false when the line is painted by the estimate.
func (f *Follower) Fill(line Line, position time.Duration) (float64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.lines[line]; ok {
		return p.fraction(position)
	}
	units := study.SpeechUnits(line.Text)
	if f.mode == ModeEstimate || !units.Spoken() || units.Lyrics || f.untrustworthy() {
		return 0, false
	}
	if track, ok := f.memory.Track(line.Start, line.End); ok {
		f.tried++
		p := &painting{}
		if found := speech.DetectLine(track, line.Start, line.End, syllables(units), f.settings.Sensitivity); found.Verdict == speech.Trusted {
			f.trusted++
			plan := study.PlanFill(units, found.Spans)
			p.plan = &plan
		}
		f.lines[line] = p
		return p.fraction(position)
	}
	if f.mode == ModeLive && position < line.End {
		p := &painting{}
		if f.liveReady() {
			p.live, p.end = study.NewLiveFill(units, line.Start, line.End), line.End
			f.live = append(f.live, p)
		}
		f.lines[line] = p
		return p.fraction(position)
	}
	return 0, false // not decoded yet
}

func (f *Follower) untrustworthy() bool {
	return f.tried >= triedEnough && float64(f.trusted) < leastTrusted*float64(f.tried)
}

func syllables(units study.Units) float64 {
	total := 0.0
	for _, word := range units.Words {
		total += word.Weight
	}
	return total
}

// decodeAhead keeps the voice decoded a window ahead of the video.
func (f *Follower) decodeAhead(ctx context.Context) {
	var from, to time.Duration // the run decoded around the position
	failures := 0
	for ctx.Err() == nil {
		position := f.clock.PositionAt(time.Now())
		if position < from || position > to+aheadLead {
			from = max(position-aheadLead, 0)
			to = from
		}
		if to-position >= aheadLow {
			select {
			case <-ctx.Done():
			case <-f.seeks:
			case <-time.After(aheadPoll):
			}
			continue
		}
		stretch := f.sources.Voice
		stretch.From, stretch.Length = to, aheadWindow
		decoded, slow, seeked, err := f.decodeWindow(ctx, stretch)
		switch {
		case ctx.Err() != nil:
			return
		case seeked:
			from, to = -1, -1
			continue
		case err != nil:
			if failures++; failures >= aheadErrors {
				f.fallBack(ctx)
				return
			}
			continue
		case slow && !f.settings.EvenWhenBusy:
			f.fallBack(ctx)
			return
		}
		failures = 0
		if decoded < stretch.Length { // the film ended
			to += decoded
			select {
			case <-ctx.Done():
				return
			case <-f.seeks:
				from, to = -1, -1
			}
			continue
		}
		to += decoded
	}
}

// decodeWindow decodes a stretch into the memory, and tells how much came,
// whether it came too slowly, and whether a seek ended it early.
func (f *Follower) decodeWindow(ctx context.Context, stretch Stretch) (time.Duration, bool, bool, error) {
	window, cancel := context.WithCancel(ctx)
	defer cancel()
	seeked := make(chan struct{})
	go func() {
		select {
		case <-f.seeks:
			close(seeked)
			cancel()
		case <-window.Done():
		}
	}()
	var analyzer *speech.Analyzer
	var frames []speech.Frame
	at := stretch.From
	started := time.Now()
	err := f.sources.Decoder.Decode(window, stretch, func(samples []float32, channels int) {
		if analyzer == nil {
			analyzer, _ = speech.NewAnalyzer(channels)
		}
		frames = analyzer.Write(samples, frames[:0])
		f.memory.Add(at, frames)
		at += time.Duration(len(frames)) * speech.FrameLength
	})
	slow := time.Since(started) > aheadSlowest
	cancel()
	select {
	case <-seeked:
		return at - stretch.From, slow, true, nil
	default:
		return at - stretch.From, slow, false, err
	}
}

// fallBack leaves decoding ahead for hearing the player, or the estimate.
func (f *Follower) fallBack(ctx context.Context) {
	if f.sources.Tap == nil || f.sources.PID <= 0 {
		f.setMode(ModeEstimate)
		return
	}
	f.mu.Lock()
	f.mode = ModeLive
	f.memory = NewMemory(RecentSound)
	f.mu.Unlock()
	f.hearLive(ctx)
}

// hearLive hears the player until ctx ends; if it cannot, the estimate.
func (f *Follower) hearLive(ctx context.Context) {
	if err := f.sources.Tap.Listen(ctx, f.sources.PID, f.hear); err != nil && ctx.Err() == nil {
		f.setMode(ModeEstimate)
	}
}

// hear takes a packet of the player's sound.
func (f *Follower) hear(packet Packet) {
	state := f.clock.State()
	now := time.Now()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastSound = now
	if late := now.Sub(packet.At); late > busyLate {
		if f.lateSince.IsZero() {
			f.lateSince = now
		}
	} else {
		f.lateSince = time.Time{}
	}
	// Paused, the sound is silence that says nothing of the film's.
	if state.Status != playback.StatusPlaying {
		f.heardAt = time.Time{}
		return
	}
	if silent(packet.Samples) {
		if f.silentSince.IsZero() {
			f.silentSince = now
		}
	} else {
		f.silentSince = time.Time{}
	}
	if f.analyzer == nil {
		f.analyzer, _ = speech.NewAnalyzer(packet.Channels)
	}
	f.frames = f.analyzer.Write(packet.Samples, f.frames[:0])
	position := f.clock.PositionAt(packet.At.Add(f.settings.Delay))
	if !f.heardAt.IsZero() {
		expected := f.heardPosition + time.Duration(float64(packet.At.Sub(f.heardAt))*float64(state.Speed))
		if (position - expected).Abs() > jumpTolerance {
			f.forget()
		}
	}
	f.heardAt, f.heardPosition = packet.At, position
	// At another speed a frame is not 10 ms of the film: kept only at 1×.
	if state.Speed == playback.NormalSpeed {
		f.memory.Add(position, f.frames)
	}
	for i, frame := range f.frames {
		voice, sure := f.listener.Hear(frame)
		f.sure = sure
		at := position + time.Duration(float64(time.Duration(i)*speech.FrameLength)*float64(state.Speed))
		for _, p := range f.live {
			p.live.Hear(at, voice)
		}
	}
	// A line over is painted whole; it needs no more sound.
	f.live = slices.DeleteFunc(f.live, func(p *painting) bool { return position > p.end })
}

// liveReady tells whether the player's sound can be followed now; callers
// hold the lock.
func (f *Follower) liveReady() bool {
	now := time.Now()
	switch {
	case !f.sure, now.Sub(f.lastSound) > staleSound:
		return false
	case !f.silentSince.IsZero() && now.Sub(f.silentSince) >= silentPlayer:
		return false
	case !f.lateSince.IsZero() && now.Sub(f.lateSince) >= busyFor && !f.settings.EvenWhenBusy:
		return false
	}
	return true
}

func silent(samples []float32) bool {
	for _, s := range samples {
		if s != 0 {
			return false
		}
	}
	return true
}
