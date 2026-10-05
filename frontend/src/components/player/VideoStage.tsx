import { useEffect, useRef } from "react";
import { t } from "../../i18n";
import { backend } from "../../services/backend";
import { hiddenVideo, videoLayout } from "../../stage";

/**
 * VideoStage keeps the video's place in the window: the Go side draws the
 * player there, over this black box, and follows it as the header, the
 * panel or the window change. Leaving the screen hides the video.
 */
export function VideoStage() {
  const box = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const element = box.current;
    if (!element) {
      return;
    }
    let frame = 0;
    const place = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const rect = element.getBoundingClientRect();
        // A place the window refused is only lost until the next change.
        backend.setVideoLayout(videoLayout(rect, window.innerWidth, window.innerHeight)).catch(() => undefined);
      });
    };
    const observer = new ResizeObserver(place);
    observer.observe(element);
    window.addEventListener("resize", place);
    place();
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      window.removeEventListener("resize", place);
      backend.setVideoLayout(hiddenVideo).catch(() => undefined);
    };
  }, []);

  return <div ref={box} aria-label={t("ui.video.label")} className="min-h-0 min-w-0 flex-1 bg-black" />;
}
