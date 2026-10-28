import { Cpu } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { adjustedApart, computerFacts, levelName } from "../../computer";
import { t } from "../../i18n";
import { backend } from "../../services/backend";
import { Button } from "../ui/Button";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import type { SectionProps } from "./section";

/**
 * ComputerCard shows the computer as a game does: what was read of it and its level, and adjusts every setting the
 * level decides again, choices made by hand too, once confirmed.
 */
export function ComputerCard({ view, dirty, run }: Pick<SectionProps, "view" | "dirty" | "run">) {
  const [confirming, setConfirming] = useState(false);
  const [adjusted, setAdjusted] = useState("");
  const confirmation = useRef<HTMLDivElement>(null);
  const computer = view.general.computer;
  const level = levelName(computer.level);

  // The question and the answer show where the button was, even at the
  // bottom of a short window.
  useEffect(() => {
    if (confirming) {
      confirmation.current?.scrollIntoView({ block: "nearest" });
    }
  }, [confirming]);

  return (
    <Card title={t("ui.computer.title")} aside={<HelpTip topic={t("ui.computer.topic")}>{t("ui.computer.help")}</HelpTip>}>
      <div className="flex flex-col gap-3">
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 rounded-control border border-line bg-surface-sunken p-3 text-xs select-text">
          {computerFacts(computer).map((fact) => (
            <div key={fact.label} className="contents">
              <dt className="text-fg-muted">{fact.label}</dt>
              <dd className="break-all text-fg">{fact.value}</dd>
            </div>
          ))}
        </dl>
        {adjustedApart(computer) && (
          <p className="text-xs text-fg-muted">{t("ui.computer.adjustedApart", { level: levelName(computer.adjusted) })}</p>
        )}
        {adjusted && (
          <p role="status" className="text-sm text-fg">
            {adjusted}
          </p>
        )}
        {confirming ? (
          <div ref={confirmation} className="flex flex-col gap-3 rounded-control border border-line bg-surface-sunken p-3">
            <p className="text-sm text-fg">{t("ui.computer.confirm", { level })}</p>
            <div className="flex gap-2">
              <Button
                variant="primary"
                icon={Cpu}
                onClick={() => {
                  setConfirming(false);
                  void run(() => backend.adjustToComputer()).then((worked) => {
                    if (worked) {
                      setAdjusted(t("ui.computer.adjusted", { level }));
                    }
                  });
                }}
              >
                {t("ui.computer.adjust")}
              </Button>
              <Button variant="ghost" onClick={() => setConfirming(false)}>
                {t("ui.common.cancel")}
              </Button>
            </div>
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            <div>
              <Button
                icon={Cpu}
                disabled={dirty}
                onClick={() => {
                  setAdjusted("");
                  setConfirming(true);
                }}
              >
                {t("ui.computer.adjust")}
              </Button>
            </div>
            {dirty && <p className="text-xs text-fg-muted">{t("ui.settings.profile.saveFirst")}</p>}
          </div>
        )}
      </div>
    </Card>
  );
}
