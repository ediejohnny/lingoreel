package spdev

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/voice"
	"github.com/ediejohnny/lingoreel/internal/domain/speech"
	"github.com/ediejohnny/lingoreel/internal/domain/study"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/ffmpeg"
)

// A clip is a line's sound, with some before and after it, for someone to
// mark by ear where the voice really begins: the subtitle's own start is a
// little off, so it cannot be the measure of the voice found.
const (
	clipBefore    = 1500 * time.Millisecond
	clipAfter     = time.Second
	clipLongest   = 12 * time.Second
	clipShortest  = 600 * time.Millisecond // shorter lines are hard to mark
	clipPeakEvery = 10 * time.Millisecond
	clipsData     = "clips.js"
)

// clip is what a page for marking needs of one line; Text stays on the
// computer, in the folder given.
type clip struct {
	ID        string    `json:"id"`
	Film      string    `json:"film"`
	File      string    `json:"file"`
	Text      string    `json:"text"`
	ClipStart int64     `json:"clipStart"` // ms of the film
	LineStart int64     `json:"lineStart"`
	LineEnd   int64     `json:"lineEnd"`
	Peaks     []float32 `json:"peaks"` // the loudest sample of every 10 ms
}

// label is a line marked by hand: where its voice begins, on the film's
// clock; no onset means it could not be told.
type label struct {
	Film      string `json:"film"`
	LineStart int64  `json:"lineStart"`
	Onset     *int64 `json:"onset"`
}

// filmKey names a film by its file (path and size), never by its name.
func filmKey(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(fmt.Appendf(nil, "%s|%d", strings.ToLower(abs), info.Size()))
	return hex.EncodeToString(sum[:6]), nil
}

// pickLines chooses up to n lines for marking, one from each n-th of the
// film, the same ones every time for the same film.
func pickLines(film string, events []subtitle.Event, shift, heard time.Duration, n int) []int {
	seed := uint64(0)
	for _, b := range []byte(film) {
		seed = seed*131 + uint64(b)
	}
	random := rand.New(rand.NewPCG(seed, 7))
	var picked []int
	for part := range n {
		from, to := heard*time.Duration(part)/time.Duration(n), heard*time.Duration(part+1)/time.Duration(n)
		var fits []int
		for i, event := range events {
			start, end := event.Start+shift, event.End+shift
			units := study.SpeechUnits(event.Text())
			if start >= from && start < to && start >= clipBefore && end+clipAfter < heard && end-start >= clipShortest &&
				units.Spoken() && !units.Lyrics {
				fits = append(fits, i)
			}
		}
		if len(fits) > 0 {
			picked = append(picked, fits[random.IntN(len(fits))])
		}
	}
	return picked
}

// writeClips cuts the lines picked into WAV files in out/clips and gathers
// every film's clips there into out/clips.js, for a page to read.
func writeClips(ctx context.Context, tool string, stretch voice.Stretch, film string, events []subtitle.Event, picked []int, shift time.Duration, out string) error {
	folder := filepath.Join(out, "clips")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	path, err := ffmpeg.Find(ctx, tool)
	if err != nil {
		return err
	}
	var clips []clip
	for n, i := range picked {
		event := events[i]
		start, end := event.Start+shift, event.End+shift
		cut := stretch
		cut.From = start - clipBefore
		cut.Length = min(end+clipAfter, cut.From+clipLongest) - cut.From
		var sound []float32
		err := ffmpeg.Decoder{Path: path}.Decode(ctx, cut, func(samples []float32, channels int) {
			for j := 0; j+channels <= len(samples); j += channels {
				sum := float32(0)
				for _, s := range samples[j : j+channels] {
					sum += s
				}
				sound = append(sound, sum/float32(channels))
			}
		})
		if err != nil {
			return err
		}
		id := fmt.Sprintf("%s-%02d", film, n+1)
		if err := os.WriteFile(filepath.Join(folder, id+".wav"), wav(sound), 0o644); err != nil {
			return err
		}
		clips = append(clips, clip{
			ID: id, Film: film, File: "clips/" + id + ".wav", Text: event.Text(),
			ClipStart: cut.From.Milliseconds(), LineStart: start.Milliseconds(), LineEnd: end.Milliseconds(),
			Peaks: peaks(sound),
		})
	}
	data, err := json.Marshal(clips)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(folder, film+".json"), data, 0o644); err != nil {
		return err
	}
	return gatherClips(out)
}

// gatherClips writes every film's clips into one script, which a page
// opened from the disk can load (it cannot fetch files).
func gatherClips(out string) error {
	files, err := filepath.Glob(filepath.Join(out, "clips", "*.json"))
	if err != nil {
		return err
	}
	slices.Sort(files)
	all := []json.RawMessage{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		var clips []json.RawMessage
		if err := json.Unmarshal(data, &clips); err != nil {
			return fmt.Errorf("voice-check: %s: %w", filepath.Base(file), err)
		}
		all = append(all, clips...)
	}
	data, err := json.Marshal(all)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, clipsData), append(append([]byte("window.clips = "), data...), ";\n"...), 0o644)
}

// wav is mono 16-bit PCM at speech.SampleRate.
func wav(sound []float32) []byte {
	size := 2 * len(sound)
	data := make([]byte, 0, 44+size)
	data = append(data, "RIFF"...)
	data = binary.LittleEndian.AppendUint32(data, uint32(36+size))
	data = append(data, "WAVEfmt "...)
	data = binary.LittleEndian.AppendUint32(data, 16)
	data = binary.LittleEndian.AppendUint16(data, 1) // PCM
	data = binary.LittleEndian.AppendUint16(data, 1) // mono
	data = binary.LittleEndian.AppendUint32(data, speech.SampleRate)
	data = binary.LittleEndian.AppendUint32(data, 2*speech.SampleRate)
	data = binary.LittleEndian.AppendUint16(data, 2)
	data = binary.LittleEndian.AppendUint16(data, 16)
	data = append(data, "data"...)
	data = binary.LittleEndian.AppendUint32(data, uint32(size))
	for _, s := range sound {
		data = binary.LittleEndian.AppendUint16(data, uint16(int16(math.Round(float64(max(min(s, 1), -1))*math.MaxInt16))))
	}
	return data
}

// peaks is the loudest sample of every clipPeakEvery, for drawing.
func peaks(sound []float32) []float32 {
	every := int(clipPeakEvery.Seconds() * speech.SampleRate)
	var out []float32
	for i := 0; i < len(sound); i += every {
		peak := float32(0)
		for _, s := range sound[i:min(i+every, len(sound))] {
			peak = max(peak, float32(math.Abs(float64(s))))
		}
		out = append(out, float32(math.Round(float64(peak)*1000)/1000))
	}
	return out
}

// readLabels reads the lines marked by hand for one film, by the line's
// start on the film's clock.
func readLabels(path, film string) (map[int64]*int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 1<<20 {
		return nil, errors.New("voice-check: the labels file is too big")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var labels []label
	if err := json.Unmarshal(data, &labels); err != nil {
		return nil, fmt.Errorf("voice-check: the labels: %w", err)
	}
	marked := map[int64]*int64{}
	for _, l := range labels {
		if strings.EqualFold(l.Film, film) {
			marked[l.LineStart] = l.Onset
		}
	}
	return marked, nil
}
