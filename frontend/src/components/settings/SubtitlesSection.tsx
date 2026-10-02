import { useState } from "react";
import { backend, messageOf, type StyleSettings } from "../../services/backend";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import { NumberField, Segmented } from "../ui/Inputs";
import { Select } from "../ui/Select";
import { change, type SectionProps } from "./section";
import { StyleEditor } from "./StyleEditor";
import { SubtitlePreview } from "./SubtitlePreview";

type Line = "primary" | "translation";

/** SubtitlesSection edits the two lines, the margins and the sync steps, with the preview on top. */
export function SubtitlesSection({ view, draft, setDraft }: SectionProps) {
  const [line, setLine] = useState<Line>("primary");
  const [problem, setProblem] = useState("");
  const options = view.options;
  const languageName = (tag: string) => options.languages.find((option) => option.value === tag)?.label ?? tag;
  const style = draft[line];
  const setStyle = (next: StyleSettings) => setDraft((current) => (current ? { ...current, [line]: next } : current));
  const preset = (name: string) => {
    backend
      .applyPreset(style, name)
      .then((next) => {
        setProblem("");
        setStyle(next);
      })
      .catch((failure) => setProblem(messageOf(failure)));
  };
  const setLayout = change(setDraft, "layout");
  const layout = draft.layout;
  const languages = draft.languages;
  const languageOptions = options.languages.map((option) => ({ value: option.value, label: option.label }));
  const keyOf = (command: string) => draft.shortcuts.find((item) => item.command === command)?.label ?? "";

  return (
    <>
      <div className="sticky top-0 z-10 -mx-6 -mt-6 bg-bg px-6 pt-6 pb-2">
        <div className="mx-auto w-full max-w-xl">
          <SubtitlePreview primary={draft.primary} translation={draft.translation} layout={draft.layout} />
        </div>
      </div>
      <Segmented
        label="Legenda"
        value={line}
        options={[
          { value: "primary", label: `De cima (${languageName(languages.primary)})` },
          { value: "translation", label: `De baixo (${languages.translation ? languageName(languages.translation) : "tradução"})` },
        ]}
        onChange={(value) => setLine(value as Line)}
      />
      <Card title={line === "primary" ? "Legenda de cima" : "Legenda de baixo"}>
        <div className="flex flex-col gap-5">
          {line === "primary" ? (
            <Select
              label="Idioma preferido"
              value={languages.primary}
              options={languageOptions}
              onChange={(primary) => change(setDraft, "languages")({ ...languages, primary })}
              help={<HelpTip topic="Idioma preferido">O idioma que o app escolhe sozinho para a legenda de cima ao abrir um vídeo: o que você estuda.</HelpTip>}
            />
          ) : (
            <Select
              label="Idioma preferido"
              value={languages.translation}
              options={[{ value: "", label: "Nenhum (sem tradução)" }, ...languageOptions]}
              onChange={(translation) => change(setDraft, "languages")({ ...languages, translation })}
              help={<HelpTip topic="Idioma da tradução">O idioma que o app escolhe sozinho para a legenda de baixo, a tradução.</HelpTip>}
            />
          )}
          {problem && <p className="text-sm text-error">{problem}</p>}
          <StyleEditor style={style} options={options} onChange={setStyle} onPreset={preset} />
        </div>
      </Card>
      <Card
        title="Margens"
        aside={
          <HelpTip topic="Margens">
            A área onde as legendas podem ficar, valendo para as duas. Os valores são partes do vídeo, então continuam certos em
            qualquer tamanho de janela.
          </HelpTip>
        }
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <NumberField label="Margem esquerda" value={layout.marginLeft} min={0} max={options.limits.marginMax} step={0.01} scale={100} unit="%" onChange={(marginLeft) => setLayout({ ...layout, marginLeft })} />
          <NumberField label="Margem direita" value={layout.marginRight} min={0} max={options.limits.marginMax} step={0.01} scale={100} unit="%" onChange={(marginRight) => setLayout({ ...layout, marginRight })} />
          <NumberField label="Margem de cima" value={layout.marginTop} min={0} max={options.limits.marginMax} step={0.01} scale={100} unit="%" onChange={(marginTop) => setLayout({ ...layout, marginTop })} />
          <NumberField label="Margem de baixo" value={layout.marginBottom} min={0} max={options.limits.marginMax} step={0.01} scale={100} unit="%" onChange={(marginBottom) => setLayout({ ...layout, marginBottom })} />
          <NumberField label="Largura máxima do texto" value={layout.maxWidth} min={0.01} max={1} step={0.01} scale={100} unit="%" onChange={(maxWidth) => setLayout({ ...layout, maxWidth })} />
        </div>
      </Card>
      <Card
        title="Sincronização"
        aside={
          <HelpTip topic="Passos da sincronização">
            {`Quanto cada toque nos atalhos de sincronia (${keyOf("sync-primary-earlier")} e ${keyOf("sync-primary-later")} para a legenda de cima; ${keyOf("sync-translation-earlier")} e ${keyOf("sync-translation-later")} para a de baixo) move a legenda. Segurando a tecla por 1 segundo, vale o passo maior.`}
          </HelpTip>
        }
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <NumberField
            label="Passo de cada toque"
            value={draft.syncStep}
            min={options.limits.syncStepMin}
            max={options.limits.syncStepMax}
            step={10}
            unit="ms"
            onChange={(syncStep) => change(setDraft, "syncStep")(syncStep)}
          />
          <NumberField
            label="Passo segurando"
            value={draft.syncFastStep}
            min={options.limits.syncStepMin}
            max={options.limits.syncFastStepMax}
            step={50}
            unit="ms"
            onChange={(syncFastStep) => change(setDraft, "syncFastStep")(syncFastStep)}
          />
        </div>
      </Card>
    </>
  );
}
