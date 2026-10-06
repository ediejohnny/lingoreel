// Package ffmpeg uses the user's own FFmpeg, never one of ours: it finds an
// FFmpeg already on the machine (or the one the user points to), checks
// that it runs, and decodes stretches of a film's voice ahead of the video
// in a process of its own, at the lowest priority.
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
)

var (
	// ErrNotFound reports a machine where no FFmpeg was found.
	ErrNotFound = errors.New("ffmpeg: not found")
	// ErrNotFFmpeg reports a program that is not an FFmpeg that runs.
	ErrNotFFmpeg = errors.New("ffmpeg: the program is not an FFmpeg that runs")
)

// WingetID is the FFmpeg the app offers to install, under the LGPL: the
// user installs it from its authors through winget; the app ships none.
const WingetID = "BtbN.FFmpeg.LGPL.8.1"

// checkTimeout bounds "ffmpeg -version", which answers at once.
const checkTimeout = 5 * time.Second

// Check tells whether a program is an FFmpeg that runs.
func Check(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, path, "-hide_banner", "-version")
	command.SysProcAttr = background()
	out, err := command.Output()
	if err != nil || !bytes.HasPrefix(out, []byte("ffmpeg version")) {
		return fmt.Errorf("%w: %s", ErrNotFFmpeg, path)
	}
	return nil
}

// Find returns the first FFmpeg that runs: the one the user chose, else
// one where FFmpeg is usually installed on this system.
func Find(ctx context.Context, chosen string) (string, error) {
	for _, candidate := range append([]string{chosen}, candidates()...) {
		if candidate != "" && Check(ctx, candidate) == nil {
			return candidate, nil
		}
	}
	return "", ErrNotFound
}

// Arguments is the command line that decodes a stretch to 16 kHz float on
// the standard output, and the channels it gives: the centre alone of a
// surround mix (the dialogue's channel), both sides of a stereo one (the
// analyzer mixes them).
func Arguments(stretch voice.Stretch) ([]string, int) {
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-threads", "1"}
	if strings.HasPrefix(stretch.Codec, "A_DTS") {
		args = append(args, "-core_only", "1") // the core is enough, and the cheapest to decode
	}
	// file: keeps FFmpeg to reading that local file, whatever its name: it
	// never takes the path for another of its protocols (http:, concat:).
	args = append(args, "-ss", seconds(stretch.From), "-i", "file:"+stretch.File, "-t", seconds(stretch.Length),
		"-map", "0:a:"+strconv.Itoa(stretch.Track), "-vn", "-sn", "-dn")
	// Without first_pts=0 a track that starts later than the video (a
	// dubbed one, often) would shift every line by that much.
	filter, channels := "aresample="+strconv.Itoa(speech.SampleRate)+":async=1:first_pts=0", 2
	if stretch.Channels >= 3 {
		filter, channels = "pan=mono|c0=FC,"+filter, 1
	}
	return append(args, "-af", filter, "-ac", strconv.Itoa(channels), "-f", "f32le", "pipe:1"), channels
}

func seconds(d time.Duration) string { return strconv.FormatFloat(d.Seconds(), 'f', 3, 64) }

// Decoder decodes with an FFmpeg, implementing voice.Decoder.
type Decoder struct{ Path string }

var _ voice.Decoder = Decoder{}

// readSize is how much sound is read and handed over at a time: 100 ms of
// stereo.
const readSize = speech.SampleRate / 10 * 2 * 4

// Decode runs FFmpeg on a stretch and hands its sound over as it comes.
func (d Decoder) Decode(ctx context.Context, stretch voice.Stretch, hear func([]float32, int)) error {
	args, channels := Arguments(stretch)
	command := exec.CommandContext(ctx, d.Path, args...)
	command.SysProcAttr = background()
	var problems bytes.Buffer
	command.Stderr = &limitedWriter{buffer: &problems, room: 2048}
	out, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("ffmpeg: starting: %w", err)
	}
	readErr := deliver(out, channels, hear)
	waitErr := command.Wait()
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case waitErr != nil:
		return fmt.Errorf("ffmpeg: %w: %s", waitErr, strings.TrimSpace(problems.String()))
	case readErr != nil:
		return fmt.Errorf("ffmpeg: reading the sound: %w", readErr)
	}
	return nil
}

// deliver reads little-endian floats and hands them over in whole frames.
func deliver(out io.Reader, channels int, hear func([]float32, int)) error {
	raw := make([]byte, readSize)
	samples := make([]float32, 0, readSize/4)
	kept := 0 // bytes of an unfinished frame carried to the next read
	frame := 4 * channels
	for {
		n, err := out.Read(raw[kept:])
		n += kept
		whole := n - n%frame
		samples = samples[:0]
		for i := 0; i < whole; i += 4 {
			samples = append(samples, math.Float32frombits(binary.LittleEndian.Uint32(raw[i:])))
		}
		if len(samples) > 0 {
			hear(samples, channels)
		}
		kept = copy(raw, raw[whole:n])
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// limitedWriter keeps the first room bytes of what FFmpeg complains about.
type limitedWriter struct {
	buffer *bytes.Buffer
	room   int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if left := w.room - w.buffer.Len(); left > 0 {
		w.buffer.Write(p[:min(len(p), left)])
	}
	return len(p), nil
}
