// Pure helpers of the settings screen: capturing a shortcut, colors with
// transparency and telling unsaved changes.

/** The key codes of the modifiers: alone they make no shortcut yet. */
const modifierCodes = new Set([
  "ControlLeft",
  "ControlRight",
  "ShiftLeft",
  "ShiftRight",
  "AltLeft",
  "MetaLeft",
  "MetaRight",
  "OSLeft",
  "OSRight",
  "CapsLock",
  "NumLock",
  "ScrollLock",
]);

export interface KeyPress {
  code: string;
  ctrlKey: boolean;
  shiftKey: boolean;
  altKey: boolean;
  metaKey: boolean;
}

export type Capture =
  | { kind: "wait" }
  | { kind: "cancel" }
  | { kind: "unsupported" }
  | { kind: "chord"; chord: string };

/**
 * captureOf reads a key going down while a shortcut is captured, as the Go
 * side writes chords ("Ctrl+Shift+Comma"): keys by their place (code), so
 * Shift+, stays Shift+Comma on ABNT2. Modifiers alone wait for a key; Esc
 * alone cancels; the Windows key is not allowed. The right Alt is a key of
 * its own: AltGr adds a Ctrl to it, which is dropped.
 */
export function captureOf(press: KeyPress): Capture {
  if (press.code === "AltRight") {
    return { kind: "chord", chord: "AltRight" };
  }
  if (press.code === "" || modifierCodes.has(press.code)) {
    return press.metaKey ? { kind: "unsupported" } : { kind: "wait" };
  }
  if (press.metaKey) {
    return { kind: "unsupported" };
  }
  if (press.code === "Escape" && !press.ctrlKey && !press.shiftKey && !press.altKey) {
    return { kind: "cancel" };
  }
  const parts: string[] = [];
  if (press.ctrlKey) parts.push("Ctrl");
  if (press.altKey) parts.push("Alt");
  if (press.shiftKey) parts.push("Shift");
  parts.push(press.code);
  return { kind: "chord", chord: parts.join("+") };
}

/** splitColor separates "#RRGGBB[AA]" into its color and its opacity (0 to 1). */
export function splitColor(color: string): { hex: string; alpha: number } {
  const match = /^#([0-9a-fA-F]{6})([0-9a-fA-F]{2})?$/.exec(color.trim());
  if (!match) {
    return { hex: "#000000", alpha: 1 };
  }
  const alpha = match[2] === undefined ? 1 : parseInt(match[2], 16) / 255;
  return { hex: `#${(match[1] ?? "000000").toUpperCase()}`, alpha };
}

/** joinColor writes a color and an opacity as the Go side reads them. */
export function joinColor(hex: string, alpha: number): string {
  const base = splitColor(hex).hex;
  const clamped = Number.isFinite(alpha) ? Math.min(Math.max(alpha, 0), 1) : 1;
  const byte = Math.round(clamped * 255);
  if (byte === 255) {
    return base;
  }
  return base + byte.toString(16).toUpperCase().padStart(2, "0");
}

/** sameSettings tells whether two drafts hold the same choices. */
export function sameSettings(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

/** clampNumber keeps a typed number inside its range; text that is no number keeps the old value. */
export function clampNumber(text: string, min: number, max: number, fallback: number): number {
  const value = Number(text.replace(",", "."));
  if (text.trim() === "" || !Number.isFinite(value)) {
    return fallback;
  }
  return Math.min(Math.max(value, min), max);
}
