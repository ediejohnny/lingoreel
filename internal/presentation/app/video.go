package app

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ediejohnny/lingoreel/internal/application/analysis"
	"github.com/ediejohnny/lingoreel/internal/domain/media"
	"github.com/ediejohnny/lingoreel/internal/domain/subtitle"
	"github.com/ediejohnny/lingoreel/internal/presentation/ptbr"
)

// minNoteOffset is the least automatic correction worth telling the user.
const minNoteOffset = 0.001

// errBusy refuses a video opened while another one plays.
var errBusy = errors.New("Um vídeo está tocando. Encerre-o (botão Encerrar) para abrir outro.")

// ChooseVideo asks for a video file; an empty path means the user cancelled.
func (a *API) ChooseVideo() (string, error) {
	a.mu.Lock()
	ui := a.ui
	a.mu.Unlock()
	if ui == nil {
		return "", ErrNotReady
	}
	path, err := ui.OpenVideoDialog()
	if err != nil {
		a.logger.Warn("file dialog failed", "error", err)
		return "", errors.New("Não foi possível abrir a janela de escolha de arquivo.")
	}
	return path, nil
}

// OpenVideo reads a video's tracks and the subtitle files next to it, and
// suggests what to watch it with.
func (a *API) OpenVideo(path string) (Video, error) {
	ctx, err := a.context()
	if err != nil {
		return Video{}, err
	}
	if a.current() != nil {
		return Video{}, errBusy
	}
	if !analysis.IsSupported(path) {
		return Video{}, fmt.Errorf("O LingoReel não abre arquivos %s. Use um vídeo .mkv, .mp4, .avi, .mov ou .wmv.", extensionOf(path))
	}
	a.mu.Lock()
	languages := a.profile().Languages
	mode := a.profile().Study.Mode
	a.mu.Unlock()
	preferences := media.Preferences{
		Audio: media.LanguageTag(languages.Audio), Primary: media.LanguageTag(languages.Primary),
		Translation: media.LanguageTag(languages.Translation),
	}
	result, err := a.deps.Service.Analyze(ctx, path, preferences)
	if err != nil {
		a.logger.Warn("video analysis failed", "path", path, "error", err)
		if errors.Is(err, fs.ErrNotExist) {
			return Video{}, fmt.Errorf("O arquivo %s não foi encontrado. Ele pode ter sido movido, renomeado ou apagado.", filepath.Base(path))
		}
		return Video{}, fmt.Errorf("O arquivo %s não parece um vídeo que o LingoReel consiga ler: pode estar corrompido ou incompleto.", filepath.Base(path))
	}
	a.logger.Info("video opened", "name", filepath.Base(path), "duration", result.Info.Duration.Round(time.Second),
		"audio", len(result.Info.TracksOf(media.TrackAudio)), "subtitles", len(result.Info.TracksOf(media.TrackSubtitle)))
	opened := a.loadProject(path, result.Info)
	suggested, mode := opened.remembered(result.Info, result.Suggested, mode)
	a.mu.Lock()
	a.video, a.project = &result, opened
	a.mu.Unlock()

	video := Video{
		Path: path, Name: filepath.Base(path), Duration: result.Info.Duration.Seconds(),
		Suggested: Selection{Audio: suggested.Audio, Primary: suggested.Primary, Translation: suggested.Translation},
		Players:   a.players(),
		Audio:     []Track{}, Subtitles: []Track{},
		Mode: string(mode), Modes: modeOptions(), ProjectNote: opened.note,
		SavedPassages: passagesOf(opened, result.Info.Duration), Resume: resumeOf(opened, result.Info.Duration),
	}
	for _, track := range result.Info.TracksOf(media.TrackAudio) {
		video.Audio = append(video.Audio, audioTrack(track))
	}
	for _, track := range result.Info.TracksOf(media.TrackSubtitle) {
		video.Subtitles = append(video.Subtitles, subtitleTrack(track))
	}
	video.LanguageNote = languageNote(result.Info, suggested, preferences)
	return video, nil
}

// languageNote tells when the video has subtitles but none in a language
// the profile asks for (PRD §52, "idioma não encontrado").
func languageNote(info media.Info, suggested media.Selection, preferences media.Preferences) string {
	readable := 0
	for _, track := range info.TracksOf(media.TrackSubtitle) {
		if track.Readable {
			readable++
		}
	}
	if readable == 0 {
		return ""
	}
	var missing []string
	if suggested.Primary == 0 && preferences.Primary != "" {
		missing = append(missing, ptbr.LanguageName(preferences.Primary))
	}
	if suggested.Translation == 0 && preferences.Translation != "" {
		missing = append(missing, ptbr.LanguageName(preferences.Translation))
	}
	if len(missing) == 0 {
		return ""
	}
	return "Não há legenda em " + strings.Join(missing, " nem em ") +
		" neste vídeo. Escolha outra abaixo, ou coloque um arquivo .srt na pasta do vídeo e abra-o de novo."
}

func extensionOf(path string) string {
	if extension := strings.ToLower(filepath.Ext(path)); extension != "" {
		return extension
	}
	return "sem extensão"
}

func audioTrack(track media.Track) Track {
	parts := []string{capitalized(ptbr.LanguageName(track.Language))}
	if track.Codec != "" {
		parts = append(parts, strings.TrimPrefix(track.Codec, "A_"))
	}
	if track.Name != "" {
		parts = append(parts, track.Name)
	}
	if track.Default {
		parts = append(parts, "padrão")
	}
	return Track{Number: track.Number, Label: strings.Join(parts, " · "), Language: string(track.Language), Readable: true}
}

func subtitleTrack(track media.Track) Track {
	label := capitalized(ptbr.LanguageName(track.Language))
	if track.Name != "" {
		label += " — " + track.Name
	}
	var flags []string
	if track.Forced {
		flags = append(flags, "forçada")
	}
	if track.HearingImpaired {
		flags = append(flags, "para surdos")
	}
	if track.External() {
		flags = append(flags, "arquivo")
	}
	if len(flags) > 0 {
		label += " (" + strings.Join(flags, ", ") + ")"
	}
	result := Track{Number: track.Number, Label: label, Language: string(track.Language), Readable: track.Readable}
	result.Note, result.Warning = subtitleNote(track)
	return result
}

// subtitleNote explains what the app found about a subtitle track.
func subtitleNote(track media.Track) (string, bool) {
	if !track.Readable {
		return "Não dá para usar: legenda em imagem ou num formato que o LingoReel não lê.", true
	}
	alignment := track.Alignment
	switch alignment.Verdict {
	case subtitle.VerdictMatch:
		if math.Abs(alignment.Offset.Seconds()) >= minNoteOffset || math.Abs(alignment.Rate-1) >= 1e-6 {
			return "O tempo foi ajustado sozinho para casar com o vídeo (" + ptbr.Offset(subtitle.Offset(alignment.Offset)) + ").", false
		}
		return "O tempo casa com o vídeo.", false
	case subtitle.VerdictOtherCut:
		return "Parece ser de outra versão do filme (estendida ou Director's Cut): sai de sincronia a cada cena.", true
	case subtitle.VerdictNoMatch:
		return "O tempo não casa com este vídeo: pode ser de outro filme ou de outra versão.", true
	}
	if track.Candidate {
		return "Encontrada numa subpasta; não deu para conferir se é deste vídeo.", true
	}
	return "", false
}

func capitalized(text string) string {
	first, size := utf8.DecodeRuneInString(text)
	if size == 0 {
		return text
	}
	return string(unicode.ToUpper(first)) + text[size:]
}
