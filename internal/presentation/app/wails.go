package app

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// videoPatterns are the videos the file dialog offers.
const videoPatterns = "*.mkv;*.webm;*.mp4;*.m4v;*.avi;*.mov;*.wmv"

// wailsUI is the real window.
type wailsUI struct {
	ctx context.Context
}

// NewWailsUI connects the API to the Wails window of ctx.
func NewWailsUI(ctx context.Context) UI {
	return wailsUI{ctx: ctx}
}

func (w wailsUI) Emit(event string, data any) {
	runtime.EventsEmit(w.ctx, event, data)
}

func (w wailsUI) OpenVideoDialog() (string, error) {
	return runtime.OpenFileDialog(w.ctx, runtime.OpenDialogOptions{
		Title:   "Abrir vídeo",
		Filters: []runtime.FileFilter{{DisplayName: "Vídeos", Pattern: videoPatterns}},
	})
}

func (w wailsUI) OpenProgramDialog(title string) (string, error) {
	return runtime.OpenFileDialog(w.ctx, runtime.OpenDialogOptions{
		Title:   title,
		Filters: []runtime.FileFilter{{DisplayName: "Programas", Pattern: "*.exe"}},
	})
}

func (w wailsUI) OpenURL(url string) {
	runtime.BrowserOpenURL(w.ctx, url)
}
