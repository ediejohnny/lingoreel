import { useEffect, useState } from "react";
import { isTyping } from "../format";
import { fullscreenOf, panelCommand, translationCommand } from "../keys";
import { backend, type PlayingCommands, type ShortcutSetting } from "../services/backend";

interface VideoKeysOptions {
  /** active is a video playing: the keys work only then. */
  active: boolean;
  /** embedded is the video inside the app's window, which takes the window's own commands too. */
  embedded: boolean;
  /** version changes when the profile's keys may have changed, so they are read again. */
  version: number;
  commands: Pick<PlayingCommands, "runShortcut" | "reveal" | "setFullscreen">;
  onCommand: (command: () => Promise<void>) => void;
}

/**
 * useVideoKeys makes the profile's shortcuts work wherever the app's screen
 * has the keys while a video plays, unless a field has the focus: the panel,
 * the header, or the screen behind the video, which gets the keys back when
 * the window comes back from Alt+Tab. The translation key also hears its
 * release. It returns the keys, which the panel names on its buttons.
 */
export function useVideoKeys({ active, embedded, version, commands, onCommand }: VideoKeysOptions): ShortcutSetting[] {
  const [keys, setKeys] = useState<ShortcutSetting[]>([]);
  useEffect(() => {
    if (!active) {
      return;
    }
    let current = true;
    backend
      .shortcuts()
      .then((list) => current && setKeys(list))
      .catch(() => current && setKeys([]));
    return () => {
      current = false;
    };
  }, [active, version]);

  useEffect(() => {
    if (!active) {
      return;
    }
    let revealCode = "";
    const release = () => {
      if (revealCode !== "") {
        revealCode = "";
        onCommand(() => commands.reveal(false));
      }
    };
    const down = (event: KeyboardEvent) => {
      if (isTyping(event.target)) {
        return;
      }
      const command = panelCommand(event, keys, embedded);
      if (command === "") {
        return;
      }
      event.preventDefault();
      const fullscreen = fullscreenOf(command);
      if (fullscreen !== undefined) {
        onCommand(() => commands.setFullscreen(fullscreen));
        return;
      }
      if (command === translationCommand) {
        if (!event.repeat) {
          revealCode = event.code;
          onCommand(() => commands.reveal(true));
        }
        return;
      }
      onCommand(() => commands.runShortcut(command, event.repeat));
    };
    const up = (event: KeyboardEvent) => {
      if (revealCode !== "" && event.code === revealCode) {
        // Also keeps the right Alt from opening the window menu.
        event.preventDefault();
        release();
      }
    };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    window.addEventListener("blur", release);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
      window.removeEventListener("blur", release);
      release();
    };
  }, [active, keys, commands, onCommand, embedded]);

  return keys;
}
