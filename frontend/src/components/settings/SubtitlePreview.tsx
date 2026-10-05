import { useEffect, useRef, useState } from "react";
import { t } from "../../i18n";
import {
  backend,
  messageOf,
  type KaraokeSettings,
  type LayoutSettings,
  type MarkSettings,
  type StyleSettings,
} from "../../services/backend";
import type { PreviewView, SubtitleLine } from "../../settings";

/** waitBeforeDrawing lets a slider move before the picture is asked again. */
const waitBeforeDrawing = 150;

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
}

/**
 * SubtitlePreview shows the two lines as the video will, drawn by the same
 * renderer as the overlay, at the picture's real size on screen.
 */
export function SubtitlePreview({ primary, translation, layout, highlight, view = "", karaoke, marks }: SubtitlePreviewProps) {
  const box = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0);
  const [picture, setPicture] = useState("");
  const [problem, setProblem] = useState("");

  useEffect(() => {
    const element = box.current;
    if (!element) {
      return;
    }
    const observer = new ResizeObserver(([entry]) => entry && setWidth(Math.round(entry.contentRect.width)));
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (width < 16) {
      return;
    }
    let current = true;
    const timer = window.setTimeout(() => {
      const scale = window.devicePixelRatio || 1;
      const pixels = Math.round(width * scale);
      backend
        .previewSubtitles({
          primary,
          translation,
          layout,
          width: pixels,
          height: Math.round((pixels * 9) / 16),
          scale,
          highlight,
          view,
          karaoke,
          marks,
        })
        .then((next) => {
          if (current) {
            setPicture(next);
            setProblem("");
          }
        })
        .catch((failure) => current && setProblem(messageOf(failure)));
    }, waitBeforeDrawing);
    return () => {
      current = false;
      window.clearTimeout(timer);
    };
  }, [primary, translation, layout, highlight, view, karaoke, marks, width]);

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
  return (
    <figure className="flex flex-col gap-2">
      <div ref={box} className="relative aspect-video w-full overflow-hidden rounded-card border border-line bg-sunken">
        {picture && <img src={picture} alt={alt} className="size-full" />}
      </div>
      <figcaption className={`text-xs ${problem ? "text-error" : "text-fg-muted"}`}>{problem || caption}</figcaption>
    </figure>
  );
}
