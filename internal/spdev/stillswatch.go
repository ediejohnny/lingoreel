package spdev

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/domain/player"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/discovery"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/player/plugins"
)

// While stills are measured, VLC may play the film beside them, to count
// the pictures it loses meanwhile, against as long a stretch without them.

const (
	// watchReady bounds the wait for VLC to show the film.
	watchReady = 30 * time.Second
	// watchWarmUp is how long the film plays before the stills begin, as
	// the app waits.
	watchWarmUp = 10 * time.Second
	// maxStatus bounds VLC's status answer.
	maxStatus = 1 << 20
)

// vlcStats is what VLC's status tells of the pictures.
type vlcStats struct {
	State string `json:"state"`
	Stats struct {
		Displayed int64 `json:"displayedpictures"`
		Lost      int64 `json:"lostpictures"`
	} `json:"stats"`
}

// vlcWatch is VLC playing the film, its own HTTP interface on the loopback.
type vlcWatch struct {
	command  *exec.Cmd
	address  string
	password string
}

var procSetProcessAffinityMask = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetProcessAffinityMask")

// pin holds a process to the processors in mask (0x3: the first two).
func pin(process windows.Handle, mask uint64) error {
	if ok, _, err := procSetProcessAffinityMask.Call(uintptr(process), uintptr(mask)); ok == 0 {
		return fmt.Errorf("setting the processors: %w", err)
	}
	return nil
}

// pinSelf holds this process, and the FFmpeg it starts after, to mask.
func pinSelf(mask uint64) error { return pin(windows.CurrentProcess(), mask) }

// startVLC plays the film in the installed VLC from a moment, muted, held to
// the processors in mask (0: all).
func startVLC(ctx context.Context, film string, from time.Duration, mask uint64) (*vlcWatch, error) {
	registry, hints, err := plugins.Registry(stderrLogger())
	if err != nil {
		return nil, err
	}
	choice, found := player.Choose(discovery.Detect(discovery.SystemEnvironment{}, hints, nil), registry.Supported, player.VLC)
	if !found || choice.Installation.ID != player.VLC {
		return nil, errors.New("stills: --watch needs VLC installed")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(film)
	if err != nil {
		return nil, err
	}
	w := &vlcWatch{address: "127.0.0.1:" + strconv.Itoa(port), password: hex.EncodeToString(secret)}
	w.command = exec.CommandContext(ctx, choice.Installation.Path, "--extraintf", "http", "--http-host", "127.0.0.1",
		"--http-port", strconv.Itoa(port), "--http-password", w.password, "--no-one-instance", "--stats",
		"--gain=0", "--no-volume-save", "--no-video-title-show", "--no-metadata-network-access",
		"--start-time", strconv.FormatFloat(from.Seconds(), 'f', 3, 64), absolute)
	if err := w.command.Start(); err != nil {
		return nil, fmt.Errorf("stills: starting VLC: %w", err)
	}
	if mask != 0 {
		handle, err := windows.OpenProcess(windows.PROCESS_SET_INFORMATION|windows.PROCESS_QUERY_INFORMATION, false, uint32(w.command.Process.Pid))
		if err == nil {
			err = pin(handle, mask)
			windows.CloseHandle(handle)
		}
		if err != nil {
			w.close()
			return nil, err
		}
	}
	deadline := time.Now().Add(watchReady)
	for {
		stats, err := w.stats(ctx)
		if err == nil && stats.State == "playing" && stats.Stats.Displayed > 0 {
			return w, nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			w.close()
			return nil, fmt.Errorf("stills: VLC never showed the film (%v)", err)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// stats asks VLC how many pictures it showed and lost.
func (w *vlcWatch) stats(ctx context.Context) (vlcStats, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+w.address+"/requests/status.json", nil)
	if err != nil {
		return vlcStats{}, err
	}
	request.SetBasicAuth("", w.password)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return vlcStats{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return vlcStats{}, fmt.Errorf("VLC answered %s", response.Status)
	}
	var stats vlcStats
	err = json.NewDecoder(io.LimitReader(response.Body, maxStatus)).Decode(&stats)
	return stats, err
}

// lost counts the pictures VLC lost over a stretch.
type lost struct{ shown, lost int64 }

func (w *vlcWatch) during(ctx context.Context, stretch func()) (lost, error) {
	before, err := w.stats(ctx)
	if err != nil {
		return lost{}, err
	}
	stretch()
	after, err := w.stats(ctx)
	if err != nil {
		return lost{}, err
	}
	return lost{shown: after.Stats.Displayed - before.Stats.Displayed, lost: after.Stats.Lost - before.Stats.Lost}, nil
}

func (w *vlcWatch) close() {
	_ = w.command.Process.Kill()
	_ = w.command.Wait()
}
