import { Download, ExternalLink, X } from "lucide-react";
import { t } from "../../i18n";
import type { PlayerInstall } from "../../services/backend";
import { Button } from "../ui/Button";
import { Banner } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";

interface InstallCardsProps {
  recommendations: PlayerInstall[];
  /** installing is the id of the player being installed. */
  installing: string;
  error: string;
  onInstall: (id: string) => void;
  onDownload: (id: string) => void;
  /** onDismiss hides a recommendation; without it none can be hidden. */
  onDismiss?: (id: string) => void;
}

/** InstallCards offer the players the app can drive when none is installed. */
export function InstallCards({ recommendations, installing, error, onInstall, onDownload, onDismiss }: InstallCardsProps) {
  return (
    <div className="flex flex-col gap-3">
      <Banner tone="warning">{t("ui.install.none")}</Banner>
      {error && <Banner tone="error">{error}</Banner>}
      {recommendations.length === 0 && onDismiss && <p className="text-sm text-fg-muted">{t("ui.install.dismissed")}</p>}
      {recommendations.map((item) => (
        <article key={item.id} className="flex flex-col gap-3 rounded-card border border-line bg-raised p-4">
          <header className="flex items-center gap-1">
            <h3 className="font-semibold text-fg">{item.name}</h3>
            <HelpTip topic={t("ui.install.why", { name: item.name })}>{item.benefit}</HelpTip>
          </header>
          <div className="flex flex-wrap gap-2">
            {item.autoInstall && (
              <Button variant="primary" icon={Download} loading={installing === item.id} onClick={() => onInstall(item.id)}>
                {installing === item.id ? t("ui.install.installing") : t("ui.install.auto")}
              </Button>
            )}
            <Button icon={ExternalLink} onClick={() => onDownload(item.id)}>
              {t("ui.install.download")}
            </Button>
            {onDismiss && (
              <Button variant="ghost" icon={X} onClick={() => onDismiss(item.id)} disabled={installing === item.id}>
                {t("ui.install.dismiss")}
              </Button>
            )}
          </div>
          {item.autoInstall && <p className="text-xs text-fg-muted">{t("ui.install.winget")}</p>}
        </article>
      ))}
    </div>
  );
}
