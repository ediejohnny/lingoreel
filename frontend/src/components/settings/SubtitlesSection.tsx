import { useState, type ReactNode } from "react";
import { backend, messageOf, type StyleSettings } from "../../services/backend";
import { lineLanguage, type SubtitleLine } from "../../settings";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import { NumberField } from "../ui/Inputs";
import { Select } from "../ui/Select";
import { change, type SectionProps } from "./section";
import { StyleEditor } from "./StyleEditor";
import { SubtitlePreview } from "./SubtitlePreview";

/**
 * SubtitlesLayout puts the preview beside the fields, where it stays in
 * sight while only the fields scroll; when there is no room for both, it
 * goes on top and scrolls with them. It follows its own width, not the
 * window's, so a narrow panel lays it out the narrow way.
 */
function SubtitlesLayout({
  draft,
  highlight,
  children,
}: Pick<SectionProps, "draft"> & { highlight: SubtitleLine | ""; children: ReactNode }) {
  return (
    <div className="@container">
      <div className="grid gap-5 @2xl:grid-cols-[minmax(0,1fr)_minmax(18rem,40%)] @2xl:items-start">
        <aside aria-label="Prévia das legendas" className="@2xl:sticky @2xl:top-4 @2xl:order-2">
          <SubtitlePreview primary={draft.primary} translation={draft.translation} layout={draft.layout} highlight={highlight} />
        </aside>
        <div className="flex min-w-0 flex-col gap-5 @container @2xl:order-1">{children}</div>
      </div>
    </div>
  );
}

/** SubtitleLineSection edits how one subtitle line looks, and its language. */
export function SubtitleLineSection({ line, view, draft, setDraft }: SectionProps & { line: SubtitleLine }) {
  const [problem, setProblem] = useState("");
  const options = view.options;
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
  const languages = draft.languages;
  const languageOptions = options.languages.map((option) => ({ value: option.value, label: option.label }));
  const language = lineLanguage(line, languages[line], options.languages);
  const title = line === "primary" ? `Legenda de cima · ${language}` : `Legenda de baixo · ${language}`;

  return (
    <SubtitlesLayout draft={draft} highlight={line}>
      <Card title={title}>
        <div className="flex flex-col gap-5">
          {line === "primary" ? (
            <Select
              label="Idioma preferido"
              value={languages.primary}
              options={languageOptions}
              onChange={(primary) => change(setDraft, "languages")({ ...languages, primary })}
              help={
                <HelpTip topic="Idioma preferido">
                  O idioma que o app escolhe sozinho para a legenda de cima ao abrir um vídeo: o que você estuda.
                </HelpTip>
              }
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
    </SubtitlesLayout>
  );
}

/** SubtitlesSection holds what the two lines share: the margins and the sync steps. */
export function SubtitlesSection({ view, draft, setDraft, open }: SectionProps) {
  const options = view.options;
  const setLayout = change(setDraft, "layout");
  const layout = draft.layout;
  const languages = draft.languages;
  const keyOf = (command: string) => draft.shortcuts.find((item) => item.command === command)?.label ?? "";
  const lines: { line: SubtitleLine; label: string }[] = [
    { line: "primary", label: `Legenda de cima · ${lineLanguage("primary", languages.primary, options.languages)}` },
    { line: "translation", label: `Legenda de baixo · ${lineLanguage("translation", languages.translation, options.languages)}` },
  ];

  return (
    <SubtitlesLayout draft={draft} highlight="">
      <Card title="As duas legendas">
        <p className="mb-3 text-sm text-fg-secondary">
          Cada legenda tem a sua fonte, o seu tamanho, as suas cores e a sua posição. Escolha qual mudar:
        </p>
        <div className="flex flex-wrap gap-2">
          {lines.map((item) => (
            <button
              key={item.line}
              type="button"
              onClick={() => open?.(item.line === "primary" ? "subtitles-primary" : "subtitles-translation")}
              className="h-9 rounded-control border border-line-strong bg-surface px-3 text-sm text-fg hover:bg-sunken"
            >
              {item.label}
            </button>
          ))}
        </div>
      </Card>
      <Card
        title="Margens"
        aside={
          <HelpTip topic="Margens">
            A área onde as legendas podem ficar, valendo para as duas. Os valores são partes do vídeo, então continuam certos em qualquer
            tamanho de janela.
          </HelpTip>
        }
      >
        <div className="grid gap-4 @lg:grid-cols-2">
          <NumberField
            label="Margem esquerda"
            value={layout.marginLeft}
            min={0}
            max={options.limits.marginMax}
            step={0.01}
            scale={100}
            unit="%"
            onChange={(marginLeft) => setLayout({ ...layout, marginLeft })}
          />
          <NumberField
            label="Margem direita"
            value={layout.marginRight}
            min={0}
            max={options.limits.marginMax}
            step={0.01}
            scale={100}
            unit="%"
            onChange={(marginRight) => setLayout({ ...layout, marginRight })}
          />
          <NumberField
            label="Margem de cima"
            value={layout.marginTop}
            min={0}
            max={options.limits.marginMax}
            step={0.01}
            scale={100}
            unit="%"
            onChange={(marginTop) => setLayout({ ...layout, marginTop })}
          />
          <NumberField
            label="Margem de baixo"
            value={layout.marginBottom}
            min={0}
            max={options.limits.marginMax}
            step={0.01}
            scale={100}
            unit="%"
            onChange={(marginBottom) => setLayout({ ...layout, marginBottom })}
          />
          <NumberField
            label="Largura máxima do texto"
            value={layout.maxWidth}
            min={0.01}
            max={1}
            step={0.01}
            scale={100}
            unit="%"
            onChange={(maxWidth) => setLayout({ ...layout, maxWidth })}
          />
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
        <div className="grid gap-4 @lg:grid-cols-2">
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
    </SubtitlesLayout>
  );
}
