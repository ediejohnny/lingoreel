import { ClipboardCopy, Refresh } from "../ui/icons";
import { useCallback, useEffect, useState } from "react";
import { t } from "../../i18n";
import { backend, messageOf, type Diagnostics } from "../../services/backend";
import { Button } from "../ui/Button";
import { FactList } from "../ui/FactList";
import { Card } from "../ui/Feedback";
import { KeyedTip } from "../ui/HelpTip";

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
    <Card title={t("ui.settings.diagnostics.title")} aside={<KeyedTip k="ui.settings.diagnostics" />}>
      <div className="flex flex-col gap-3">
        {diagnostics && <FactList facts={diagnostics.items} />}
        <div className="flex flex-wrap items-center gap-2">
          <Button icon={ClipboardCopy} disabled={!diagnostics} onClick={() => void copy()}>
            {t("ui.settings.diagnostics.copy")}
          </Button>
          <Button icon={Refresh} variant="ghost" onClick={refresh}>
            {t("ui.settings.diagnostics.refresh")}
          </Button>
          {status && (
            <span role="status" className="text-sm text-fg-secondary">
              {status}
            </span>
          )}
        </div>
      </div>
    </Card>
  );
}
