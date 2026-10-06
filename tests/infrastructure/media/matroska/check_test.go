package matroska_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
)

// Opening a video goes through what it really is: a script, a page or a
// program renamed as any video the app opens is refused before a player
// sees it, while a real container's start is enough to pass.
func TestARenamedFileIsNeverOpenedAsAVideo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	put := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	service := analysis.NewService(matroska.FileReader{}, nil)
	preferences := media.Preferences{Primary: "en"}

	script := "alert('hacked'); require('child_process').exec('calc')"
	for _, name := range []string{"script.mkv", "script.webm", "script.mp4", "script.m4v", "script.mov", "script.avi", "script.wmv"} {
		if _, err := service.Analyze(context.Background(), put(name, script), preferences); err == nil {
			t.Errorf("%s opened as a video", name)
		}
	}
	for _, name := range []string{"page.html", "program.exe", "script.js", "run.sh", "run.bat"} {
		if _, err := service.Analyze(context.Background(), put(name, script), preferences); !errors.Is(err, analysis.ErrUnsupportedFile) {
			t.Errorf("%s: %v, want an unsupported file", name, err)
		}
	}

	mp4 := put("real.mp4", "\x00\x00\x00\x18ftypisom\x00\x00\x02\x00isomiso2avc1mp41")
	if _, err := service.Analyze(context.Background(), mp4, preferences); err != nil {
		t.Fatalf("a real MP4's start: %v", err)
	}
	if err := (matroska.FileReader{}).Check(put("tiny.avi", "RIFF")); !errors.Is(err, media.ErrNotTheContainer) {
		t.Fatalf("a 4-byte AVI: %v", err)
	}
}
