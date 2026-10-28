// Small pure helpers for what the screens show; numbers carry the
// interface's decimal separator.
import { decimal, t } from "./i18n";

/** speedLabel shows a playback speed as "1,00×" ("1.00×" in English). */
export function speedLabel(speed: number): string {
  const shown = !Number.isFinite(speed) || speed <= 0 ? 1 : speed;
  return `${shown.toFixed(2).replace(".", decimal())}×`;
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
    return t("ui.format.minutes", { minutes });
  }
  return t("ui.format.hoursMinutes", { hours, minutes: String(minutes % 60).padStart(2, "0") });
}

/** sizeLabel shows a file's size in whole megabytes as Windows counts them (1024 × 1024 bytes), "339 MB". */
export function sizeLabel(bytes: number): string {
  const megabytes = Number.isFinite(bytes) && bytes > 0 ? Math.max(1, Math.round(bytes / (1024 * 1024))) : 0;
  return t("ui.format.megabytes", { size: megabytes });
}

/** isTyping tells whether a key event comes from a field where keys type text. */
export function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  const tag = target.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || target.isContentEditable;
}

/**
 * passageClock shows a time for the passage fields as the Go side reads it
 * back: h:mm:ss or m:ss, with tenths when it is not a whole second.
 */
export function passageClock(seconds: number): string {
  const tenthsTotal = Number.isFinite(seconds) && seconds > 0 ? Math.round(seconds * 10) : 0;
  const tenths = tenthsTotal % 10;
  const clock = wholeClock(Math.floor(tenthsTotal / 10));
  return tenths > 0 ? `${clock}${decimal()}${tenths}` : clock;
}

/**
 * editClock shows a time for the subtitle editor to the millisecond,
 * "1:02:03,456" or "2:03,456", as the Go side shows and reads it.
 */
export function editClock(seconds: number): string {
  const millisTotal = Number.isFinite(seconds) && seconds > 0 ? Math.round(seconds * 1000) : 0;
  const millis = String(millisTotal % 1000).padStart(3, "0");
  return `${wholeClock(Math.floor(millisTotal / 1000))}${decimal()}${millis}`;
}

/** wholeClock shows whole seconds as h:mm:ss, or m:ss below an hour. */
function wholeClock(whole: number): string {
  const hours = Math.floor(whole / 3600);
  const minutes = Math.floor((whole % 3600) / 60);
  const rest = whole % 60;
  const pad = (value: number) => String(value).padStart(2, "0");
  return hours > 0 ? `${hours}:${pad(minutes)}:${pad(rest)}` : `${minutes}:${pad(rest)}`;
}
