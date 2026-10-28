// Pure helpers of the video inside the app's window: the room the screen's
// controls leave it, and the width of the panel beside it. A VideoLayout is
// what the Go side needs to place the video: whether it shows, and the room
// around it, in CSS pixels.

import type { VideoLayout } from "./services/backend";

/** hiddenVideo is the layout of a screen that shows no video. */
export const hiddenVideo: VideoLayout = { visible: false, top: 0, right: 0, bottom: 0, left: 0 };

/** Box is where an element sits in the window, in CSS pixels. */
interface Box {
  left: number;
  top: number;
  right: number;
  bottom: number;
}

const around = (value: number) => (Number.isFinite(value) ? Math.max(0, Math.round(value)) : 0);

/**
 * videoLayout is the room around the video's place in a window of width ×
 * height: what the header, the panel and the other controls take. A place
 * with no area shows no video.
 */
export function videoLayout(box: Box | null, width: number, height: number): VideoLayout {
  if (!box || !(box.right - box.left > 0) || !(box.bottom - box.top > 0)) {
    return hiddenVideo;
  }
  return {
    visible: true,
    top: around(box.top),
    right: around(width - box.right),
    bottom: around(height - box.bottom),
    left: around(box.left),
  };
}

/** The panel's width: at least panelMin, at most half the window, panelDefault when not chosen yet (0). */
export const panelMin = 280;
export const panelDefault = 380;

/** panelWidth keeps a width the panel can have in a window this wide. */
export function panelWidth(asked: number, windowWidth: number): number {
  const widest = Math.max(panelMin, Math.floor(windowWidth / 2));
  const width = Number.isFinite(asked) && asked > 0 ? asked : panelDefault;
  return Math.round(Math.min(Math.max(width, panelMin), widest));
}
