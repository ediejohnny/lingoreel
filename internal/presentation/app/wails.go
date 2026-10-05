package app

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
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
		Title:   ptbr.T("dialog.openVideo"),
		Filters: []runtime.FileFilter{{DisplayName: ptbr.T("dialog.videos"), Pattern: videoPatterns}},
	})
}

func (w wailsUI) OpenProgramDialog(title string) (string, error) {
	return runtime.OpenFileDialog(w.ctx, runtime.OpenDialogOptions{
		Title:   title,
		Filters: []runtime.FileFilter{{DisplayName: ptbr.T("dialog.programs"), Pattern: "*.exe"}},
	})
}

func (w wailsUI) SaveSubtitleDialog(name string) (string, error) {
	return runtime.SaveFileDialog(w.ctx, runtime.SaveDialogOptions{
		Title:           ptbr.T("dialog.exportSubtitle"),
		DefaultFilename: name,
		Filters: []runtime.FileFilter{
			{DisplayName: ptbr.T("dialog.srtFiles"), Pattern: "*.srt"},
			{DisplayName: ptbr.T("dialog.assFiles"), Pattern: "*.ass"},
		},
	})
}

func (w wailsUI) OpenURL(url string) {
	runtime.BrowserOpenURL(w.ctx, url)
}

func (w wailsUI) SetTitle(title string) {
	runtime.WindowSetTitle(w.ctx, title)
}

func (w wailsUI) Quit() {
	runtime.Quit(w.ctx)
}
