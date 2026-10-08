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
// sound, lap after lap. It needs the probe clip, whose picture is white and
// whose sound is a beep for 2 s, then black and silent for 2 s (spdev help
// prints the FFmpeg command that makes it); the passage starts in the white
// and beeping part, and each lap starts paused in the black and silent one,
// as after a passage's gap.

var (
	procGetDC     = windows.NewLazySystemDLL("user32.dll").NewProc("GetDC")
	procReleaseDC = windows.NewLazySystemDLL("user32.dll").NewProc("ReleaseDC")
	procGetPixel  = windows.NewLazySystemDLL("gdi32.dll").NewProc("GetPixel")
)

// The probe's thresholds: a picture is white when each channel is above
// whiteLevel, and a packet has the beep when its loudness (RMS) is above
// loudLevel; the beep is about 0.09.
const (
	whiteLevel = 200
	loudLevel  = 0.02
	// probeWait bounds the wait for a lap's picture and sound.
	probeWait = 4 * time.Second
	// probeSettle is how long the paused video rests before each lap.
	probeSettle = 1500 * time.Millisecond
)

// lapTimes is one lap: the delay from the order to its picture and to its
// sound; negative when it never came.
type lapTimes struct{ picture, sound time.Duration }

func seekProbe(ctx context.Context, args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("seek-probe", flag.ContinueOnError)
	flags.SetOutput(stdout)
	start := flags.Duration("start", 61*time.Second, "where the passage starts (white and beeping)")
	rest := flags.Duration("rest", 63*time.Second, "where each lap waits, paused (black and silent)")
	laps := flags.Int("laps", 20, "how many laps")
	mode := flags.String("mode", "together", "how a lap is sent: together (one order) or apart (a seek, then a play)")
	mask := flags.Uint64("affinity", 0, "holds VLC to these processors (0x3: the first two), as a smaller computer")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("seek-probe: give the probe clip")
	}
	if *mode != "together" && *mode != "apart" {
		return fmt.Errorf("seek-probe: --mode is together or apart, not %q", *mode)
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

	var (
		mu    sync.Mutex
		loud  time.Time
		since time.Time
	)
	listening, stopListening := context.WithCancel(ctx)
	defer stopListening()
	go func() {
		_ = audiotap.Tap{}.Listen(listening, session.ProcessID(), func(packet voice.Packet) {
			sum := 0.0
			for _, sample := range packet.Samples {
				sum += float64(sample) * float64(sample)
			}
			if len(packet.Samples) == 0 || math.Sqrt(sum/float64(len(packet.Samples))) < loudLevel {
				return
			}
			mu.Lock()
			if loud.IsZero() && !since.IsZero() && packet.At.After(since) {
				loud = packet.At
			}
			mu.Unlock()
		})
	}()

	out := &printer{w: stdout}
	out.line("Probing %d laps of %s, sent %s, the passage from %v, resting at %v.", *laps, filepath.Base(clip), *mode, *start, *rest)
	x, y := int32(placement.X+placement.Width/2), int32(placement.Y+placement.Height/2)
	var results []lapTimes
	for lap := 1; lap <= *laps && ctx.Err() == nil; lap++ {
		if err := session.Pause(); err != nil {
			return err
		}
		if err := session.SeekTo(*rest); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
		case <-time.After(probeSettle):
		}
		mu.Lock()
		loud, since = time.Time{}, time.Now()
		sent := since
		mu.Unlock()
		if *mode == "together" {
			err = session.SeekAndPlay(*start)
		} else {
			err = errors.Join(session.SeekTo(*start), session.Play())
		}
		if err != nil {
			return err
		}
		picture := waitWhite(ctx, x, y, sent)
		for deadline := sent.Add(probeWait); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			mu.Lock()
			heard := !loud.IsZero()
			mu.Unlock()
			if heard {
				break
			}
		}
		mu.Lock()
		times := lapTimes{picture: picture, sound: -1}
		if !loud.IsZero() {
			times.sound = loud.Sub(sent)
		}
		mu.Unlock()
		results = append(results, times)
		out.line("lap %2d  picture %s  sound %s  picture - sound %s", lap, millis(times.picture), millis(times.sound), apart(times))
	}
	summarize(out, results)
	return out.err
}

// waitWhite polls the screen at (x, y) until it turns white, and tells how
// long after sent; -1 when it did not within probeWait.
func waitWhite(ctx context.Context, x, y int32, sent time.Time) time.Duration {
	screen, _, _ := procGetDC.Call(0)
	if screen == 0 {
		return -1
	}
	defer procReleaseDC.Call(0, screen)
	for time.Since(sent) < probeWait && ctx.Err() == nil {
		color, _, _ := procGetPixel.Call(screen, uintptr(x), uintptr(y))
		red, green, blue := color&0xFF, (color>>8)&0xFF, (color>>16)&0xFF
		if color != 0xFFFFFFFF && red > whiteLevel && green > whiteLevel && blue > whiteLevel {
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

func apart(times lapTimes) string {
	if times.picture < 0 || times.sound < 0 {
		return "   none"
	}
	return fmt.Sprintf("%+5d ms", (times.picture - times.sound).Milliseconds())
}

// summarize prints the median and the 95th percentile of each delay, and of
// the distance between picture and sound.
func summarize(out *printer, results []lapTimes) {
	var pictures, sounds, gaps []time.Duration
	for _, r := range results {
		if r.picture >= 0 {
			pictures = append(pictures, r.picture)
		}
		if r.sound >= 0 {
			sounds = append(sounds, r.sound)
		}
		if r.picture >= 0 && r.sound >= 0 {
			gaps = append(gaps, (r.picture - r.sound).Abs())
		}
	}
	for _, row := range []struct {
		name   string
		values []time.Duration
	}{{"picture", pictures}, {"sound", sounds}, {"|picture - sound|", gaps}} {
		if len(row.values) == 0 {
			out.line("%-18s none came", row.name)
			continue
		}
		slices.Sort(row.values)
		out.line("%-18s median %4d ms  p95 %4d ms  (%d laps)", row.name,
			row.values[len(row.values)/2].Milliseconds(), row.values[min(len(row.values)*95/100, len(row.values)-1)].Milliseconds(), len(row.values))
	}
}
