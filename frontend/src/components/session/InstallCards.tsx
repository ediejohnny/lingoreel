import { Download, ExternalLink, X } from "lucide-react";
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
  onDismiss: (id: string) => void;
}

/** InstallCards offer the players the app can drive when none is installed. */
export function InstallCards({ recommendations, installing, error, onInstall, onDownload, onDismiss }: InstallCardsProps) {
  return (
    <div className="flex flex-col gap-3">
      <Banner tone="warning">
        Nenhum player compatível encontrado. Instale um para assistir; o LingoReel encontra o player sozinho depois.
      </Banner>
      {error && <Banner tone="error">{error}</Banner>}
      {recommendations.length === 0 && (
        <p className="text-sm text-fg-muted">Você dispensou as recomendações. Instale um player compatível e abra o vídeo de novo.</p>
      )}
      {recommendations.map((item) => (
        <article key={item.id} className="flex flex-col gap-3 rounded-card border border-line bg-raised p-4">
          <header className="flex items-center gap-1">
            <h3 className="font-semibold text-fg">{item.name}</h3>
            <HelpTip topic={`por que o ${item.name}`}>{item.benefit}</HelpTip>
          </header>
          <div className="flex flex-wrap gap-2">
            {item.autoInstall && (
              <Button variant="primary" icon={Download} loading={installing === item.id} onClick={() => onInstall(item.id)}>
                {installing === item.id ? "Instalando…" : "Instalar automaticamente"}
              </Button>
            )}
            <Button icon={ExternalLink} onClick={() => onDownload(item.id)}>
              Baixar no site oficial
            </Button>
            <Button variant="ghost" icon={X} onClick={() => onDismiss(item.id)} disabled={installing === item.id}>
              Dispensar
            </Button>
          </div>
          {item.autoInstall && (
            <p className="text-xs text-fg-muted">A instalação usa o winget do Windows; o Windows pode pedir sua permissão.</p>
          )}
        </article>
      ))}
    </div>
  );
}
