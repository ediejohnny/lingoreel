package ffmpeg

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/thumbnail"
)

// StillWidth and StillHeight are the box a still fits in, keeping its
// shape: sharp on the bar (240x135 at 100%) up to a screen scaled to 160%.
const (
	StillWidth  = 384
	StillHeight = 216
)

const (
	// describeTimeout bounds FFmpeg's description of a film, stillTimeout
	// one still.
	describeTimeout = 15 * time.Second
	stillTimeout    = 15 * time.Second
	// maxDescription is how much of the description is kept: the video's
	// line comes after the chapters and the tags, well within it.
	maxDescription = 64 << 10
	// maxStill bounds the JPEG handed back; a still takes 20 to 35 KB.
	maxStill = 512 << 10
	// stillMemory is the most a still's FFmpeg may commit: decoding one 4K
	// frame on one thread takes well under it.
	stillMemory = 384 << 20
	// pacedShare is the share of one processor the stills nobody waits for
	// take, together.
	pacedShare = 0.25
	// maxRead and maxReads bound what one still reads: a still reads 0.6 to
	// 1.5 MB in some 50 reads; one that reads far more is in a film that
	// seeks slowly (no index, keyframes far apart).
	maxRead  = 24 << 20
	maxReads = 500
	// slowAfter is how many stills that read too much stop a film's stills.
	slowAfter = 2
)

// errReadTooMuch reports a still ended for reading past its bounds.
var errReadTooMuch = errors.New("ffmpeg: the still read too much of the film")

// reads is what one FFmpeg read, and whether that went past its bounds.
type reads struct {
	bytes, count uint64
	over         bool
}

func tooMuch(bytes, count uint64) bool { return bytes > maxRead || count > maxReads }

// Run is what one still cost, for measuring.
type Run struct {
	At time.Duration
	// Paced tells a still nobody waited for; Capped, one the system held to
	// its share of the processor (one not held rests by itself after).
	Paced, Capped bool
	Wall, CPU     time.Duration
	// Read is how much of the film it read, in bytes, and Reads in how many.
	Read, Reads uint64
	Err         error
}

// The HDR curves a still is tone-mapped from.
const (
	TransferPQ  = "smpte2084"
	TransferHLG = "arib-std-b67"
)

// Look is what a film's picture needs to come out right in a still, read
// once from FFmpeg's description of the file.
type Look struct {
	// UntaggedHD is a picture larger than SD with no colour tags: players
	// take it as BT.709, and so must the still.
	UntaggedHD bool
	// FullRange is a picture tagged as using every level (pc).
	FullRange  bool
	Interlaced bool
	// Transfer is the HDR curve to tone-map (TransferPQ, TransferHLG), ""
	// for an ordinary picture.
	Transfer string
	// DolbyVision is the profile, 0 for none.
	DolbyVision int
}

// formats are FFmpeg's names for the containers the app opens, given to it
// so that it never guesses another one from the file's content.
var formats = map[string]string{
	".mkv": "matroska", ".webm": "matroska", ".mp4": "mov", ".m4v": "mov", ".mov": "mov", ".avi": "avi", ".wmv": "asf",
}

// ReadLook reads, from FFmpeg's description of a file, how its first
// picture (never a cover) must be taken.
func ReadLook(description string) (Look, error) {
	lines := strings.Split(strings.ReplaceAll(description, "\r\n", "\n"), "\n")
	for i, line := range lines {
		_, video, found := strings.Cut(line, ": Video: ")
		if !found || !strings.Contains(line, "Stream #") || strings.Contains(video, "(attached pic)") {
			continue
		}
		look := lookOf(video)
		for _, next := range lines[i+1:] {
			if strings.Contains(next, "Stream #") {
				break
			}
			if _, record, ok := strings.Cut(next, "DOVI configuration record:"); ok {
				look.DolbyVision = doviProfile(record)
			}
		}
		return look, nil
	}
	return Look{}, fmt.Errorf("%w: no picture in the file", thumbnail.ErrNoStills)
}

