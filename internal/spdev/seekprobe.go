package spdev

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/discovery"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/plugins"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/vlc"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/audiotap"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/playerwindow"
)

// The seek probe measures how a passage's lap starts in VLC: from the order
// sent to the first picture of the passage on the screen and to its first
// sound, lap after lap, and whether the two still go together when the
// white and beeping part ends. It needs the probe clip, whose picture is
// white and whose sound is a beep for 2 s, then black and silent for 2 s
// (spdev help prints the FFmpeg command that makes it); the passage starts
// in the white and beeping part, and each lap starts paused in the black and
// silent one, as after a passage's gap.

var (
	procGetDC     = windows.NewLazySystemDLL("user32.dll").NewProc("GetDC")
	procReleaseDC = windows.NewLazySystemDLL("user32.dll").NewProc("ReleaseDC")
	procGetPixel  = windows.NewLazySystemDLL("gdi32.dll").NewProc("GetPixel")
)

// The probe's thresholds: a picture is white when each channel is above
// whiteLevel and black when each is below darkLevel, and a packet has the
// beep when its loudness (RMS) is above loudLevel; the beep is about 0.09.
const (
	whiteLevel = 200
	darkLevel  = 55
	loudLevel  = 0.02
	// probeWait bounds the wait for a lap's picture and sound.
	probeWait = 4 * time.Second
	// probeSettle is how long the paused video rests before each lap.
	probeSettle = 1500 * time.Millisecond
)

// lapTimes is one lap, each a delay from the play order; negative when it
// never came: its picture and its sound, and the end of the white picture
// and of the beep. ready, with --mode prepare, is how long the picture took
// to show after the order that made it ready.
type lapTimes struct{ ready, picture, sound, pictureEnd, soundEnd time.Duration }

func seekProbe(ctx context.Context, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("seek-probe", flag.ContinueOnError)
	flags.SetOutput(stdout)
	start := flags.Duration("start", 61*time.Second, "where the passage starts (white and beeping)")
	rest := flags.Duration("rest", 63*time.Second, "where each lap waits, paused (black and silent)")
	laps := flags.Int("laps", 20, "how many laps")
	mode := flags.String("mode", "together", "how a lap is sent: together (one order), apart (a seek, then a play) or prepare (the picture made ready during the gap, then a play)")
	mask := flags.Uint64("affinity", 0, "holds VLC to these processors (0x3: the first two), as a smaller computer")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("seek-probe: give the probe clip")
	}
	if *mode != "together" && *mode != "apart" && *mode != "prepare" {
		return fmt.Errorf("seek-probe: --mode is together, apart or prepare, not %q", *mode)
	}
	clip, err := filepath.Abs(flags.Arg(0))
	if err != nil {
		return err
	}
	registry, hints, err := plugins.Registry(stderrLogger())
	if err != nil {
		return err
	}
	choice, found := player.Choose(discovery.Detect(discovery.SystemEnvironment{}, hints, nil), registry.Supported, player.VLC)
	if !found || choice.Installation.ID != player.VLC {
		return errors.New("seek-probe: needs VLC installed")
	}

	// The window must be on the screen and uncovered: the picture is read
	// from the screen at its center.
	placement := player.Placement{X: 80, Y: 80, Width: 640, Height: 360}
	host, err := playerwindow.Open(ctx, playerwindow.Options{Title: "seek-probe", Placement: placement, NoActivate: true})
	if err != nil {
		return err
	}
	defer host.Close()
	session, err := vlc.Launch(ctx, choice.Installation.Path, player.LaunchRequest{
		File: clip, AudioIndex: -1, StartPaused: true, Start: *rest, Host: host.Handle(),
	})
	if err != nil {
		return err
	}
	defer session.Close()
	if *mask != 0 {
		process, err := windows.OpenProcess(windows.PROCESS_SET_INFORMATION, false, uint32(session.ProcessID()))
		if err != nil {
			return fmt.Errorf("seek-probe: reaching VLC: %w", err)
		}
		err = pin(process, *mask)
		windows.CloseHandle(process)
		if err != nil {
			return err
		}
	}

	// loud is the first packet with the beep after since, and quiet the
	// first one without it after loud.
	var (
		mu                 sync.Mutex
		loud, quiet, since time.Time
	)
	listening, stopListening := context.WithCancel(ctx)
	defer stopListening()
	go func() {
		_ = audiotap.Tap{}.Listen(listening, session.ProcessID(), func(packet voice.Packet) {
			if len(packet.Samples) == 0 {
				return
			}
			sum := 0.0
			for _, sample := range packet.Samples {
				sum += float64(sample) * float64(sample)
			}
			beeping := math.Sqrt(sum/float64(len(packet.Samples))) >= loudLevel
			mu.Lock()
			defer mu.Unlock()
			switch {
			case since.IsZero() || !packet.At.After(since):
			case beeping && loud.IsZero():
				loud = packet.At
			case !beeping && !loud.IsZero() && quiet.IsZero():
				quiet = packet.At
			}
		})
	}()
	heard := func(at *time.Time, sent time.Time) time.Duration {
		for deadline := sent.Add(probeWait); time.Now().Before(deadline) && ctx.Err() == nil; time.Sleep(10 * time.Millisecond) {
			mu.Lock()
			came := *at
			mu.Unlock()
			if !came.IsZero() {
				return came.Sub(sent)
			}
		}
		return -1
	}
	pause := func(d time.Duration) {
		select {
		case <-ctx.Done():
		case <-time.After(d):
		}
	}

	out := &printer{w: stdout}
	out.line("Probing %d laps of %s, sent %s, the passage from %v, resting at %v.", *laps, filepath.Base(clip), *mode, *start, *rest)
	x, y := int32(placement.X+placement.Width/2), int32(placement.Y+placement.Height/2)
	var results []lapTimes
	for lap := 1; lap <= *laps && ctx.Err() == nil; lap++ {
		// Paused, a seek shows no picture: one frame forward shows the
		// black part, or the last lap's white would still be on the screen.
		if err := errors.Join(session.Pause(), session.SeekTo(*rest), session.StepFrame(true)); err != nil {
			return err
		}
		if waitColor(ctx, x, y, time.Now(), false) < 0 {
			return errors.New("seek-probe: the black part never showed; is the window uncovered?")
		}
		pause(probeSettle)
		times := lapTimes{ready: -1, pictureEnd: -1, soundEnd: -1}
		if *mode == "prepare" {
			asked := time.Now()
			if err := session.Prepare(*start); err != nil {
				return err
			}
			times.ready = waitColor(ctx, x, y, asked, true)
			pause(probeSettle)
		}
		mu.Lock()
		loud, quiet, since = time.Time{}, time.Time{}, time.Now()
		sent := since
		mu.Unlock()
		switch *mode {
		case "together":
			err = session.SeekAndPlay(*start)
		case "apart":
			err = errors.Join(session.SeekTo(*start), session.Play())
		default:
			err = session.Play()
		}
		if err != nil {
			return err
		}
		times.picture = waitColor(ctx, x, y, sent, true)
		times.sound = heard(&loud, sent)
		if times.picture >= 0 {
			times.pictureEnd = waitColor(ctx, x, y, sent, false)
		}
		if times.sound >= 0 {
			times.soundEnd = heard(&quiet, sent)
		}
		results = append(results, times)
		line := fmt.Sprintf("lap %2d  picture %s  sound %s  picture - sound %s  at the end %s",
			lap, millis(times.picture), millis(times.sound), apart(times.picture, times.sound), apart(times.pictureEnd, times.soundEnd))
		if *mode == "prepare" {
			line += "  ready " + millis(times.ready)
		}
		out.line("%s", line)
	}
	summarize(out, results)
	return out.err
}

