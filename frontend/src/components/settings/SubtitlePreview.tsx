import { useMemo } from "react";
import { t } from "../../i18n";
import {
  backend,
  type KaraokeSettings,
  type LanguageSettings,
  type LayoutSettings,
  type MarkSettings,
  type PreviewRequest,
  type StyleSettings,
} from "../../services/backend";
import type { PreviewView, SubtitleLine } from "../../settings";
import { PreviewFigure, type PictureSize } from "./PreviewFigure";

interface SubtitlePreviewProps {
  primary: StyleSettings;
  translation: StyleSettings;
  layout: LayoutSettings;
  /** highlight outlines the line being edited, as marks say; "" outlines none. */
  highlight: SubtitleLine | "";
  /**
   * view "fill" paints half the line on top in the karaoke's fill color, as while the actor speaks; "cue" shows
   * it before its time with the karaoke's indicator; "notice" adds a notice as marks say.
   */
  view?: PreviewView;
  karaoke: KaraokeSettings;
  marks: MarkSettings;
  /** languages are the profile's: each sample line speaks its line's. */
  languages: LanguageSettings;
}

type LinesRequest = Omit<PreviewRequest, keyof PictureSize>;

const drawLines = (request: LinesRequest, size: PictureSize) => backend.previewSubtitles({ ...request, ...size });

/**
 * SubtitlePreview shows the two lines as the video will, drawn by the same
 * renderer as the overlay, at the picture's real size on screen.
 */
export function SubtitlePreview({ primary, translation, layout, highlight, view = "", karaoke, marks, languages }: SubtitlePreviewProps) {
  // The languages as plain values: another draft of the same languages draws nothing again.
  const { audio: audioLanguage, primary: primaryLanguage, translation: translationLanguage } = languages;
  const request = useMemo<LinesRequest>(
    () => ({
      primary,
      translation,
      layout,
      highlight,
      view,
      karaoke,
      marks,
      languages: { audio: audioLanguage, primary: primaryLanguage, translation: translationLanguage },
    }),
    [primary, translation, layout, highlight, view, karaoke, marks, audioLanguage, primaryLanguage, translationLanguage],
  );

  const marked =
    highlight === "primary" ? t("ui.settings.preview.markedTop") : highlight === "translation" ? t("ui.settings.preview.markedBottom") : "";
  const scene =
    view === "fill"
      ? t("ui.settings.preview.filled")
      : view === "cue"
        ? t("ui.settings.preview.cued")
        : view === "notice"
          ? t("ui.settings.preview.noticed")
          : "";
  const caption = [t("ui.settings.preview.caption"), marked, scene].filter((sentence) => sentence !== "").join(" ");
  const alt =
    view === "fill"
      ? t("ui.settings.preview.altFill")
      : view === "cue"
        ? t("ui.settings.preview.altCue")
        : view === "notice"
          ? t("ui.settings.preview.altNotice")
          : t("ui.settings.preview.alt");
  return <PreviewFigure request={request} draw={drawLines} alt={alt} caption={caption} />;
}