// lookOf reads a video stream's line: "h264 (High), yuv420p(tv, bt709,
// progressive), 1920x1080 [SAR 1:1 DAR 16:9], ...".
func lookOf(video string) Look {
	fields := splitOutside(video)
	width, height := 0, 0
	for _, field := range fields[1:] {
		var w, h int
		if n, _ := fmt.Sscanf(field, "%dx%d", &w, &h); n == 2 {
			width, height = w, h
			break
		}
	}
	var look Look
	space := ""
	if len(fields) > 1 {
		if _, inside, found := strings.Cut(fields[1], "("); found {
			for _, item := range splitOutside(strings.TrimSuffix(inside, ")")) {
				switch {
				case item == "tv":
				case item == "pc":
					look.FullRange = true
				case item == "progressive":
				case strings.Contains(item, "first"):
					look.Interlaced = true
				default:
					// "bt709" names the matrix, the primaries and the curve at
					// once; "bt2020nc/bt2020/smpte2084" names them one by one.
					transfer := item
					if parts := strings.Split(item, "/"); len(parts) == 3 {
						space, transfer = parts[0], parts[2]
					} else {
						space = item
					}
					if transfer == TransferPQ || transfer == TransferHLG {
						look.Transfer = transfer
					}
				}
			}
		}
	}
	look.UntaggedHD = (space == "" || space == "unknown") && (width >= 1280 || height > 576)
	return look
}

// splitOutside splits on ", " outside parentheses.
func splitOutside(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '(':
			depth++
		case s[i] == ')':
			depth = max(depth-1, 0)
		case depth == 0 && strings.HasPrefix(s[i:], ", "):
			parts = append(parts, strings.TrimSpace(s[start:i]))
			start = i + 2
		}
	}
	return append(parts, strings.TrimSpace(s[start:]))
}

var doviProfileRule = regexp.MustCompile(`profile: (\d+)`)

func doviProfile(record string) int {
	if match := doviProfileRule.FindStringSubmatch(record); match != nil {
		profile, _ := strconv.Atoi(match[1])
		return profile
	}
	return 0
}

// StillFilter scales a film's picture into the still's box and turns it to
// the colours of a JPEG (BT.601, every level), as Go's decoder reads them.
// An HDR picture is made small first and tone-mapped after: FFmpeg's own
// tone mapping inside "scale" costs seconds of every core for each still.
// The curve is mobius, which leaves the mid tones as they are and bends only
// the highlights, so a still is as bright as the film looks.
func StillFilter(look Look) string {
	var chain []string
	if look.Interlaced {
		chain = append(chain, "bwdif=mode=send_frame:deint=interlaced")
	}
	size := fmt.Sprintf("w='if(gte(dar,%[1]d/%[2]d),%[1]d,round(%[2]d*dar))':h='if(gte(dar,%[1]d/%[2]d),round(%[1]d/dar),%[2]d)'",
		StillWidth, StillHeight)
	if look.Transfer != "" {
		return strings.Join(append(chain,
			"scale="+size+":flags=bilinear", "format=yuv444p10le", "setsar=1",
			"zscale=tin="+look.Transfer+":pin=bt2020:min=bt2020nc:rin=limited:t=linear:npl=100",
			"format=gbrpf32le", "zscale=p=bt709", "tonemap=mobius:desat=0",
			"zscale=t=bt709:m=bt470bg:r=full", "format=yuvj444p"), ",")
	}
	scale := "scale=" + size + ":flags=bicubic"
	if look.UntaggedHD {
		scale += ":in_color_matrix=bt709"
		if !look.FullRange {
			scale += ":in_range=tv"
		}
	}
	return strings.Join(append(chain, scale+":out_color_matrix=bt601:out_range=pc", "setsar=1", "format=yuvj444p"), ",")
}

// StillArguments is the command line that hands over, as a JPEG on the
// standard output, the keyframe at or just before a moment: the seek lands
// on it and nothing after it is decoded ("passthrough" keeps the frame
// before the moment, which FFmpeg would otherwise drop and decode on to the
// moment itself). withTime also has FFmpeg tell, on its error output, how
// far before the moment that frame is. (Skipping the decoder's deblocking
// saves only some 6% of a still: not worth the rougher picture.)
func StillArguments(file, format string, at time.Duration, look Look, withTime bool) []string {
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-filter_threads", "1",
		"-threads", "1", "-noaccurate_seek", "-ss", seconds(at),
		"-protocol_whitelist", "file", "-f", format, "-i", "file:" + file,
		"-map", "0:V:0", "-an", "-sn", "-dn", "-frames:v", "1", "-fps_mode", "passthrough",
		"-vf", StillFilter(look), "-c:v", "mjpeg", "-q:v", "4"}
	if withTime {
		args = append(args, "-stats_enc_pre", "pipe:2", "-stats_enc_pre_fmt", "{t}")
	}
	return append(args, "-f", "image2pipe", "pipe:1")
}

// Stills takes a film's stills with an FFmpeg.
type Stills struct {
	Path string
	// Indexed tells whether a film has the index a seek lands through
	// (false: no stills); nil, or an error, leaves it to the bounds on reads.
	Indexed func(file string) (bool, error)
	// Report hears what each still cost; nil hears nothing.
	Report func(Run)
	// ProbeSize is how many bytes FFmpeg reads to learn a film's streams
	// before a still (its -probesize); zero leaves FFmpeg's own 5 MB. For
	// measuring a smaller one.
	ProbeSize int64
}

