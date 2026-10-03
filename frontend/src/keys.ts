// Pure helpers of the keyboard shortcuts: reading a key press as a chord, as
// the Go side writes them, and what a press does on the app's panel.

/** The key codes of the modifiers: alone they make no shortcut. */
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

/** isModifier tells a modifier key alone, or a key without a code. */
export function isModifier(code: string): boolean {
  return code === "" || modifierCodes.has(code);
}

/**
 * chordOf reads a key going down as the Go side writes chords
 * ("Ctrl+Shift+Comma"): keys by their place (code), so Shift+, stays
 * Shift+Comma on ABNT2. The right Alt is a key of its own: AltGr adds a Ctrl
 * to it, which is dropped. A modifier alone, or anything with the Windows
 * key, is no chord: "".
 */
export function chordOf(press: KeyPress): string {
  if (press.code === "AltRight") {
    return "AltRight";
  }
  if (isModifier(press.code) || press.metaKey) {
    return "";
  }
  const parts: string[] = [];
  if (press.ctrlKey) parts.push("Ctrl");
  if (press.altKey) parts.push("Alt");
  if (press.shiftKey) parts.push("Shift");
  parts.push(press.code);
  return parts.join("+");
}

/** A command and its chord, "" when it has no shortcut. */
export interface CommandKey {
  command: string;
  chord: string;
}

/** The translation key: shown on its press and, held, hidden on its release. */
export const translationCommand = "translation";

/** The commands the player window keeps for itself. */
const windowCommands = new Set(["fullscreen", "exit-fullscreen"]);

/**
 * panelCommand is the command a key going down runs on the panel, or ""
 * when none: a chord no command has, or a command of the player window.
 */
export function panelCommand(press: KeyPress, keys: CommandKey[]): string {
  const chord = chordOf(press);
  if (chord === "") {
    return "";
  }
  const found = keys.find((key) => key.chord === chord);
  if (!found || windowCommands.has(found.command)) {
    return "";
  }
  return found.command;
}

/** keyLabel is a command's keys as shown ("Shift+,"), or "" without a shortcut. */
export function keyLabel(keys: { command: string; label: string }[], command: string): string {
  return keys.find((key) => key.command === command)?.label ?? "";
}

/** withKey adds a command's keys to a text: "Pausar (Espaço)", or the text alone. */
export function withKey(text: string, label: string): string {
  return label === "" ? text : `${text} (${label})`;
}
