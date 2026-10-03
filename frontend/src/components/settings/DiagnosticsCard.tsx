import { ClipboardCopy, RefreshCw } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
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
    setStatus(copied ? "Diagnóstico copiado. Cole-o na sua mensagem de suporte." : "Não foi possível copiar. Selecione o texto e copie.");
  };

  return (
    <Card
      title="Diagnóstico"
      aside={
        <HelpTip topic="diagnóstico">
          O estado do app, do computador e do vídeo aberto, para relatar um problema. Não tem o texto das legendas, e nada é enviado: você
          copia e cola onde quiser.
        </HelpTip>
      }
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
          Copiar diagnóstico
        </Button>
        <Button icon={RefreshCw} variant="ghost" onClick={refresh}>
          Atualizar
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