// Open looks at a film once, to know how its stills must be taken.
func (s Stills) Open(ctx context.Context, file string) (thumbnail.Film, error) {
	format, known := formats[strings.ToLower(filepath.Ext(file))]
	if !known {
		return nil, fmt.Errorf("%w: %s is not a video the app opens", thumbnail.ErrNoStills, filepath.Ext(file))
	}
	if s.Indexed != nil {
		if indexed, err := s.Indexed(file); err == nil && !indexed {
			return nil, fmt.Errorf("%w: the film has no index to seek in", thumbnail.ErrNoStills)
		}
	}
	description, err := s.describe(ctx, file, format)
	if err != nil {
		return nil, err
	}
	look, err := ReadLook(description)
	if err != nil {
		return nil, err
	}
	// Profile 5 has no picture of its own a player shows without Dolby's
	// reshaping, which no CPU path does right.
	if look.DolbyVision == 5 {
		return nil, fmt.Errorf("%w: Dolby Vision profile 5", thumbnail.ErrNoStills)
	}
	return &film{path: s.Path, file: file, format: format, look: look, report: s.Report, probe: s.ProbeSize}, nil
}

// describe has FFmpeg describe the file: given no output, it describes its
// input on the error output and ends. Every still waits for it, and it is
// brief, so it runs at the normal priority; past its time, waiting for its
// turn behind another FFmpeg of the app or held back by the computer, it is
// busy (thumbnail.ErrBusy).
func (s Stills) describe(ctx context.Context, file, format string) (string, error) {
	const what = "describing the film"
	ctx, cancel := context.WithTimeout(ctx, describeTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, s.Path, "-hide_banner", "-nostdin", "-protocol_whitelist", "file", "-f", format, "-i", "file:"+file)
	command.SysProcAttr = brief()
	var description bytes.Buffer
	command.Stderr = &limitedWriter{buffer: &description, room: maxDescription}
	release, err := takeTurn(ctx)
	if err != nil {
		return "", late(what, err, true) // the wait for the turn is never the film's doing
	}
	defer release()
	began := time.Now()
	if _, err := start(command, plainRun); err != nil {
		return "", err
	}
	_ = command.Wait() // it always fails: there is no output
	if err := ctx.Err(); err != nil {
		return "", late(what, err, Starved(processorOf(command), time.Since(began), 1))
	}
	return description.String(), nil
}

// film takes the stills of one video.
type film struct {
	path, file, format string
	look               Look
	report             func(Run)
	// probe is FFmpeg's -probesize, zero for its own.
	probe int64
	// untimed is an FFmpeg older than 6.1, which cannot tell the frame's
	// time: its stills stand for the moment asked.
	untimed atomic.Bool
	// slow counts the stills that read too much.
	slow atomic.Int32
}

// cost is what one run of FFmpeg took.
type cost struct {
	wall, cpu time.Duration
	reads     reads
	capped    bool
}

// Starved tells, of a run that reached its deadline, whether the computer
// held it back rather than the work being too much: it had less than half
// the processor it could take (share: 1 for one thread, less for a run held
// to a share of one). What was starved is asked again later; what worked the
// whole time is not, so a film too heavy for its stills is never tried for
// ever.
func Starved(cpu, wall time.Duration, share float64) bool {
	return float64(cpu) < share*float64(wall)/2
}

// processorOf is the processor time an ended run took.
func processorOf(command *exec.Cmd) time.Duration {
	if state := command.ProcessState; state != nil {
		return state.UserTime() + state.SystemTime()
	}
	return 0
}

// late words work that reached its deadline: busy (thumbnail.ErrBusy) when
// the computer held it back, its plain context error otherwise.
func late(what string, err error, heldBack bool) error {
	if heldBack && errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %s: %w", thumbnail.ErrBusy, what, err)
	}
	return fmt.Errorf("ffmpeg: %s: %w", what, err)
}

