// What the screens tell of the computer the app runs on: what was read of
// it and its level, named as a game names its own.
import { t } from "./i18n";
import type { ComputerView } from "./services/backend";

/** Fact is one thing read of the computer, as a line of the settings shows it. */
export interface Fact {
  label: string;
  value: string;
}

/** levelName names a level ("basic", "medium" or "high") in the interface's language. */
export function levelName(level: string): string {
  switch (level) {
    case "basic":
      return t("ui.computer.level.basic");
    case "high":
      return t("ui.computer.level.high");
    default:
      return t("ui.computer.level.medium");
  }
}

/** computerFacts are what was read of the computer, a part not read said so, and its level. */
export function computerFacts(computer: ComputerView): Fact[] {
  const unread = t("ui.computer.unread");
  return [
    { label: t("ui.computer.processor"), value: computer.processor || unread },
    { label: t("ui.computer.processors"), value: computer.processors > 0 ? String(computer.processors) : unread },
    { label: t("ui.computer.memory"), value: computer.memoryGiB > 0 ? t("ui.computer.gib", { count: computer.memoryGiB }) : unread },
    { label: t("ui.computer.level"), value: levelName(computer.level) },
  ];
}

/** adjustedApart tells the settings were adjusted to a computer of another level than this one. */
export function adjustedApart(computer: ComputerView): boolean {
  return computer.adjusted !== "" && computer.adjusted !== computer.level;
}
