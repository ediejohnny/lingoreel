// Where a floating note opens beside the control it explains.

/** margin is the room kept between a note and the window's edges, and between it and its control. */
const margin = 8;

/**
 * fitsAbove tells whether a note of the given height opens above its control (top and bottom of the control, in
 * window pixels): only when below it would pass the window's bottom and above it fits.
 */
export function fitsAbove(top: number, bottom: number, height: number, windowHeight: number): boolean {
  const fitsBelow = bottom + margin + height <= windowHeight - margin;
  const fitsOver = top - margin - height >= margin;
  return !fitsBelow && fitsOver;
}
