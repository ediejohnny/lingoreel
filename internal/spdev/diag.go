package spdev

import (
	"fmt"
	"io"

	"github.com/ediejohnny/lingoreel/internal/domain/layout"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// diag prints what decides where the player and the subtitles go on this
// machine: every monitor with its size, work area and scaling, and whether
// spdev sees physical pixels. It guides the manual check on other machines
// (several monitors, 125–200% scaling).
func diag(stdout io.Writer) error {
	out := &printer{w: stdout}
	found, err := win32.Monitors()
	if err != nil {
		return fmt.Errorf("listing the monitors: %w", err)
	}
	out.line("Monitors: %d", len(found))
	for i, monitor := range found {
		primary := ""
		if monitor.Primary {
			primary = " (primary)"
		}
		b, w := monitor.Bounds, monitor.WorkArea
		out.line("  %d%s: %d×%d at (%d, %d); work area %d×%d at (%d, %d); scaling %d%% (%d DPI)",
			i+1, primary, b.Right-b.Left, b.Bottom-b.Top, b.Left, b.Top,
			w.Right-w.Left, w.Bottom-w.Top, w.Left, w.Top, monitor.DPI*100/layout.BaseDPI, monitor.DPI)
	}
	aware := "no (the sizes above may be shrunk by the scaling)"
	if win32.ThreadUsesPerMonitorDPI() {
		aware = "yes (physical pixels)"
	}
	out.line("Per-monitor DPI aware: %s", aware)
	out.line("")
	out.line("Checklist on this machine:")
	out.line("  1. spdev overlay \"<video.mkv>\": the player's window opens whole inside the primary monitor.")
	out.line("  2. Drag the player to each monitor: the subtitles follow it and keep their size in proportion.")
	out.line("  3. Change the Windows scaling (125%%, 150%%) with the video open: the subtitles adjust without a restart.")
	out.line("  4. Fullscreen on each monitor: the subtitles stay over the video.")
	out.line("  5. Note what does not match, with this command's output.")
	return out.err
}
