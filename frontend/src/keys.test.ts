import { describe, expect, it } from "vitest";
import { chordOf, keyLabel, panelCommand, withKey, type CommandKey, type KeyPress } from "./keys";

function press(code: string, held: Partial<KeyPress> = {}): KeyPress {
  return { code, ctrlKey: false, shiftKey: false, altKey: false, metaKey: false, ...held };
}

describe("chordOf", () => {
  it("writes the chord as the Go side reads it, by the key's place", () => {
    expect(chordOf(press("KeyP"))).toBe("KeyP");
    expect(chordOf(press("Comma", { shiftKey: true }))).toBe("Shift+Comma");
    expect(chordOf(press("ArrowLeft", { altKey: true, ctrlKey: true, shiftKey: true }))).toBe("Ctrl+Alt+Shift+ArrowLeft");
    expect(chordOf(press("Escape"))).toBe("Escape");
  });

  it("drops the Ctrl that AltGr adds to the right Alt", () => {
    expect(chordOf(press("AltRight", { ctrlKey: true, altKey: true }))).toBe("AltRight");
    expect(chordOf(press("AltRight", { shiftKey: true }))).toBe("AltRight");
  });

  it("gives no chord for a modifier alone or the Windows key", () => {
    for (const code of ["ControlLeft", "ShiftRight", "AltLeft", "MetaLeft", "CapsLock", ""]) {
      expect(chordOf(press(code, { ctrlKey: true }))).toBe("");
    }
    expect(chordOf(press("KeyE", { metaKey: true }))).toBe("");
  });
});

describe("panelCommand", () => {
  const keys: CommandKey[] = [
    { command: "play-pause", chord: "Space" },
    { command: "seek-back-10", chord: "Shift+ArrowLeft" },
    { command: "translation", chord: "AltRight" },
    { command: "fullscreen", chord: "KeyF" },
    { command: "exit-fullscreen", chord: "Escape" },
    { command: "show-both", chord: "" },
    { command: "primary-only", chord: "" },
  ];

  it("finds the command of the profile's chord", () => {
    expect(panelCommand(press("Space"), keys)).toBe("play-pause");
    expect(panelCommand(press("ArrowLeft", { shiftKey: true }), keys)).toBe("seek-back-10");
    expect(panelCommand(press("AltRight", { ctrlKey: true, altKey: true }), keys)).toBe("translation");
  });

  it("leaves other keys, the window's commands and the commands without a key alone", () => {
    expect(panelCommand(press("ArrowLeft"), keys)).toBe("");
    expect(panelCommand(press("KeyF"), keys)).toBe("");
    expect(panelCommand(press("Escape"), keys)).toBe("");
    expect(panelCommand(press("ShiftLeft", { shiftKey: true }), keys)).toBe("");
    expect(panelCommand(press("Space", { metaKey: true }), keys)).toBe("");
    expect(panelCommand(press("Space"), [])).toBe("");
  });
});

describe("labels", () => {
  const keys = [
    { command: "play-pause", label: "Espaço" },
    { command: "show-both", label: "" },
  ];

  it("names a command's keys, or none", () => {
    expect(keyLabel(keys, "play-pause")).toBe("Espaço");
    expect(keyLabel(keys, "show-both")).toBe("");
    expect(keyLabel(keys, "unknown")).toBe("");
    expect(withKey("Pausar", "Espaço")).toBe("Pausar (Espaço)");
    expect(withKey("Mostrar as duas legendas", "")).toBe("Mostrar as duas legendas");
  });
});
