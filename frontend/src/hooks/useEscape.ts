import { useEffect, useRef } from "react";

/**
 * useEscape calls onEscape when Esc is pressed anywhere in the window, unless something already used the key. A modal
 * one hears the key before everything else and keeps it: nothing behind a dialog acts on it, such as the video's own
 * keys.
 */
export function useEscape(onEscape: () => void, { modal = false } = {}): void {
  const latest = useRef(onEscape);
  useEffect(() => {
    latest.current = onEscape;
  });
  useEffect(() => {
    const listener = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented) {
        return;
      }
      event.preventDefault();
      if (modal) {
        event.stopPropagation();
      }
      latest.current();
    };
    window.addEventListener("keydown", listener, modal);
    return () => window.removeEventListener("keydown", listener, modal);
  }, [modal]);
}
