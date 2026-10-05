import { useState, type ReactNode } from "react";
import { t } from "../../i18n";
import { backend, messageOf, type KaraokeSettings, type StyleSettings } from "../../services/backend";
import { lineLanguage, type PreviewView, type SubtitleLine } from "../../settings";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import { ColorField, NumberField, Segmented } from "../ui/Inputs";
import { Select } from "../ui/Select";
import { MarksCards } from "./MarksCards";
import { RestoreButton } from "./RestoreButton";
import { change, type SectionProps } from "./section";
import { StyleEditor } from "./StyleEditor";
import { SubtitlePreview } from "./SubtitlePreview";

/** Chooser offers what the preview shows, above it. */
interface Chooser {
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
}

/**
 * SubtitlesLayout puts the preview beside the fields, where it stays in
 * sight while only the fields scroll; when there is no room for both, it
 * goes on top and scrolls with them. It follows its own width, not the
 * window's, so a narrow panel lays it out the narrow way.
 */
function SubtitlesLayout({
  draft,
  highlight,
  view = "",
  karaoke = draft.karaoke,
  chooser,
  children,
}: Pick<SectionProps, "draft"> & {
  highlight: SubtitleLine | "";
  view?: PreviewView;
  /** karaoke is what a "cue" view shows, when it is not the draft's. */
  karaoke?: KaraokeSettings;
  chooser?: Chooser;
  children: ReactNode;
}) {
  return (
    <div className="@container">
      <div className="grid gap-5 @2xl:grid-cols-[minmax(0,1fr)_minmax(18rem,40%)] @2xl:items-start">
        <aside aria-label={t("ui.settings.subtitles.preview")} className="flex flex-col gap-3 @2xl:sticky @2xl:top-4 @2xl:order-2">
          {chooser && (
            <Segmented
              label={t("ui.settings.subtitles.view.label")}
              value={chooser.value}
              options={chooser.options}
              onChange={chooser.onChange}
            />
          )}
          <SubtitlePreview
            primary={draft.primary}
            translation={draft.translation}
            layout={draft.layout}
            highlight={highlight}
            view={view}
            karaoke={karaoke}
            marks={draft.marks}
          />
        </aside>
        <div className="flex min-w-0 flex-col gap-5 @container @2xl:order-1">{children}</div>
      </div>
    </div>
  );
}

/** KaraokeView is what the preview of the line on top shows. */
type KaraokeView = "" | "fill" | "cue";

