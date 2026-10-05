import { describe, expect, it } from "vitest";
import { hiddenVideo, panelDefault, panelMin, panelWidth, videoLayout } from "./stage";

describe("videoLayout", () => {
  it("is the room the header and the panel leave around the video", () => {
    // A 1280×720 window: a 56-pixel header, a 380-pixel panel on the right.
    expect(videoLayout({ left: 0, top: 56, right: 900, bottom: 720 }, 1280, 720)).toEqual({
      visible: true,
      top: 56,
      right: 380,
      bottom: 0,
      left: 0,
    });
  });

  it("rounds, and never gives negative room", () => {
    expect(videoLayout({ left: -2, top: 56.4, right: 1290, bottom: 719.6 }, 1280, 720)).toEqual({
      visible: true,
      top: 56,
      right: 0,
      bottom: 0,
      left: 0,
    });
  });

  it("shows no video for a place with no area or no place", () => {
    expect(videoLayout(null, 1280, 720)).toEqual(hiddenVideo);
    expect(videoLayout({ left: 10, top: 56, right: 10, bottom: 720 }, 1280, 720)).toEqual(hiddenVideo);
    expect(videoLayout({ left: 0, top: Number.NaN, right: 900, bottom: 720 }, 1280, 720)).toEqual(hiddenVideo);
  });
});

describe("panelWidth", () => {
  it("keeps the width between the minimum and half the window", () => {
    expect(panelWidth(400, 1280)).toBe(400);
    expect(panelWidth(100, 1280)).toBe(panelMin);
    expect(panelWidth(1000, 1280)).toBe(640);
  });

  it("starts at the default, and never below the minimum in a narrow window", () => {
    expect(panelWidth(0, 1280)).toBe(panelDefault);
    expect(panelWidth(Number.NaN, 1280)).toBe(panelDefault);
    expect(panelWidth(400, 400)).toBe(panelMin);
  });
});
