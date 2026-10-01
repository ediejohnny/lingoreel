package spdev

import (
	"fmt"
	"io"

	"lingoreel/internal/domain/layout"
	"lingoreel/internal/infrastructure/windows/win32"
)

// diag prints what decides where the player and the subtitles go on this
// machine: every monitor with its size, work area and scaling, and whether
// spdev sees physical pixels. It guides the manual check on other machines
// (several monitors, 125–200% scaling).
func diag(stdout io.Writer) error {
	out := &printer{w: stdout}
	found, err := win32.Monitors()
	if err != nil {
		return fmt.Errorf("listando os monitores: %w", err)
	}
	out.line("Monitores: %d", len(found))
	for i, monitor := range found {
		primary := ""
		if monitor.Primary {
			primary = " (principal)"
		}
		b, w := monitor.Bounds, monitor.WorkArea
		out.line("  %d%s: %d×%d em (%d, %d); área útil %d×%d em (%d, %d); escala %d%% (%d DPI)",
			i+1, primary, b.Right-b.Left, b.Bottom-b.Top, b.Left, b.Top,
			w.Right-w.Left, w.Bottom-w.Top, w.Left, w.Top, monitor.DPI*100/layout.BaseDPI, monitor.DPI)
	}
	aware := "não (os tamanhos acima podem estar reduzidos pela escala)"
	if win32.ThreadUsesPerMonitorDPI() {
		aware = "sim (pixels físicos)"
	}
	out.line("Ciente de DPI por monitor: %s", aware)
	out.line("")
	out.line("Roteiro de conferência nesta máquina:")
	out.line("  1. spdev overlay \"<vídeo.mkv>\": a janela do player abre inteira dentro do monitor principal.")
	out.line("  2. Arraste o player para cada monitor: as legendas acompanham e mantêm o tamanho proporcional.")
	out.line("  3. Mude a escala do Windows (125%%, 150%%) com o vídeo aberto: as legendas se ajustam sem reiniciar.")
	out.line("  4. Tela cheia em cada monitor: as legendas continuam por cima do vídeo.")
	out.line("  5. Anote o que não bater e a saída deste comando.")
	return out.err
}
