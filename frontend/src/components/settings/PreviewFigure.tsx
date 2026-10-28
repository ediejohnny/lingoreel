import { useEffect, useRef, useState } from "react";
import { messageOf } from "../../services/backend";
import { after } from "../../timing";

/** waitBeforeDrawing lets a slider move before the picture is asked again. */
const waitBeforeDrawing = 150;

/** PictureSize is the picture asked for, in device pixels, and the screen's scale. */
export interface PictureSize {
  width: number;
  height: number;
  scale: number;
}

interface PreviewFigureProps<T> {
  /** request is what the picture shows: another one draws it again. */
  request: T;
  /** draw asks the Go side for the picture of a request at a size. */
  draw: (request: T, size: PictureSize) => Promise<string>;
  alt: string;
  caption: string;
}

/**
 * PreviewFigure shows a picture the Go side draws with the same code as over the video, as wide as it shows on the
 * screen and in 16:9. It is drawn again once the request or the width stop changing; a failure is told in the caption.
 */
export function PreviewFigure<T>({ request, draw, alt, caption }: PreviewFigureProps<T>) {
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
    return after(waitBeforeDrawing, (current) => {
      const scale = window.devicePixelRatio || 1;
      const pixels = Math.round(width * scale);
      draw(request, { width: pixels, height: Math.round((pixels * 9) / 16), scale })
        .then((next) => {
          if (current()) {
            setPicture(next);
            setProblem("");
          }
        })
        .catch((failure) => current() && setProblem(messageOf(failure)));
    });
  }, [request, draw, width]);

  return (
    <figure className="flex flex-col gap-2">
      <div ref={box} className="relative aspect-video w-full overflow-hidden rounded-card border border-line bg-sunken">
        {picture && <img src={picture} alt={alt} className="size-full" />}
      </div>
      <figcaption className={`text-xs ${problem ? "text-error" : "text-fg-muted"}`}>{problem || caption}</figcaption>
    </figure>
  );
}
