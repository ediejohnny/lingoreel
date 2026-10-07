package ffmpeg

import (
	"bytes"
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
	// maxStill bounds the JPEG handed back; a still takes 5 to 25 KB.
	maxStill = 512 << 10
)

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
			"format=gbrpf32le", "zscale=p=bt709", "tonemap=hable:desat=0",
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
// far before the moment that frame is.
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
type Stills struct{ Path string }

// Open looks at a film once, to know how its stills must be taken.
func (s Stills) Open(ctx context.Context, file string) (thumbnail.Film, error) {
	format, known := formats[strings.ToLower(filepath.Ext(file))]
	if !known {
		return nil, fmt.Errorf("%w: %s is not a video the app opens", thumbnail.ErrNoStills, filepath.Ext(file))
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
	return &film{path: s.Path, file: file, format: format, look: look}, nil
}

// describe has FFmpeg describe the file: given no output, it describes its
// input on the error output and ends.
func (s Stills) describe(ctx context.Context, file, format string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, describeTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, s.Path, "-hide_banner", "-nostdin", "-protocol_whitelist", "file", "-f", format, "-i", "file:"+file)
	command.SysProcAttr = background()
	var description bytes.Buffer
	command.Stderr = &limitedWriter{buffer: &description, room: maxDescription}
	if err := start(command); err != nil {
		return "", err
	}
	_ = command.Wait() // it always fails: there is no output
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("ffmpeg: describing the film: %w", err)
	}
	return description.String(), nil
}

// film takes the stills of one video.
type film struct {
	path, file, format string
	look               Look
	// untimed is an FFmpeg older than 6.1, which cannot tell the frame's
	// time: its stills stand for the moment asked.
	untimed atomic.Bool
}

func (f *film) Grab(ctx context.Context, shot thumbnail.Shot) (thumbnail.Picture, error) {
	picture, problems, err := f.grab(ctx, shot, !f.untimed.Load())
	if err != nil && !f.untimed.Load() && strings.Contains(problems, "stats_enc_pre") {
		f.untimed.Store(true)
		picture, _, err = f.grab(ctx, shot, false)
	}
	return picture, err
}

// grab runs one still, and returns what FFmpeg complained about.
func (f *film) grab(ctx context.Context, shot thumbnail.Shot, withTime bool) (thumbnail.Picture, string, error) {
	ctx, cancel := context.WithTimeout(ctx, stillTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, f.path, StillArguments(f.file, f.format, shot.At, f.look, withTime)...)
	command.SysProcAttr = background()
	var told bytes.Buffer
	command.Stderr = &limitedWriter{buffer: &told, room: 4096}
	out, err := command.StdoutPipe()
	if err != nil {
		return thumbnail.Picture{}, "", fmt.Errorf("ffmpeg: %w", err)
	}
	if err := start(command); err != nil {
		return thumbnail.Picture{}, "", err
	}
	still, readErr := io.ReadAll(io.LimitReader(out, maxStill+1))
	if len(still) > maxStill {
		cancel()
		_ = command.Wait()
		return thumbnail.Picture{}, "", errors.New("ffmpeg: the still is larger than any still")
	}
	waitErr := command.Wait()
	switch {
	case ctx.Err() != nil:
		return thumbnail.Picture{}, "", fmt.Errorf("ffmpeg: taking the still at %v: %w", shot.At, ctx.Err())
	case waitErr != nil:
		problems := strings.TrimSpace(told.String())
		return thumbnail.Picture{}, problems, fmt.Errorf("ffmpeg: %w: %s", waitErr, problems)
	case readErr != nil:
		return thumbnail.Picture{}, "", fmt.Errorf("ffmpeg: reading the still: %w", readErr)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(still))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > StillWidth || config.Height > StillHeight {
		return thumbnail.Picture{}, "", fmt.Errorf("ffmpeg: what came back is not a still (%v)", err)
	}
	return thumbnail.Picture{At: frameTime(shot.At, told.String()), Width: config.Width, Height: config.Height, JPEG: still}, "", nil
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
