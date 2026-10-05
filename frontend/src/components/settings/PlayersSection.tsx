import { FolderOpen, RefreshCw, RotateCcw } from "lucide-react";
import { t } from "../../i18n";
import { backend } from "../../services/backend";
import { Button } from "../ui/Button";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import { Select } from "../ui/Select";
import { change, type SectionProps } from "./section";

/** PlayersSection holds the preferred player and where each one is. */
export function PlayersSection({ view, draft, setDraft, run }: SectionProps) {
  const supported = view.players.filter((item) => item.supported);
  const others = view.players.filter((item) => !item.supported);

  return (
    <>
      <Card title={t("ui.settings.players.preferred.title")}>
        <Select
          label={t("ui.settings.players.preferred.label")}
          value={draft.preferredPlayer}
          options={[
            { value: "", label: t("ui.settings.players.preferred.auto") },
            ...supported.map((item) => ({ value: item.id, label: item.name })),
          ]}
          onChange={(id) => change(setDraft, "preferredPlayer")(id)}
          help={<HelpTip topic={t("ui.settings.players.preferred.topic")}>{t("ui.settings.players.preferred.help")}</HelpTip>}
        />
      </Card>
      <Card
        title={t("ui.settings.players.where.title")}
        aside={<HelpTip topic={t("ui.settings.players.where.topic")}>{t("ui.settings.players.where.help")}</HelpTip>}
      >
        <ul className="flex flex-col divide-y divide-line">
          {supported.map((item) => (
            <li key={item.id} className="flex flex-wrap items-center gap-3 py-3">
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-fg">{item.name}</p>
                <p className="truncate text-xs text-fg-muted" title={item.path}>
                  {item.path
                    ? item.custom
                      ? t("ui.settings.players.custom", { path: item.path })
                      : t("ui.settings.players.found", { path: item.path })
                    : t("ui.settings.players.notFound")}
                </p>
              </div>
              <Button icon={FolderOpen} onClick={() => void run(() => backend.choosePlayerProgram(item.id))}>
                {t("ui.settings.players.choose")}
              </Button>
              {item.custom && (
                <Button
                  icon={RotateCcw}
                  variant="ghost"
                  onClick={() => void run(() => backend.clearPlayerPath(item.id), t("ui.settings.players.searchAgain"))}
                >
                  {t("ui.settings.players.search")}
                </Button>
              )}
            </li>
          ))}
          {others.map((item) => (
            <li key={item.id} className="flex items-center gap-3 py-3">
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-fg">{item.name}</p>
                <p className="text-xs text-fg-muted">{t("ui.settings.players.soon")}</p>
              </div>
            </li>
          ))}
        </ul>
        <div className="mt-4 flex flex-wrap gap-2">
          <Button icon={RefreshCw} onClick={() => void run(() => backend.players(), t("ui.settings.players.detected"))}>
            {t("ui.settings.players.detect")}
          </Button>
          {view.general.recommendationsHidden && (
            <Button
              variant="ghost"
              onClick={() => void run(() => backend.showRecommendations(), t("ui.settings.players.recommendationsBack"))}
            >
              {t("ui.settings.players.showRecommendations")}
            </Button>
          )}
        </div>
      </Card>
    </>
  );
}
