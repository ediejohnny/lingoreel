import { FolderOpen, RefreshCw, RotateCcw } from "lucide-react";
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
      <Card title="Player preferido">
        <Select
          label="Abrir os vídeos com"
          value={draft.preferredPlayer}
          options={[{ value: "", label: "Automático (o melhor instalado)" }, ...supported.map((item) => ({ value: item.id, label: item.name }))]}
          onChange={(id) => change(setDraft, "preferredPlayer")(id)}
          help={
            <HelpTip topic="Player preferido">
              O player que este perfil usa sempre que estiver instalado. Em “Automático”, o app escolhe o melhor que encontrar no
              computador.
            </HelpTip>
          }
        />
      </Card>
      <Card
        title="Onde estão os players"
        aside={
          <HelpTip topic="Onde estão os players">
            O app procura os players nos lugares de costume. Se o seu estiver em outra pasta, escolha o programa dele (o arquivo
            .exe).
          </HelpTip>
        }
      >
        <ul className="flex flex-col divide-y divide-line">
          {supported.map((item) => (
            <li key={item.id} className="flex flex-wrap items-center gap-3 py-3">
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-fg">{item.name}</p>
                <p className="truncate text-xs text-fg-muted" title={item.path}>
                  {item.path ? `${item.custom ? "Escolhido por você" : "Encontrado"}: ${item.path}` : "Não encontrado neste computador"}
                </p>
              </div>
              <Button icon={FolderOpen} onClick={() => void run(() => backend.choosePlayerProgram(item.id))}>
                Escolher programa…
              </Button>
              {item.custom && (
                <Button icon={RotateCcw} variant="ghost" onClick={() => void run(() => backend.clearPlayerPath(item.id), "O app volta a procurar o player sozinho.")}>
                  Procurar sozinho
                </Button>
              )}
            </li>
          ))}
          {others.map((item) => (
            <li key={item.id} className="flex items-center gap-3 py-3">
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-fg">{item.name}</p>
                <p className="text-xs text-fg-muted">Encontrado · suporte em breve</p>
              </div>
            </li>
          ))}
        </ul>
        <div className="mt-4 flex flex-wrap gap-2">
          <Button icon={RefreshCw} onClick={() => void run(() => backend.players(), "Players detectados de novo.")}>
            Detectar novamente
          </Button>
          {view.general.recommendationsHidden && (
            <Button variant="ghost" onClick={() => void run(() => backend.showRecommendations(), "As recomendações de instalação voltam a aparecer.")}>
              Mostrar recomendações de instalação novamente
            </Button>
          )}
        </div>
      </Card>
    </>
  );
}
