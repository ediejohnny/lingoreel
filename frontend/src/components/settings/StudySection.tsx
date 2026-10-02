import type { KaraokeSettings } from "../../services/backend";
import { Card } from "../ui/Feedback";
import { Toggle } from "../ui/Field";
import { HelpTip } from "../ui/HelpTip";
import { ColorField, NumberField, Segmented } from "../ui/Inputs";
import { change, type SectionProps } from "./section";
import { ShortcutCapture } from "./ShortcutCapture";
import { useShortcutEditing } from "./ShortcutsSection";

/** StudySection holds the Modo Karaokê and the translation key. */
export function StudySection(props: SectionProps) {
  const { view, draft, setDraft } = props;
  const karaoke = draft.karaoke;
  const limits = view.options.limits;
  const set = <K extends keyof KaraokeSettings>(key: K) => (value: KaraokeSettings[K]) =>
    change(setDraft, "karaoke")({ ...karaoke, [key]: value });
  const { taken, set: setShortcut, defaultOf } = useShortcutEditing(props);
  const translationKey = draft.shortcuts.find((item) => item.command === "translation");
  const standardKey = defaultOf("translation");

  return (
    <>
      <Card
        title="Modo Karaokê"
        aside={
          <HelpTip topic="Modo Karaokê">
            Mostra a próxima fala antes da hora, para você se preparar e falar junto com o ator, e pinta o texto enquanto ele fala.
            Também dá para ligar e desligar pelo painel enquanto o filme toca.
          </HelpTip>
        }
      >
        <div className="flex flex-col gap-5">
          <Toggle label="Ligar o Modo Karaokê ao abrir um vídeo" checked={karaoke.enabled} onChange={set("enabled")} />
          <div className="grid gap-4 sm:grid-cols-2">
            <NumberField
              label="Lacuna mínima"
              value={karaoke.minGap}
              min={limits.gapMin}
              max={limits.gapMax}
              step={1}
              unit="s"
              onChange={set("minGap")}
              help={<HelpTip topic="Lacuna mínima">A prévia só aparece depois de uma pausa sem fala pelo menos deste tamanho. Em diálogos rápidos ela não aparece.</HelpTip>}
            />
            <NumberField
              label="Antecedência"
              value={karaoke.lead}
              min={limits.leadMin}
              max={limits.leadMax}
              step={0.5}
              unit="s"
              onChange={set("lead")}
              help={<HelpTip topic="Antecedência">Quanto antes da fala a prévia aparece. Ela nunca cobre a fala anterior: numa pausa mais curta, aparece só no fim da pausa.</HelpTip>}
            />
            <Segmented
              label="Tempo até a fala"
              value={karaoke.indicator}
              options={[
                { value: "bar", label: "Barra" },
                { value: "countdown", label: "Contagem regressiva" },
              ]}
              onChange={set("indicator")}
              help={<HelpTip topic="Tempo até a fala">Uma barra fina que diminui até a fala, ou os segundos que faltam (5… 4… 3…), em tempo real.</HelpTip>}
            />
            <ColorField label="Cor da prévia" value={karaoke.previewColor} onChange={set("previewColor")} />
            <Toggle
              label="Pintar o texto enquanto o ator fala"
              checked={karaoke.fill}
              onChange={set("fill")}
              help={<HelpTip topic="Preenchimento">O texto é pintado da esquerda para a direita. É uma estimativa pelo número de letras: a legenda não traz o tempo de cada palavra.</HelpTip>}
            />
            <ColorField label="Cor do preenchimento" value={karaoke.fillColor} disabled={!karaoke.fill} onChange={set("fillColor")} />
            <NumberField
              label="Repetir cada fala"
              value={karaoke.repeat}
              min={0}
              max={limits.repeatMax}
              step={1}
              unit="×"
              onChange={set("repeat")}
              help={<HelpTip topic="Repetir cada fala">Quantas vezes cada fala toca de novo antes de seguir. 0 desliga.</HelpTip>}
            />
          </div>
        </div>
      </Card>
      <Card
        title="Tecla da tradução"
        aside={
          <HelpTip topic="Tecla da tradução">
            Na janela do filme, esta tecla mostra e esconde a legenda de baixo. Desmarcada a caixa, cada toque alterna: aperta e
            mostra, aperta de novo e esconde. Marcada, a tradução aparece só enquanto a tecla está pressionada.
          </HelpTip>
        }
      >
        <div className="flex flex-col gap-4">
          {translationKey && (
            <div className="flex items-center justify-between gap-4">
              <span className="text-sm text-fg">Tecla</span>
              <ShortcutCapture
                name={translationKey.name}
                label={translationKey.label}
                taken={taken("translation")}
                onChange={(chord, label) => setShortcut("translation", chord, label)}
                onDefault={
                  standardKey && standardKey.chord !== translationKey.chord && !taken("translation")(standardKey.chord)
                    ? () => setShortcut("translation", standardKey.chord, standardKey.label)
                    : undefined
                }
              />
            </div>
          )}
          <Toggle
            label="Mostrar a tradução só enquanto a tecla estiver pressionada"
            checked={draft.reveal === "hold"}
            onChange={(hold) => change(setDraft, "reveal")(hold ? "hold" : "toggle")}
          />
        </div>
      </Card>
    </>
  );
}
