import { describe, expect, it } from "vitest";
import { keepDropsInTheApp } from "./drop";

describe("keepDropsInTheApp", () => {
  it("leaves nothing dragged onto the window for the WebView to open", () => {
    const window = new EventTarget();
    keepDropsInTheApp(window);
    for (const type of ["dragover", "drop"]) {
      const event = new Event(type, { cancelable: true });
      window.dispatchEvent(event);
      expect(event.defaultPrevented, type).toBe(true);
    }
  });
});
