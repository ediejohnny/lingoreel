import { useEffect, useRef, useState } from "react";
import { backend, messageOf, type LayoutSettings, type StyleSettings } from "../../services/backend";
import type { SubtitleLine } from "../../settings";

/** waitBeforeDrawing lets a slider move before the picture is asked again. */
const waitBeforeDrawing = 150;

interface SubtitlePreviewProps {
  primary: StyleSettings;
  translation: StyleSettings;
  layout: LayoutSettings;
  /** highlight outlines the line being edited; "" outlines none. */
  highlight: SubtitleLine | "";
}

/**
 * SubtitlePreview shows the two lines as the video will, drawn by the same
 * renderer as the overlay, at the picture's real size on screen.
 */
export function SubtitlePreview({ primary, translation, layout, highlight }: SubtitlePreviewProps) {
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
        .previewSubtitles({ primary, translation, layout, width: pixels, height: Math.round((pixels * 9) / 16), scale, highlight })
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
  }, [primary, translation, layout, highlight, width]);

  const marked =
    highlight === "primary" ? " A legenda de cima está marcada." : highlight === "translation" ? " A legenda de baixo está marcada." : "";
  return (
    <figure className="flex flex-col gap-2">
      <div ref={box} className="relative aspect-video w-full overflow-hidden rounded-card border border-line bg-sunken">
        {picture && <img src={picture} alt="Prévia das legendas: “This is a preview.” e “Esta é uma prévia.”" className="size-full" />}
      </div>
      <figcaption className={`text-xs ${problem ? "text-error" : "text-fg-muted"}`}>
        {problem || `Prévia num vídeo deste tamanho. Ela acompanha as mudanças antes de salvar.${marked}`}
      </figcaption>
    </figure>
  );
}