/** SubtitleLineSection edits how one subtitle line looks, and its language. */
export function SubtitleLineSection({ line, view, draft, setDraft }: SectionProps & { line: SubtitleLine }) {
  const [problem, setProblem] = useState("");
  const [previewView, setPreviewView] = useState<KaraokeView>("");
  // indicator is the one the "cue" view shows: the one last styled, at first the profile's.
  const [indicator, setIndicator] = useState(draft.karaoke.indicator);
  const karaoke = draft.karaoke;
  const options = view.options;
  const defaults = options.defaults;
  const limits = options.limits;
  const style = draft[line];
  const setStyle = (next: StyleSettings) => setDraft((current) => (current ? { ...current, [line]: next } : current));
  const setKaraoke = (next: Partial<KaraokeSettings>, shows: KaraokeView, shown = indicator) => {
    change(setDraft, "karaoke")({ ...karaoke, ...next });
    setPreviewView(shows);
    setIndicator(shown);
  };
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
  const title =
    line === "primary" ? t("ui.settings.subtitles.top.title", { language }) : t("ui.settings.subtitles.bottom.title", { language });
  const chooser: Chooser | undefined =
    line === "primary"
      ? {
          value: previewView,
          options: [
            { value: "", label: t("ui.settings.subtitles.view.normal") },
            { value: "fill", label: t("ui.settings.subtitles.view.fill") },
            { value: "cue", label: t("ui.settings.subtitles.view.cue") },
          ],
          onChange: (next) => setPreviewView(next === "fill" || next === "cue" ? next : ""),
        }
      : undefined;

  return (
    <SubtitlesLayout
      draft={draft}
      highlight={line}
      view={line === "primary" ? previewView : ""}
      karaoke={{ ...karaoke, indicator }}
      chooser={chooser}
    >
      <Card title={title} aside={<RestoreButton onRestore={() => setStyle(defaults[line])} />}>
        <div className="flex flex-col gap-5">
          {line === "primary" ? (
            <Select
              label={t("ui.settings.subtitles.language")}
              value={languages.primary}
              options={languageOptions}
              onChange={(primary) => change(setDraft, "languages")({ ...languages, primary })}
              help={<HelpTip topic={t("ui.settings.subtitles.top.languageTopic")}>{t("ui.settings.subtitles.top.languageHelp")}</HelpTip>}
            />
          ) : (
            <Select
              label={t("ui.settings.subtitles.language")}
              value={languages.translation}
              options={[{ value: "", label: t("ui.settings.subtitles.bottom.none") }, ...languageOptions]}
              onChange={(translation) => change(setDraft, "languages")({ ...languages, translation })}
              help={
                <HelpTip topic={t("ui.settings.subtitles.bottom.languageTopic")}>{t("ui.settings.subtitles.bottom.languageHelp")}</HelpTip>
              }
            />
          )}
          {problem && <p className="text-sm text-error">{problem}</p>}
          <StyleEditor style={style} options={options} onChange={setStyle} onPreset={preset} />
        </div>
      </Card>
      {line === "primary" && (
        <Card
          title={t("ui.settings.subtitles.karaoke.title")}
          aside={
            <div className="flex items-center gap-1">
              <RestoreButton
                onRestore={() =>
                  setKaraoke(
                    {
                      fillColor: defaults.karaoke.fillColor,
                      barColor: defaults.karaoke.barColor,
                      barThickness: defaults.karaoke.barThickness,
                      countdownSize: defaults.karaoke.countdownSize,
                    },
                    previewView,
                  )
                }
              />
              <HelpTip topic={t("ui.settings.subtitles.karaoke.topic")}>{t("ui.settings.subtitles.karaoke.help")}</HelpTip>
            </div>
          }
        >
          <div className="grid gap-4 @lg:grid-cols-2">
            <div className="flex flex-col gap-2">
              <ColorField
                label={t("ui.settings.subtitles.karaoke.fillColor")}
                value={karaoke.fillColor}
                disabled={!karaoke.fill}
                onChange={(fillColor) => setKaraoke({ fillColor }, "fill")}
              />
              {!karaoke.fill && <p className="text-sm text-fg-muted">{t("ui.settings.subtitles.karaoke.off")}</p>}
            </div>
            <ColorField
              label={t("ui.settings.subtitles.karaoke.barColor")}
              value={karaoke.barColor}
              alpha
              onChange={(barColor) => setKaraoke({ barColor }, "cue", "bar")}
            />
            <NumberField
              label={t("ui.settings.subtitles.karaoke.barThickness")}
              value={karaoke.barThickness}
              min={limits.barThicknessMin}
              max={limits.barThicknessMax}
              step={1}
              unit="px"
              onChange={(barThickness) => setKaraoke({ barThickness }, "cue", "bar")}
            />
            <NumberField
              label={t("ui.settings.subtitles.karaoke.countdownSize")}
              value={karaoke.countdownSize}
              min={limits.countdownSizeMin}
              max={limits.countdownSizeMax}
              step={0.05}
              scale={100}
              unit="%"
              onChange={(countdownSize) => setKaraoke({ countdownSize }, "cue", "countdown")}
            />
          </div>
        </Card>
      )}
    </SubtitlesLayout>
  );
}

/** CommonScene is what the preview of the common page shows. */
type CommonScene = "" | "notice" | "selection";

