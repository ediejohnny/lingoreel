import { Cpu } from "lucide-react";
import { useState } from "react";
import { levelName } from "../computer";
import { useFollowed } from "../hooks/useFollowed";
import { t } from "../i18n";
import { backend, messageOf } from "../services/backend";
import { Button } from "./ui/Button";
import { Banner } from "./ui/Feedback";

/**
 * ComputerChangeBanner asks, when the app runs on a computer of another level than the settings were adjusted to,
 * whether to adjust again; nothing changes without the answer, and choices made by hand stay either way. It goes once
 * the question is answered, here or by the settings' own button.
 */
export function ComputerChangeBanner() {
  const change = useFollowed(backend.computerChange, backend.onComputerChanged);
  const [answering, setAnswering] = useState(false);
  const [error, setError] = useState("");

  if (!change?.asked) {
    return null;
  }
  const answer = (adjust: boolean) => {
    setAnswering(true);
    setError("");
    backend
      .answerComputerChange(adjust)
      .catch((failure) => setError(messageOf(failure)))
      .finally(() => setAnswering(false));
  };

  return (
    <div className="px-6 pt-4">
      <Banner tone="info">
        <div className="flex flex-col gap-3">
          <p>
            <strong>{t("ui.computer.changed.title")}</strong>{" "}
            {t("ui.computer.changed.text", { level: levelName(change.computer.level), adjusted: levelName(change.computer.adjusted) })}
          </p>
          {error && <p className="text-error">{error}</p>}
          <div className="flex flex-wrap gap-2">
            <Button variant="primary" icon={Cpu} disabled={answering} onClick={() => answer(true)}>
              {t("ui.computer.changed.adjust")}
            </Button>
            <Button variant="ghost" disabled={answering} onClick={() => answer(false)}>
              {t("ui.computer.changed.keep")}
            </Button>
          </div>
        </div>
      </Banner>
    </div>
  );
}
