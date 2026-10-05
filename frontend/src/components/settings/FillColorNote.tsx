import { useEffect, useState } from "react";
import { t } from "../../i18n";
import { backend, type FillCheck } from "../../services/backend";
import { Button } from "../ui/Button";

/** waitBeforeChecking lets a color picker settle before the colors are checked. */
const waitBeforeChecking = 150;

interface FillColorNoteProps {
  /** text is the color of the line on top, the one the fill paints over. */
  text: string;
  fill: string;
  /** enabled is whether the spoken part is painted at all. */
  enabled: boolean;
  onAdopt: (color: string) => void;
}

/**
 * FillColorNote tells when the video paints the spoken part in another color
 * than the one chosen, because the line on top would hide it, and offers to
 * keep that color. Turning the fill off is its own switch.
 */
export function FillColorNote({ text, fill, enabled, onAdopt }: FillColorNoteProps) {
  // The answer keeps the colors it was for, so an old answer never shows for new colors.
  const [answer, setAnswer] = useState<{ text: string; fill: string; check: FillCheck } | null>(null);

  useEffect(() => {
    if (!enabled) {
      return;
    }
    let current = true;
    const timer = window.setTimeout(() => {
      backend
        .checkFill(text, fill)
        .then((check) => current && setAnswer({ text, fill, check }))
        .catch(() => current && setAnswer(null));
    }, waitBeforeChecking);
    return () => {
      current = false;
      window.clearTimeout(timer);
    };
  }, [text, fill, enabled]);

  const check = answer && answer.text === text && answer.fill === fill ? answer.check : null;
  if (!enabled || !check?.changed) {
    return null;
  }
  return (
    <div role="note" className="flex flex-col gap-2 rounded-control border border-line bg-sunken p-3 text-sm text-fg">
      <p className="flex items-center gap-2">
        <span aria-hidden className="size-4 shrink-0 rounded-control border border-line-strong" style={{ backgroundColor: check.used }} />
        {t("ui.settings.fill.standIn", { color: check.used })}
      </p>
      <p className="text-fg-muted">{t("ui.settings.fill.turnOff")}</p>
      <div>
        <Button onClick={() => onAdopt(check.used)}>{t("ui.settings.fill.adopt")}</Button>
      </div>
    </div>
  );
}