// Grab takes a still: the pointer's at once, the others in their turn after
// any other FFmpeg of the app; a paced one the system could not hold to its
// share rests after, in proportion to the processor it took.
func (f *film) Grab(ctx context.Context, shot thumbnail.Shot) (thumbnail.Picture, error) {
	if f.slow.Load() >= slowAfter {
		return thumbnail.Picture{}, fmt.Errorf("%w: the film seeks too slowly", thumbnail.ErrNoStills)
	}
	release := func() {}
	if !shot.Waited {
		var err error
		if release, err = takeTurn(ctx); err != nil {
			return thumbnail.Picture{}, err
		}
	}
	picture, spent, problems, err := f.grab(ctx, shot, !f.untimed.Load())
	if err != nil && !f.untimed.Load() && strings.Contains(problems, "stats_enc_pre") {
		f.untimed.Store(true)
		picture, spent, _, err = f.grab(ctx, shot, false)
	}
	release()
	if errors.Is(err, errReadTooMuch) && f.slow.Add(1) >= slowAfter {
		err = fmt.Errorf("%w: the film seeks too slowly: %w", thumbnail.ErrNoStills, err)
	}
	if f.report != nil {
		f.report(Run{At: shot.At, Paced: shot.Paced, Capped: spent.capped, Wall: spent.wall, CPU: spent.cpu,
			Read: spent.reads.bytes, Reads: spent.reads.count, Err: err})
	}
	if shot.Paced && !spent.capped {
		rest(ctx, cmp.Or(spent.cpu, spent.wall))
	}
	return picture, err
}

// rest waits as long as a paced still's share asks after work, or until ctx
// ends.
func rest(ctx context.Context, work time.Duration) {
	select {
	case <-time.After(time.Duration(float64(work) * (1 - pacedShare) / pacedShare)):
	case <-ctx.Done():
	}
}

// grab runs one still, and tells what it cost and what FFmpeg complained
// about.
func (f *film) grab(ctx context.Context, shot thumbnail.Shot, withTime bool) (thumbnail.Picture, cost, string, error) {
	ctx, cancel := context.WithTimeout(ctx, stillTimeout)
	defer cancel()
	args := StillArguments(f.file, f.format, shot.At, f.look, withTime)
	if f.probe > 0 {
		args = append([]string{"-probesize", strconv.FormatInt(f.probe, 10)}, args...)
	}
	command := exec.CommandContext(ctx, f.path, args...)
	command.SysProcAttr = background()
	var told bytes.Buffer
	command.Stderr = &limitedWriter{buffer: &told, room: 4096}
	out, err := command.StdoutPipe()
	if err != nil {
		return thumbnail.Picture{}, cost{}, "", fmt.Errorf("ffmpeg: %w", err)
	}
	kind := stillRun
	if shot.Paced {
		kind = pacedRun
	}
	began := time.Now()
	capped, err := start(command, kind)
	if err != nil {
		return thumbnail.Picture{}, cost{}, "", err
	}
	watch := watchReads(command.Process, cancel)
	still, readErr := io.ReadAll(io.LimitReader(out, maxStill+1))
	if len(still) > maxStill {
		cancel()
	}
	waitErr := command.Wait()
	spent := cost{wall: time.Since(began), cpu: processorOf(command), reads: watch.stop(), capped: capped && kind == pacedRun}
	// A still runs on one thread, a capped one on its share of it.
	share := 1.0
	if spent.capped {
		share = pacedShare
	}
	switch {
	case spent.reads.over:
		return thumbnail.Picture{}, spent, "", fmt.Errorf("%w at %v (%d KB in %d reads)", errReadTooMuch, shot.At,
			spent.reads.bytes>>10, spent.reads.count)
	case len(still) > maxStill:
		return thumbnail.Picture{}, spent, "", errors.New("ffmpeg: the still is larger than any still")
	case ctx.Err() != nil:
		what := fmt.Sprintf("taking the still at %v", shot.At)
		return thumbnail.Picture{}, spent, "", late(what, ctx.Err(), Starved(spent.cpu, spent.wall, share))
	case waitErr != nil:
		problems := strings.TrimSpace(told.String())
		return thumbnail.Picture{}, spent, problems, fmt.Errorf("ffmpeg: %w: %s", waitErr, problems)
	case readErr != nil:
		return thumbnail.Picture{}, spent, "", fmt.Errorf("ffmpeg: reading the still: %w", readErr)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(still))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > StillWidth || config.Height > StillHeight {
		return thumbnail.Picture{}, spent, "", fmt.Errorf("ffmpeg: what came back is not a still (%v)", err)
	}
	return thumbnail.Picture{At: frameTime(shot.At, told.String()), Width: config.Width, Height: config.Height, JPEG: still}, spent, "", nil
}

var offsetRule = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// frameTime is the moment asked moved by what FFmpeg told of the frame (its
// time from that moment, on the last line); without a sane one, the moment.
func frameTime(at time.Duration, told string) time.Duration {
	words := strings.Fields(told)
	if len(words) == 0 || !offsetRule.MatchString(words[len(words)-1]) {
		return at
	}
	offset, err := strconv.ParseFloat(words[len(words)-1], 64)
	if err != nil || offset > 1 || offset < -600 {
		return at
	}
	return max(at+time.Duration(offset*float64(time.Second)), 0)
}
