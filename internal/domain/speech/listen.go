package speech

import "slices"

// While the sound is only heard as it plays there is no margin ahead to
// judge a line by: the Listener judges each frame against the last
// listenWindow instead, with the same quiet, loudest and threshold as
// DetectLine, worked out again every relistenEvery. The loudest fades only
// slowly: a line after a long silence (the karaoke mode's preview) is
// measured against the voice heard before it.
const (
	listenWindow  = 400 // frames: 4 s
	relistenEvery = 10  // frames: 100 ms
	listenWarmUp  = 100 // frames heard before the Listener can be sure
	peakFall      = 0.5 // dB a second
)

// Listener tells, frame by frame, whether someone is speaking.
type Listener struct {
	share, contrast float64
	levels          [2][listenWindow]float64
	heard           int
	peak, threshold [2]float64
	hasPeak         bool
	usable          [2]bool
	on              [2]bool
	scratch         []float64
}

// NewListener hears with a sensitivity.
func NewListener(sensitivity Sensitivity) *Listener {
	share, contrast := sensitivity.tuning()
	return &Listener{share: share, contrast: contrast, scratch: make([]float64, listenWindow)}
}

// Reset forgets the sound heard, which says nothing about what comes after
// a seek, but not how loud the voice was: the film keeps its loudness.
func (l *Listener) Reset() {
	*l = Listener{share: l.share, contrast: l.contrast, scratch: l.scratch, peak: l.peak, hasPeak: l.hasPeak}
}

// Hear takes the next frame. Voice is whether someone speaks in it; sure,
// whether the sound around told voice from the rest at all (when not, the
// voice cannot be followed and the estimate stays).
func (l *Listener) Hear(frame Frame) (voice, sure bool) {
	at := l.heard % listenWindow
	l.levels[0][at], l.levels[1][at] = voiceLevel(frame), hissLevel(frame)
	l.heard++
	if l.heard%relistenEvery == 0 {
		l.relisten()
	}
	if l.heard < listenWarmUp {
		return false, false
	}
	for band := range l.levels {
		level := l.levels[band][at]
		l.on[band] = l.usable[band] && (level >= l.threshold[band] || (l.on[band] && level >= l.threshold[band]-hysteresis))
		voice = voice || l.on[band]
		sure = sure || l.usable[band]
	}
	return voice, sure
}

func (l *Listener) relisten() {
	n := min(l.heard, listenWindow)
	for band := range l.levels {
		sorted := l.scratch[:n]
		copy(sorted, l.levels[band][:n])
		slices.Sort(sorted)
		floor := sorted[int(floorPercentile*float64(n-1))]
		peak := sorted[int(peakPercentile*float64(n-1))]
		if l.hasPeak {
			peak = max(peak, l.peak[band]-peakFall*relistenEvery*FrameLength.Seconds())
		}
		l.peak[band] = peak
		l.usable[band] = peak-floor >= l.contrast
		l.threshold[band] = floor + l.share*(peak-floor)
	}
	l.hasPeak = true
}
