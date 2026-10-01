// Small pure helpers for what the screens show.

/** speedLabel shows a playback speed as "1,00×". */
export function speedLabel(speed: number): string {
  if (!Number.isFinite(speed) || speed <= 0) {
    return "1,00×";
  }
  return `${speed.toFixed(2).replace(".", ",")}×`;
}

/** progress is the share of the video already played, from 0 to 1. */
export function progress(position: number, duration: number): number {
  if (!Number.isFinite(position) || !Number.isFinite(duration) || duration <= 0) {
    return 0;
  }
  return Math.min(Math.max(position / duration, 0), 1);
}

/** durationLabel shows a length in seconds as "1h29min" or "42min". */
export function durationLabel(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return "";
  }
  const minutes = Math.round(seconds / 60);
  const hours = Math.floor(minutes / 60);
  if (hours === 0) {
    return `${minutes}min`;
  }
  return `${hours}h${String(minutes % 60).padStart(2, "0")}min`;
}

/** isTyping tells whether a key event comes from a field where keys type text. */
export function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  const tag = target.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || target.isContentEditable;
}
