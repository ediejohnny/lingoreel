package ffmpeg_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/seekable"
	"github.com/ediejohnny/lingoreel/tests/testkit"
)

// makePipedFilm has the test FFmpeg write a film through a pipe, as a
// stream: a Matroska film made so has no index (its Cues) to seek in.
func makePipedFilm(t *testing.T, path, file string, args ...string) {
	t.Helper()
	out, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	command := exec.Command(path, append(append([]string{"-hide_banner", "-loglevel", "error"}, args...), "pipe:1")...)
	var problems bytes.Buffer
	command.Stdout, command.Stderr = out, &problems
	if err := command.Run(); err != nil {
		t.Fatalf("making the film: %v: %s", err, problems.String())
	}
}

// runs keeps what each still cost.
type runs struct {
	mu   sync.Mutex
	seen []ffmpeg.Run
}

func (r *runs) hear(run ffmpeg.Run) {
	r.mu.Lock()
	r.seen = append(r.seen, run)
	r.mu.Unlock()
}

func (r *runs) all() []ffmpeg.Run {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ffmpeg.Run(nil), r.seen...)
}

// A film with no index gives no stills: told at once when the app can see
// it (an MKV's Cues, an AVI's index), else after two stills that read far more than a still reads (a seek
// there reads the film from its start). A still of a film with its index
// reads little, and one nobody waits for is held to its share of a
// processor.
func TestAFilmThatSeeksSlowlyGivesNoStills(t *testing.T) {
	testkit.Slow(t)
	path := testFFmpeg(t)
	dir := t.TempDir()
	// About 3 MB a second: reaching its last seconds reads some 170 MB.
	piped := filepath.Join(dir, "piped.mkv")
	makePipedFilm(t, path, piped, "-f", "lavfi", "-i", "testsrc=s=640x360:r=25:d=60,noise=alls=30:allf=t",
		"-c:v", "mjpeg", "-q:v", "3", "-f", "matroska")
	if stat, err := os.Stat(piped); err != nil || stat.Size() < 150<<20 {
		t.Fatalf("the film without an index is too small to read too much of: %v", err)
	}
	pipedAVI := filepath.Join(dir, "piped.avi")
	makePipedFilm(t, path, pipedAVI, "-f", "lavfi", "-i", "testsrc=s=640x360:r=25:d=5", "-c:v", "mpeg4", "-f", "avi")
	indexed := map[string][]string{
		"film.mkv": {"-c:v", "mpeg4"},
		"film.avi": {"-c:v", "mpeg4"},
		"film.mp4": {"-c:v", "mpeg4"},
	}
	for name, codec := range indexed {
		makeFilm(t, path, append(append([]string{"-f", "lavfi", "-i", "testsrc=s=640x360:r=25:d=20"}, codec...), filepath.Join(dir, name))...)
	}
	known := ffmpeg.Stills{Path: path, Indexed: seekable.Indexed}
	for _, file := range []string{piped, pipedAVI} {
		if _, err := known.Open(context.Background(), file); !errors.Is(err, thumbnail.ErrNoStills) {
			t.Fatalf("%s, without its index, opened for stills: %v", filepath.Base(file), err)
		}
	}
	cost := &runs{}
	for name := range indexed {
		film, err := ffmpeg.Stills{Path: path, Indexed: seekable.Indexed, Report: cost.hear}.Open(context.Background(), filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := film.Grab(context.Background(), thumbnail.Shot{At: 15 * time.Second, Paced: true}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, run := range cost.all() {
		t.Logf("paced still: %v, %v of a processor, %d KB in %d reads", run.Wall, run.CPU, run.Read>>10, run.Reads)
		if run.Read == 0 || run.Read > 4<<20 || run.Reads == 0 || !run.Capped {
			t.Errorf("a paced still of a film with its index: %+v, want a little read, held to its share", run)
		}
	}

	cost = &runs{}
	film, err := ffmpeg.Stills{Path: path, Report: cost.hear}.Open(context.Background(), piped)
	if err != nil {
		t.Fatal(err)
	}
	first, err := film.Grab(context.Background(), thumbnail.Shot{At: 50 * time.Second})
	if err == nil || errors.Is(err, thumbnail.ErrNoStills) {
		t.Fatalf("the first still that read too much: %+v, %v; want a still that failed", first, err)
	}
	if _, err := film.Grab(context.Background(), thumbnail.Shot{At: 55 * time.Second}); !errors.Is(err, thumbnail.ErrNoStills) {
		t.Fatalf("the second still that read too much: %v, want no stills", err)
	}
	if _, err := film.Grab(context.Background(), thumbnail.Shot{At: time.Second}); !errors.Is(err, thumbnail.ErrNoStills) {
		t.Fatalf("after two: %v, want no stills", err)
	}
	seen := cost.all()
	if len(seen) != 2 {
		t.Fatalf("%d stills run, want 2: %+v", len(seen), seen)
	}
	for _, run := range seen {
		t.Logf("still ended: %v, %d KB in %d reads", run.Wall, run.Read>>10, run.Reads)
		// Ended as it passes its bounds, it never reads its way to the end.
		if run.Read < 16<<20 && run.Reads <= 500 || run.Read > 100<<20 {
			t.Errorf("a still ended for reading too much read %d bytes in %d reads", run.Read, run.Reads)
		}
	}
}

// One of the app's FFmpeg runs at a time: while the voice's decodes, a still
// nobody waits for waits its turn, and the one the pointer waits for does
// not.
func TestOneFFmpegRunsAtATime(t *testing.T) {
	testkit.Slow(t)
	path := testFFmpeg(t)
	file := filepath.Join(t.TempDir(), "film.mkv")
	makeFilm(t, path, "-f", "lavfi", "-i", "testsrc=s=640x360:r=25:d=60", "-f", "lavfi", "-i", "sine=frequency=440:duration=60",
		"-c:v", "mpeg4", "-c:a", "aac", "-ac", "2", file)
	film, err := ffmpeg.Stills{Path: path}.Open(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	holding, release := make(chan struct{}), make(chan struct{})
	decoded := make(chan error, 1)
	go func() {
		once := sync.Once{}
		decoded <- ffmpeg.Decoder{Path: path}.Decode(context.Background(),
			voice.Stretch{File: file, Track: 0, Channels: 2, Codec: "A_AAC", Length: 50 * time.Second},
			func([]float32, int) {
				once.Do(func() {
					close(holding)
					<-release
				})
			})
	}()
	select {
	case <-holding:
	case <-time.After(30 * time.Second):
		t.Fatal("the voice never began")
	}
	background := make(chan error, 1)
	go func() {
		_, err := film.Grab(context.Background(), thumbnail.Shot{At: 30 * time.Second})
		background <- err
	}()
	select {
	case err := <-background:
		t.Fatalf("a still nobody waits for ran beside the voice (%v)", err)
	case <-time.After(time.Second):
	}
	pointed, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	_, err = film.Grab(pointed, thumbnail.Shot{At: 30 * time.Second, Waited: true})
	cancel()
	close(release)
	if err != nil {
		t.Fatalf("the pointer's still beside the voice: %v", err)
	}
	for _, result := range []chan error{decoded, background} {
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("the turn was never handed on")
		}
	}
}
