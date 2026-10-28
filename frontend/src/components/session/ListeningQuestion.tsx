import { Hourglass, Play } from "../ui/icons";
import { t } from "../../i18n";
import type { Listening } from "../../services/backend";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { ProgressBar } from "../ui/ProgressBar";

interface ListeningQuestionProps {
  listening: Listening;
  /** waiting tells the person chose to wait: the film starts on its own once the hearing ends. */
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
  return (
    <Dialog title={t("ui.session.listening.title")} onCancel={onCancel}>
      <p className="text-sm leading-relaxed text-fg">{waiting ? t("ui.session.listening.waiting") : t("ui.session.listening.question")}</p>
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
    </Dialog>
  );
}
