package spdev

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/seekable"
)

// maxParallel bounds the stills taken at a time.
const maxParallel = 4

// stills measures the seek bar's stills of a film as the app takes them:
// what each costs in time, processor and reads. It prints numbers and the
// film's key, never its name.
func stills(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("%w: the video's path", ErrMissingArgument)
	}
	path := args[0]
	flags := flag.NewFlagSet("stills", flag.ContinueOnError)
	flags.SetOutput(stdout)
	tool := flags.String("ffmpeg", "", "the FFmpeg to take the stills with; found as the app finds it by default")
	count := flags.Int("count", 20, "how many stills, spread over the film")
	length := flags.Duration("duration", 0, "the film's length (1h32m), for a video whose length the app cannot read")
	paced := flags.Bool("paced", false, "takes them as the background does, held to a share of one processor")
	rough := flags.Bool("rough", false, "has the decoder skip the deblocking")
	parallel := flags.Int("parallel", 1, "how many at a time; more than one skips the app's one-at-a-time turn")
	watching := flags.Bool("watch", false, "plays the film in VLC beside the stills, counting the pictures it loses, against as long without them")
	affinity := flags.String("affinity", "", "holds VLC, this tool and its FFmpeg to these processors (0xF: the first four), as a smaller computer")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *count < 1 || *parallel < 1 || *parallel > maxParallel {
		return fmt.Errorf("%w: --count at least 1, --parallel from 1 to %d", ErrInvalidOption, maxParallel)
	}
	mask := uint64(0)
	if *affinity != "" {
		var err error
		if mask, err = strconv.ParseUint(*affinity, 0, 64); err != nil || mask == 0 {
			return fmt.Errorf("%w: --affinity %q, want a mask such as 0xF", ErrInvalidOption, *affinity)
		}
	}
	key, err := filmKey(path)
	if err != nil {
		return err
	}
	duration := *length
	if duration <= 0 {
		if info, err := (matroska.FileReader{}).Analyze(path); err == nil {
			duration = info.Duration
		}
	}
	if duration <= 0 {
		return fmt.Errorf("%w: give the film's length with --duration", ErrMissingArgument)
	}
	found, err := ffmpeg.Find(ctx, *tool)
	if err != nil {
		return err
	}

	out := &printer{w: stdout}
	indexed, indexErr := seekable.Indexed(path)
	out.line("Film:     %s, %s", key, formatClock(duration))
	out.line("Index:    %v%s", indexed, errorNote(indexErr, path))
	out.line("Stills:   %d, paced %v, rough %v, %d at a time, processors %s", *count, *paced, *rough, *parallel,
		cmp.Or(*affinity, "all"))
	if mask != 0 {
		if err := pinSelf(mask); err != nil {
			return err
		}
	}
	var mu sync.Mutex
	var seen []ffmpeg.Run
	film, err := ffmpeg.Stills{Path: found, Rough: *rough, Report: func(run ffmpeg.Run) {
		mu.Lock()
		seen = append(seen, run)
		mu.Unlock()
		out.line("  %s  %4d ms  cpu %4d ms  %6d KB in %3d reads%s%s", formatClock(run.At), run.Wall.Milliseconds(),
			run.CPU.Milliseconds(), run.Read>>10, run.Reads, capped(run), errorNote(run.Err, path))
	}}.Open(ctx, path)
	if err != nil {
		return err
	}

	var vlc *vlcWatch
	if *watching {
		if vlc, err = startVLC(ctx, path, duration/10, mask); err != nil {
			return err
		}
		defer vlc.close()
		out.line("VLC:      playing from %s, muted; the stills begin in %v", formatClock(duration/10), watchWarmUp)
		select {
		case <-time.After(watchWarmUp):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	var took time.Duration
	take := func() {
		moments := make(chan time.Duration)
		var workers sync.WaitGroup
		began := time.Now()
		for range *parallel {
			workers.Go(func() {
				for at := range moments {
					_, _ = film.Grab(ctx, thumbnail.Shot{At: at, Paced: *paced, Waited: *parallel > 1})
				}
			})
		}
		for i := range *count {
			select {
			case moments <- time.Duration(float64(duration) * (float64(i) + 0.5) / float64(*count)):
			case <-ctx.Done():
			}
		}
		close(moments)
		workers.Wait()
		took = time.Since(began)
	}
	if vlc == nil {
		take()
	} else {
		with, err := vlc.during(ctx, take)
		if err != nil {
			return err
		}
		without, err := vlc.during(ctx, func() {
			select {
			case <-time.After(took):
			case <-ctx.Done():
			}
		})
		if err != nil {
			return err
		}
		out.line("VLC:      %d of %d pictures lost while the stills ran, %d of %d in as long without them",
			with.lost, with.shown+with.lost, without.lost, without.shown+without.lost)
	}

	failed := 0
	var walls, cpus []time.Duration
	var read []uint64
	for _, run := range seen {
		if run.Err != nil {
			failed++
			continue
		}
		walls, cpus, read = append(walls, run.Wall), append(cpus, run.CPU), append(read, run.Read)
	}
	out.line("Took:     %.1f s for %d stills (%.2f a second), %d failed", took.Seconds(), len(seen),
		float64(len(seen))/took.Seconds(), failed)
	if len(walls) > 0 {
		out.line("Median:   %d ms, cpu %d ms, %d KB; slowest %d ms, most read %d KB",
			median(walls).Milliseconds(), median(cpus).Milliseconds(), median(read)>>10,
			slices.Max(walls).Milliseconds(), slices.Max(read)>>10)
	}
	if out.err == nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return out.err
}

func median[T time.Duration | uint64](values []T) T {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}

func capped(run ffmpeg.Run) string {
	if run.Capped {
		return "  capped"
	}
	return ""
}

// errorNote words an error for the output, with the film's name left out:
// FFmpeg names the file in its complaints.
func errorNote(err error, path string) string {
	if err == nil {
		return ""
	}
	text := strings.NewReplacer(path, "<film>", filepath.Base(path), "<film>").Replace(err.Error())
	if errors.Is(err, thumbnail.ErrNoStills) {
		return "  (no stills: " + text + ")"
	}
	return "  (" + text + ")"
}
