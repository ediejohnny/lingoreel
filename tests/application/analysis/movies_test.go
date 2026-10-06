package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/media/matroska"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/subtitlefile"
)

// movieCeiling is the most a real film may take from a drop on the window to
// the session screen: reading the tracks, the subtitle files and the chosen
// subtitles. The user asked for it in a few seconds.
const movieCeiling = 10 * time.Second

// TestRealMoviesOpenQuickly measures the films in LINGOREEL_MOVIES (the
// movies folder of the developer's machine); without it, it skips.
func TestRealMoviesOpenQuickly(t *testing.T) {
	t.Parallel()
	dir := os.Getenv("LINGOREEL_MOVIES")
	if dir == "" {
		t.Skip("set LINGOREEL_MOVIES to a folder of real films")
	}
	videos, err := filepath.Glob(filepath.Join(dir, "*.mkv"))
	if err != nil || len(videos) == 0 {
		t.Fatalf("no .mkv in %s: %v", dir, err)
	}
	service := analysis.NewService(matroska.FileReader{}, subtitlefile.Files{})
	preferences := media.Preferences{Audio: "en", Primary: "en", Translation: "pt-BR"}
	for _, video := range videos {
		t.Run(strings.TrimSuffix(filepath.Base(video), ".mkv"), func(t *testing.T) {
			began := time.Now()
			result, err := service.Analyze(t.Context(), video, preferences)
			if err != nil {
				t.Fatal(err)
			}
			analyzed := time.Since(began)
			lines := []uint64{result.Suggested.Primary, result.Suggested.Translation}
			loaded, err := service.LoadSubtitles(t.Context(), result, lines, nil)
			if err != nil {
				t.Fatal(err)
			}
			total := time.Since(began)
			events := 0
			for _, track := range loaded {
				if track.Timeline != nil {
					events += len(track.Timeline.Events())
				}
			}
			info, _ := os.Stat(video)
			t.Logf("%.1f GB, %d tracks: analyzed in %v, %d lines loaded, %v in all", float64(info.Size())/(1<<30), len(result.Info.Tracks), analyzed, events, total)
			if total > movieCeiling {
				t.Fatalf("took %v, over %v", total, movieCeiling)
			}
		})
	}
}
