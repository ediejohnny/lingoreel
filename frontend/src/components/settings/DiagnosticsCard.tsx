import { ClipboardCopy, RefreshCw } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { t } from "../../i18n";
import { backend, messageOf, type Diagnostics } from "../../services/backend";
import { Button } from "../ui/Button";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";

/** DiagnosticsCard shows the state of the app for a problem report, to copy. */
export function DiagnosticsCard() {
  const [diagnostics, setDiagnostics] = useState<Diagnostics | null>(null);
  const [status, setStatus] = useState("");

  const refresh = useCallback(() => {
    backend
      .diagnostics()
      .then((next) => {
        setDiagnostics(next);
        setStatus("");
      })
      .catch((failure) => setStatus(messageOf(failure)));
  }, []);

  useEffect(refresh, [refresh]);

  const copy = async () => {
    if (!diagnostics) {
      return;
    }
    const copied = await backend.copyText(diagnostics.text).catch(() => false);
    setStatus(copied ? t("ui.settings.diagnostics.copied") : t("ui.settings.diagnostics.copyFailed"));
  };

  return (
    <Card
      title={t("ui.settings.diagnostics.title")}
      aside={<HelpTip topic={t("ui.settings.diagnostics.topic")}>{t("ui.settings.diagnostics.help")}</HelpTip>}
    >
      {diagnostics && (
        <dl className="mb-3 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 rounded-control border border-line bg-surface-sunken p-3 text-xs select-text">
          {diagnostics.items.map((item) => (
            <div key={item.label} className="contents">
              <dt className="text-fg-muted">{item.label}</dt>
              <dd className="break-all text-fg">{item.value}</dd>
            </div>
          ))}
        </dl>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Button icon={ClipboardCopy} disabled={!diagnostics} onClick={() => void copy()}>
          {t("ui.settings.diagnostics.copy")}
        </Button>
        <Button icon={RefreshCw} variant="ghost" onClick={refresh}>
          {t("ui.settings.diagnostics.refresh")}
        </Button>
        {status && (
          <span role="status" className="text-sm text-fg-secondary">
            {status}
          </span>
        )}
      </div>
    </Card>
  );
}