// waitColor polls the screen at (x, y) until it turns white (or black, when
// white is false), and tells how long after sent; -1 when it did not within
// probeWait.
func waitColor(ctx context.Context, x, y int32, sent time.Time, white bool) time.Duration {
	screen, _, _ := procGetDC.Call(0)
	if screen == 0 {
		return -1
	}
	defer procReleaseDC.Call(0, screen)
	for time.Since(sent) < probeWait && ctx.Err() == nil {
		color, _, _ := procGetPixel.Call(screen, uintptr(x), uintptr(y))
		red, green, blue := color&0xFF, (color>>8)&0xFF, (color>>16)&0xFF
		switch {
		case color == 0xFFFFFFFF: // CLR_INVALID: the point is off the screen
		case white && red > whiteLevel && green > whiteLevel && blue > whiteLevel,
			!white && red < darkLevel && green < darkLevel && blue < darkLevel:
			return time.Since(sent)
		}
		time.Sleep(2 * time.Millisecond)
	}
	return -1
}

func millis(d time.Duration) string {
	if d < 0 {
		return "   none"
	}
	return fmt.Sprintf("%4d ms", d.Milliseconds())
}

func apart(picture, sound time.Duration) string {
	if picture < 0 || sound < 0 {
		return "   none"
	}
	return fmt.Sprintf("%+5d ms", (picture - sound).Milliseconds())
}

// summarize prints the median and the 95th percentile of each delay, and of
// the distance between picture and sound at the start and at the end.
func summarize(out *printer, results []lapTimes) {
	var readies, pictures, sounds, gaps, endGaps []time.Duration
	for _, r := range results {
		if r.ready >= 0 {
			readies = append(readies, r.ready)
		}
		if r.picture >= 0 {
			pictures = append(pictures, r.picture)
		}
		if r.sound >= 0 {
			sounds = append(sounds, r.sound)
		}
		if r.picture >= 0 && r.sound >= 0 {
			gaps = append(gaps, (r.picture - r.sound).Abs())
		}
		if r.pictureEnd >= 0 && r.soundEnd >= 0 {
			endGaps = append(endGaps, (r.pictureEnd - r.soundEnd).Abs())
		}
	}
	rows := []struct {
		name   string
		values []time.Duration
	}{{"picture", pictures}, {"sound", sounds}, {"|picture - sound|", gaps}, {"at the end", endGaps}}
	if len(readies) > 0 {
		rows = append(rows, struct {
			name   string
			values []time.Duration
		}{"ready", readies})
	}
	for _, row := range rows {
		if len(row.values) == 0 {
			out.line("%-18s none came", row.name)
			continue
		}
		slices.Sort(row.values)
		out.line("%-18s median %4d ms  p95 %4d ms  (%d laps)", row.name,
			row.values[len(row.values)/2].Milliseconds(), row.values[min(len(row.values)*95/100, len(row.values)-1)].Milliseconds(), len(row.values))
	}
}