/** SubtitlesSection holds what the two lines share: the margins, the marks over the video and the sync steps. */
export function SubtitlesSection({ view, draft, setDraft, open }: SectionProps) {
  const [scene, setScene] = useState<CommonScene>("");
  const options = view.options;
  const setLayout = change(setDraft, "layout");
  const layout = draft.layout;
  const languages = draft.languages;
  const keyOf = (command: string) => draft.shortcuts.find((item) => item.command === command)?.label ?? "";
  const lines: { line: SubtitleLine; label: string }[] = [
    {
      line: "primary",
      label: t("ui.settings.subtitles.top.title", { language: lineLanguage("primary", languages.primary, options.languages) }),
    },
    {
      line: "translation",
      label: t("ui.settings.subtitles.bottom.title", { language: lineLanguage("translation", languages.translation, options.languages) }),
    },
  ];
  const chooser: Chooser = {
    value: scene,
    options: [
      { value: "", label: t("ui.settings.subtitles.view.normal") },
      { value: "notice", label: t("ui.settings.subtitles.view.notice") },
      { value: "selection", label: t("ui.settings.subtitles.view.selection") },
    ],
    onChange: (next) => setScene(next === "notice" || next === "selection" ? next : ""),
  };

  return (
    <SubtitlesLayout
      draft={draft}
      highlight={scene === "selection" ? "primary" : ""}
      view={scene === "notice" ? "notice" : ""}
      chooser={chooser}
    >
      <Card title={t("ui.settings.subtitles.both.title")}>
        <p className="mb-3 text-sm text-fg-secondary">{t("ui.settings.subtitles.both.intro")}</p>
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
        title={t("ui.settings.subtitles.margins.title")}
        aside={
          <div className="flex items-center gap-1">
            <RestoreButton
              onRestore={() => {
                setLayout(options.defaults.layout);
                setScene("");
              }}
            />
            <HelpTip topic={t("ui.settings.subtitles.margins.topic")}>{t("ui.settings.subtitles.margins.help")}</HelpTip>
          </div>
        }
      >
        <div className="grid gap-4 @lg:grid-cols-2">
          <NumberField
            label={t("ui.settings.subtitles.margins.left")}
            value={layout.marginLeft}
            min={0}
            max={options.limits.marginMax}
            step={0.01}
            scale={100}
            unit="%"
            onChange={(marginLeft) => setLayout({ ...layout, marginLeft })}
          />
          <NumberField
            label={t("ui.settings.subtitles.margins.right")}
            value={layout.marginRight}
            min={0}
            max={options.limits.marginMax}
            step={0.01}
            scale={100}
            unit="%"
            onChange={(marginRight) => setLayout({ ...layout, marginRight })}
          />
          <NumberField
            label={t("ui.settings.subtitles.margins.top")}
            value={layout.marginTop}
            min={0}
            max={options.limits.marginMax}
            step={0.01}
            scale={100}
            unit="%"
            onChange={(marginTop) => setLayout({ ...layout, marginTop })}
          />
          <NumberField
            label={t("ui.settings.subtitles.margins.bottom")}
            value={layout.marginBottom}
            min={0}
            max={options.limits.marginMax}
            step={0.01}
            scale={100}
            unit="%"
            onChange={(marginBottom) => setLayout({ ...layout, marginBottom })}
          />
          <NumberField
            label={t("ui.settings.subtitles.margins.maxWidth")}
            value={layout.maxWidth}
            min={0.01}
            max={1}
            step={0.01}
            scale={100}
            unit="%"
            onChange={(maxWidth) => setLayout({ ...layout, maxWidth })}
          />
          <NumberField
            label={t("ui.settings.subtitles.gap.label")}
            value={layout.lineGap}
            min={0}
            max={options.limits.lineGapMax}
            step={0.05}
            onChange={(lineGap) => setLayout({ ...layout, lineGap })}
            help={<HelpTip topic={t("ui.settings.subtitles.gap.label")}>{t("ui.settings.subtitles.gap.help")}</HelpTip>}
          />
        </div>
      </Card>
      <MarksCards view={view} draft={draft} setDraft={setDraft} onShow={setScene} />
      <Card
        title={t("ui.settings.subtitles.sync.title")}
        aside={
          <HelpTip topic={t("ui.settings.subtitles.sync.topic")}>
            {t("ui.settings.subtitles.sync.help", {
              primaryEarlier: keyOf("sync-primary-earlier"),
              primaryLater: keyOf("sync-primary-later"),
              translationEarlier: keyOf("sync-translation-earlier"),
              translationLater: keyOf("sync-translation-later"),
            })}
          </HelpTip>
        }
      >
        <div className="grid gap-4 @lg:grid-cols-2">
          <NumberField
            label={t("ui.settings.subtitles.sync.step")}
            value={draft.syncStep}
            min={options.limits.syncStepMin}
            max={options.limits.syncStepMax}
            step={10}
            unit="ms"
            onChange={(syncStep) => change(setDraft, "syncStep")(syncStep)}
          />
          <NumberField
            label={t("ui.settings.subtitles.sync.fastStep")}
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
