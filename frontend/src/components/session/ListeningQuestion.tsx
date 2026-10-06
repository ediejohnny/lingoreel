import { Hourglass, Play } from "lucide-react";
import { useEffect } from "react";
import { t } from "../../i18n";
import type { Listening } from "../../services/backend";
import { Button } from "../ui/Button";
import { ProgressBar } from "../ui/ProgressBar";

interface ListeningQuestionProps {
  listening: Listening;
  /** waiting tells "Aguardar" was chosen: the film starts on its own once the hearing ends. */
  waiting: boolean;
  onWait: () => void;
  onWatchNow: () => void;
  onCancel: () => void;
}

/**
 * ListeningQuestion asks, when Watch is pressed while the app still hears the film to choose its subtitle, whether to
 * wait for the best subtitle or start now with the one chosen; waiting, it shows how far the hearing went.
 */
export function ListeningQuestion({ listening, waiting, onWait, onWatchNow, onCancel }: ListeningQuestionProps) {
  useEffect(() => {
    const close = (event: KeyboardEvent) => event.key === "Escape" && onCancel();
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [onCancel]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4" onClick={onCancel}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="listening-title"
        className="flex w-full max-w-md flex-col gap-4 rounded-card border border-line bg-surface p-5 shadow-card"
        onClick={(event) => event.stopPropagation()}
      >
        <h2 id="listening-title" className="text-base font-semibold text-fg">
          {t("ui.session.listening.title")}
        </h2>
        <p className="text-sm leading-relaxed text-fg">
          {waiting ? t("ui.session.listening.waiting") : t("ui.session.listening.question")}
        </p>
        <ProgressBar
          label={t("ui.session.listening.label", { stretch: listening.stretch, stretches: listening.stretches })}
          percent={listening.percent}
        />
        <div className="flex flex-wrap justify-end gap-2">
          {waiting ? (
            <Button variant="ghost" onClick={onCancel}>
              {t("ui.common.cancel")}
            </Button>
          ) : (
            <Button variant="primary" icon={Hourglass} onClick={onWait}>
              {t("ui.session.listening.wait")}
            </Button>
          )}
          <Button variant={waiting ? "primary" : "secondary"} icon={Play} onClick={onWatchNow}>
            {t("ui.session.listening.now")}
          </Button>
        </div>
      </div>
    </div>
  );
}
