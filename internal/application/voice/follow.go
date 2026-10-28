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
	// Seeks counts the jumps of the position; a change means what was
	// heard before says nothing of what comes.
	Seeks() uint64
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

// Sources are the ways to the film's sound, and the model that tells the
// voice in it; any may be missing.
type Sources struct {
	// Model tells speech from the rest of the sound; without it nothing is
	// decoded or heard, and every line keeps the estimate.
	Model speech.Model
	// Decoder decodes the voice track ahead (the user's FFmpeg); Voice says
	// which track, its From and Length set as needed.
	Decoder Decoder
	Voice   Stretch
	// Tap hears the player's process PID as it plays.
	Tap Tap
	PID int
	// Words places each line's words in the sound decoded ahead (the line's
	// language's speech pack), and Lines lists the lines shown between two
	// moments of the playback clock, so they are placed before they show.
	// Without both, or without the Decoder, lines are painted by the voice
	// found. The follower closes Words when it closes.
	Words WordModel
	Lines func(from, to time.Duration) []Line
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
	// Each window is decoded from aheadWarmUp before it, so the model has
	// heard what leads into its first line, and aheadTail past it, so its
	// last frames are judged on the sound after them; only the window's own
	// frames are kept.
	aheadWarmUp = time.Second
	aheadTail   = 100 * time.Millisecond
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

	// sound is the decoded sound kept while words are placed in it, which
	// keepSound tells (set before anything runs).
	sound     recording
	keepSound bool

	mu             sync.Mutex
	mode           Mode
	lines          map[Line]*painting
	live           []*painting // lines painted live, fed every frame
	tried, trusted int
	// placed holds each line's words as placed in the sound, nil when no
	// words are placed.
	placed map[Line][]speech.Word
	// seeks is the clock's count of jumps already taken into account.
	seeks uint64
	// Live hearing.
	listener    *speech.Listener
	sure        bool
	lastSound   time.Time
	silentSince time.Time
	lateSince   time.Time
	// Where the last packet heard while playing went, to notice a seek.
	heardAt       time.Time
	heardPosition time.Duration
}

// painting is how one line is painted: by a plan, live, or by the estimate;
// byWords tells a plan from the line's words placed.
type painting struct {
	plan    *study.FillPlan
	live    *study.LiveFill
	end     time.Duration
	byWords bool
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
		lines: make(map[Line]*painting), seeks: clock.Seeks(),
		listener: speech.NewListener(settings.Sensitivity),
	}
	switch {
	case sources.Model == nil:
		f.memory = NewMemory(RecentSound)
	case sources.Decoder != nil && sources.Voice.File != "":
		f.mode, f.memory = ModeAhead, NewMemory(aheadMemory)
		if sources.Words != nil && sources.Lines != nil {
			f.placed, f.keepSound = map[Line][]speech.Word{}, true
			f.done.Go(func() { f.placeWords(ctx) })
		}
		f.done.Go(func() { f.decodeAhead(ctx) })
	case sources.Tap != nil && sources.PID > 0:
		f.mode, f.memory = ModeLive, NewMemory(RecentSound)
		f.done.Go(func() { f.hearLive(ctx) })
	default:
		f.memory = NewMemory(RecentSound)
	}
	return f
}

