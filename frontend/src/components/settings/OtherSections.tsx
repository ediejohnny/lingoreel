import { FolderOpen, RotateCcw } from "lucide-react";
import { useState } from "react";
import { speedLabel } from "../../format";
import { keyLabel } from "../../keys";
import { backend } from "../../services/backend";
import { themeLabel, type Theme } from "../../stores/theme";
import { Button } from "../ui/Button";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import { NumberField, Segmented } from "../ui/Inputs";
import { Select } from "../ui/Select";
import { change, type SectionProps } from "./section";

/** PlaybackSection holds how a video starts, its speed and its audio, and the precision step. */
export function PlaybackSection({ view, draft, setDraft }: SectionProps) {
  const options = view.options;
  const precisionKeys = [keyLabel(draft.shortcuts, "precision-back"), keyLabel(draft.shortcuts, "precision-forward")]
    .filter((label) => label !== "")
    .join(" e ");
  return (
    <>
      <Card title="Ao abrir um vídeo">
        <div className="grid gap-4 sm:grid-cols-2">
          <Select
            label="Velocidade inicial"
            value={String(draft.speed)}
            options={options.speeds.map((speed) => ({ value: String(speed), label: speedLabel(speed) }))}
            onChange={(speed) => change(setDraft, "speed")(Number(speed))}
            help={
              <HelpTip topic="Velocidade inicial">
                A velocidade com que o vídeo começa. Durante o filme, ela muda pelo painel ou pela barra do vídeo.
              </HelpTip>
            }
          />
          <Select
            label="Idioma do áudio preferido"
            value={draft.languages.audio}
            options={options.languages.map((option) => ({ value: option.value, label: option.label }))}
            onChange={(audio) => change(setDraft, "languages")({ ...draft.languages, audio })}
            help={<HelpTip topic="Idioma do áudio">O áudio que o app escolhe sozinho quando o vídeo tem mais de um.</HelpTip>}
          />
        </div>
      </Card>
      <Card title="Avançar e voltar">
        <NumberField
          label="Passo de precisão"
          value={draft.precisionStep}
          min={options.limits.precisionStepMin}
          max={options.limits.precisionStepMax}
          step={10}
          unit="ms"
          onChange={(step) => change(setDraft, "precisionStep")(Math.round(step))}
          help={
            <HelpTip topic="Passo de precisão">
              {`Quanto os atalhos de precisão${precisionKeys ? ` (${precisionKeys})` : ""} andam no vídeo, para achar o começo exato ` +
                "de uma fala. Os outros saltos são de 5, 10 e 30 segundos."}
            </HelpTip>
          }
        />
      </Card>
    </>
  );
}

interface AppearanceSectionProps {
  theme: Theme;
  onTheme: (theme: Theme) => void;
}

/** AppearanceSection holds the app's theme. */
export function AppearanceSection({ theme, onTheme }: AppearanceSectionProps) {
  const themes: Theme[] = ["system", "light", "dark"];
  return (
    <Card title="Aparência do app">
      <Segmented
        label="Tema"
        value={theme}
        options={themes.map((value) => ({ value, label: themeLabel(value) }))}
        onChange={(value) => onTheme(value as Theme)}
        help={<HelpTip topic="Tema">“Tema do Windows” segue o modo claro ou escuro do sistema. Muda na hora, sem reiniciar.</HelpTip>}
      />
    </Card>
  );
}

/** AdvancedSection holds the settings folder and the reset of the profile. */
export function AdvancedSection({ view, dirty, run }: SectionProps) {
  const [confirming, setConfirming] = useState(false);
  return (
    <>
      <Card title="Pasta das configurações">
        <p className="mb-3 text-sm break-all text-fg-secondary">{view.folder}</p>
        <p className="mb-3 text-xs text-fg-muted">
          Ali ficam os perfis e, na pasta "logs", os registros dos últimos 14 dias, úteis para relatar um problema.
        </p>
        <Button icon={FolderOpen} onClick={() => void run(() => backend.openSettingsFolder())}>
          Abrir a pasta
        </Button>
      </Card>
      <Card title="Restaurar o perfil">
        <p className="mb-3 text-sm text-fg-secondary">
          Volta todas as escolhas do perfil “{view.general.activeProfile}” ao padrão: legendas, cores, atalhos, Modo Karaokê e player. O
          nome do perfil continua.
        </p>
        {confirming ? (
          <div className="flex flex-wrap items-center gap-2 rounded-control border border-error/40 bg-error-soft p-3">
            <span className="flex-1 text-sm text-fg">Restaurar o padrão? Não dá para desfazer.</span>
            <Button
              icon={RotateCcw}
              onClick={() => {
                setConfirming(false);
                void run(() => backend.resetProfile(), "Perfil restaurado ao padrão.");
              }}
            >
              Restaurar
            </Button>
            <Button variant="ghost" onClick={() => setConfirming(false)}>
              Cancelar
            </Button>
          </div>
        ) : (
          <Button icon={RotateCcw} disabled={dirty} onClick={() => setConfirming(true)}>
            Restaurar o padrão…
          </Button>
        )}
      </Card>
    </>
  );
}