// Close stops following; it returns once the sound stopped coming and the
// words model is closed.
func (f *Follower) Close() {
	f.cancel()
	f.done.Wait()
	if f.sources.Words != nil {
		f.sources.Words.Close()
	}
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

// noticeSeeks forgets what a jump of the clock made stale, paused or
// playing; callers hold the lock.
func (f *Follower) noticeSeeks() {
	if seeks := f.clock.Seeks(); seeks != f.seeks {
		f.seeks = seeks
		f.forget()
	}
}

// forget drops what a seek makes stale: lines painted live are painted anew
// (a plan from the sound kept stays); callers hold the lock.
func (f *Follower) forget() {
	for line, p := range f.lines {
		if p.plan == nil {
			delete(f.lines, line)
		}
	}
	f.live = nil
	f.listener.Reset()
	f.heardAt = time.Time{}
}

// Fill is the share of a line painted at a position of the playback clock,
// following its voice; false when the line is painted by the estimate.
func (f *Follower) Fill(line Line, position time.Duration) (float64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.noticeSeeks()
	if words, ok := f.placed[line]; ok {
		if p := f.lines[line]; p == nil || !p.byWords {
			plan := study.PlanWords(study.SpeechUnits(line.Text), words)
			f.lines[line] = &painting{plan: &plan, byWords: true}
		}
		return f.lines[line].fraction(position)
	}
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

// Begins is when a line's voice begins, its first word placed in the sound;
// false when its words were not placed (yet, or none was heard).
func (f *Follower) Begins(line Line) (time.Duration, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, word := range f.placed[line] {
		if word.Heard {
			return word.Start, true
		}
	}
	return 0, false
}

// Heard is the voice heard around a moment of the film, up to before it and
// after it, as Memory.Around tells it; false when none was heard there.
func (f *Follower) Heard(at, before, after time.Duration) (speech.Track, bool) {
	return f.memory.Around(at, before, after)
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
		if !kept(from, to, to, position) {
			from = max(position-aheadLead, 0)
			to = from
		}
		if to-position >= aheadLow {
			if !wait(ctx, aheadPoll) {
				return
			}
			continue
		}
		stretch := f.sources.Voice
		stretch.From, stretch.Length = to, aheadWindow
		decoded, slow, moved, err := f.decodeWindow(ctx, stretch, func(position time.Duration) bool {
			return kept(from, to, stretch.From+stretch.Length, position)
		})
		switch {
		case ctx.Err() != nil:
			return
		case moved:
			continue // the next turn starts a run where the video went
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
		to += decoded
		// At the film's end there is nothing more to decode until the video
		// goes somewhere not kept.
		for decoded < stretch.Length && kept(from, to, to, f.clock.PositionAt(time.Now())) {
			if !wait(ctx, aheadPoll) {
				return
			}
		}
	}
}

// kept tells whether a position is in the run decoded from from to to, or
// in the stretch being decoded up to end, with the sound still in memory.
func kept(from, to, end, position time.Duration) bool {
	return position >= max(from, to-aheadMemory) && position <= end+aheadLead
}

// wait sleeps for a while, or less if ctx ends; it tells whether ctx goes on.
func wait(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// decodeWindow decodes a stretch into the memory, and tells how much came,
// whether it came too slowly, and whether the video went somewhere else
// meanwhile (keep false), which ends it early.
func (f *Follower) decodeWindow(ctx context.Context, stretch Stretch, keep func(time.Duration) bool) (time.Duration, bool, bool, error) {
	stream, err := f.sources.Model.NewStream()
	if err != nil {
		return 0, false, false, err
	}
	window, cancel := context.WithCancel(ctx)
	defer cancel()
	seeked, watched := make(chan struct{}), make(chan struct{})
	defer func() { // keep reads what the caller changes next
		cancel()
		<-watched
	}()
	go func() {
		defer close(watched)
		ticker := time.NewTicker(aheadPoll)
		defer ticker.Stop()
		for {
			select {
			case <-window.Done():
				return
			case <-ticker.C:
				if !keep(f.clock.PositionAt(time.Now())) {
					close(seeked)
					cancel()
					return
				}
			}
		}
	}()
	decode := stretch
	decode.From = max(stretch.From-aheadWarmUp, 0)
	decode.Length = stretch.From + stretch.Length + aheadTail - decode.From
	end := stretch.From + stretch.Length
	var analyzer *speech.Analyzer
	var frames []speech.Frame
	at, kept := decode.From, stretch.From // the next frame's time; how far the window's frames reach
	store := func() {
		first := min(framesUntil(stretch.From-at), len(frames))
		last := max(min(framesUntil(end-at), len(frames)), first)
		if last > first {
			f.memory.Add(at+time.Duration(first)*speech.FrameLength, frames[first:last])
			kept = at + time.Duration(last)*speech.FrameLength
		}
		at += time.Duration(len(frames)) * speech.FrameLength
	}
	paced := pace{duty: aheadDuty}
	soundAt, mono := decode.From, []float32(nil) // where the next samples go, for placing words
	started := time.Now()
	err = f.sources.Decoder.Decode(window, decode, func(samples []float32, channels int) {
		began := time.Now()
		if f.keepSound {
			mono = appendMono(mono[:0], samples, channels)
			f.sound.add(soundAt, mono)
			soundAt += time.Duration(len(mono)) * time.Second / speech.SampleRate
		}
		if analyzer == nil {
			// The decoder hands mono or stereo; a sound of another layout
			// stays unjudged (the estimate goes on) rather than guessed at.
			analyzer, _ = speech.NewAnalyzer(channels, stream)
		}
		if analyzer != nil {
			frames = analyzer.Write(samples, frames[:0])
			store()
		}
		paced.worked(window, time.Since(began))
	})
	if err == nil && analyzer != nil {
		frames = analyzer.Flush(frames[:0])
		store()
	}
	// A whole decode that stops before the window's end, though asked for
	// aheadTail more, reached the end of the voice track.
	if err == nil && window.Err() == nil && f.keepSound && soundAt < end {
		f.sound.finish(soundAt)
	}
	slow := time.Since(started) > aheadSlowest
	cancel()
	select {
	case <-seeked:
		return kept - stretch.From, slow, true, nil
	default:
		return kept - stretch.From, slow, false, err
	}
}

// framesUntil is how many frames start before a time from now: none for a
// time already past.
func framesUntil(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int((d + speech.FrameLength - 1) / speech.FrameLength)
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
	stream, err := f.sources.Model.NewStream()
	if err != nil {
		f.setMode(ModeEstimate)
		return
	}
	sound := &liveSound{stream: stream}
	hear := func(packet Packet) { f.hear(sound, packet) }
	if err := f.sources.Tap.Listen(ctx, f.sources.PID, hear); err != nil && ctx.Err() == nil {
		f.setMode(ModeEstimate)
	}
}

// liveSound is what hearing the player keeps from one packet to the next;
// only the tap's goroutine touches it.
type liveSound struct {
	stream   speech.Stream
	analyzer *speech.Analyzer
	frames   []speech.Frame
}

// hear takes a packet of the player's sound. The model judges it before the
// lock is taken, so drawing a line never waits for the model; the system
// keeps the sound coming meanwhile.
func (f *Follower) hear(sound *liveSound, packet Packet) {
	state := f.clock.State()
	// Paused, the sound is silence that says nothing of the film's.
	playing := state.Status == playback.StatusPlaying
	sound.frames = sound.frames[:0]
	if playing {
		if sound.analyzer == nil {
			// The tap asks Windows for stereo; a sound of another layout stays
			// unjudged (the estimate goes on) rather than guessed at.
			sound.analyzer, _ = speech.NewAnalyzer(packet.Channels, sound.stream)
		}
		if sound.analyzer != nil {
			sound.frames = sound.analyzer.Write(packet.Samples, sound.frames)
		}
	}
	now := time.Now() // after the model: how late the packet is tells whether the computer keeps up
	f.mu.Lock()
	defer f.mu.Unlock()
	f.noticeSeeks()
	f.lastSound = now
	if late := now.Sub(packet.At); late > busyLate {
		if f.lateSince.IsZero() {
			f.lateSince = now
		}
	} else {
		f.lateSince = time.Time{}
	}
	if !playing {
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
	if sound.analyzer == nil {
		return // neither mono nor stereo
	}
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
		f.memory.Add(position, sound.frames)
	}
	for i, frame := range sound.frames {
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
