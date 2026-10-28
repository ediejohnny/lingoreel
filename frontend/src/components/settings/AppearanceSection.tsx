import { useEffect, useRef, useState } from "react";
import { t } from "../../i18n";
import { newAccent } from "../../interface";
import {
  backend,
  messageOf,
  type AppearanceSettings,
  type BarSettings,
  type EditSettings,
  type InterfaceSettings,
} from "../../services/backend";
import { themeLabel, type Theme } from "../../theme";
import { after } from "../../timing";
import { Banner, Card } from "../ui/Feedback";
import { Toggle } from "../ui/Field";
import { HelpTip, KeyedTip } from "../ui/HelpTip";
import { ColorField, NumberField, Segmented } from "../ui/Inputs";
import { Select } from "../ui/Select";
import { PreviewFigure, type PictureSize } from "./PreviewFigure";
import { RestoreButton } from "./RestoreButton";
import { fieldSetter, type SectionProps } from "./section";
import { ThumbnailsCard } from "./ThumbnailsCard";

/** waitBeforeApplying lets a slider move before the look is applied to the video. */
const waitBeforeApplying = 300;

/** sampleSize is the sample line's size in the correction box's sample, in pixels. */
const sampleSize = 24;

interface AppearanceSectionProps extends Pick<SectionProps, "view" | "draft" | "run"> {
  theme: Theme;
  onTheme: (theme: Theme) => void;
  /** look is the app's own accent, font and text size; onLook applies and keeps another. */
  look: InterfaceSettings;
  onLook: (look: InterfaceSettings) => void;
}

/**
 * AppearanceSection holds the app's theme and own look, and the look of the
 * controls over the video: the control bar with its menus and the seek bar's
 * stills, and the box that corrects a line. Like the theme, they are not part
 * of the profile: each change applies at once, to the video being watched too.
 */
export function AppearanceSection({ view, draft, run, theme, onTheme, look, onLook }: AppearanceSectionProps) {
  const [controls, setControls] = useState<AppearanceSettings>(view.appearance);
  const applied = useRef(JSON.stringify(view.appearance));
  const [problem, setProblem] = useState("");

  useEffect(() => {
    const next = JSON.stringify(controls);
    if (next === applied.current) {
      return;
    }
    return after(waitBeforeApplying, (current) => {
      backend
        .setAppearance(controls)
        .then(() => {
          applied.current = next;
          if (current()) {
            setProblem("");
          }
        })
        .catch((failure) => current() && setProblem(messageOf(failure)));
    });
  }, [controls]);

  return (
    <>
      {problem && (
        <Banner tone="error" onClose={() => setProblem("")}>
          {problem}
        </Banner>
      )}
      <AppCard view={view} theme={theme} onTheme={onTheme} look={look} onLook={onLook} />
      <BarCard view={view} bar={controls.bar} onChange={(bar) => setControls((now) => ({ ...now, bar }))} />
      <ThumbnailsCard view={view} run={run} />
      <EditCard
        view={view}
        edit={controls.edit}
        font={draft.primary.fontFamily}
        onChange={(edit) => setControls((now) => ({ ...now, edit }))}
      />
    </>
  );
}

/** AppCard edits the app's own look: its theme, accent, font and text size. */
function AppCard({ view, theme, onTheme, look, onLook }: Omit<AppearanceSectionProps, "draft" | "run">) {
  const themes: Theme[] = ["system", "light", "dark"];
  const options = view.options;
  const limits = options.limits;
  const set = fieldSetter(look, onLook);
  const fonts = look.fontFamily === "" || options.fonts.includes(look.fontFamily) ? options.fonts : [look.fontFamily, ...options.fonts];
  const ownAccent = (on: boolean) =>
    set("accent")(on ? newAccent(getComputedStyle(document.documentElement).getPropertyValue("--sp-accent")) : "");
  return (
    <Card
      title={t("ui.settings.appearance.title")}
      aside={
        <RestoreButton
          onRestore={() => onLook(options.interfaceDefaults)}
          help={<HelpTip topic={t("ui.settings.appearance.title")}>{t("ui.settings.appearance.app.help")}</HelpTip>}
        />
      }
    >
      <div className="@container flex flex-col gap-4">
        <Segmented
          label={t("ui.settings.appearance.theme")}
          value={theme}
          options={themes.map((value) => ({ value, label: themeLabel(value) }))}
          onChange={(value) => onTheme(value as Theme)}
          help={<KeyedTip k="ui.settings.appearance" />}
        />
        <div className="grid grid-cols-[minmax(0,1fr)] gap-4 @lg:grid-cols-2">
          <div className="flex flex-col gap-3">
            <Toggle label={t("ui.settings.appearance.app.ownAccent")} checked={look.accent !== ""} onChange={ownAccent} />
            {look.accent !== "" && (
              <ColorField label={t("ui.settings.appearance.app.accent")} value={look.accent} onChange={set("accent")} />
            )}
          </div>
          <div className="flex flex-col gap-4">
            <Select
              label={t("ui.settings.style.font")}
              value={look.fontFamily}
              options={[
                { value: "", label: t("ui.settings.appearance.app.fontDefault") },
                ...fonts.map((font) => ({ value: font, label: font })),
              ]}
              onChange={set("fontFamily")}
            />
            <NumberField
              label={t("ui.settings.appearance.app.textSize")}
              value={look.textSize}
              min={limits.textSizeMin}
              max={limits.textSizeMax}
              step={0.05}
              scale={100}
              unit="%"
              onChange={set("textSize")}
            />
          </div>
        </div>
      </div>
    </Card>
  );
}

interface BarCardProps {
  view: SectionProps["view"];
  bar: BarSettings;
  onChange: (bar: BarSettings) => void;
}

const drawBar = (bar: BarSettings, size: PictureSize) => backend.previewBar({ bar, ...size });

/** BarCard edits the control bar and its menus, beside a picture of it over a sample, drawn as over the video. */
function BarCard({ view, bar, onChange }: BarCardProps) {
  const options = view.options;
  const limits = options.limits;
  const set = fieldSetter(bar, onChange);
  const fonts = options.fonts.includes(bar.fontFamily) ? options.fonts : [bar.fontFamily, ...options.fonts];
  return (
    <Card
      title={t("ui.settings.appearance.bar.title")}
      aside={
        <RestoreButton
          onRestore={() => onChange(options.appearanceDefaults.bar)}
          help={<HelpTip topic={t("ui.settings.appearance.bar.title")}>{t("ui.settings.appearance.bar.help")}</HelpTip>}
        />
      }
    >
      <div className="@container flex flex-col gap-4">
        <PreviewFigure
          request={bar}
          draw={drawBar}
          alt={t("ui.settings.appearance.bar.alt")}
          caption={t("ui.settings.appearance.bar.caption")}
        />
        <div className="grid grid-cols-[minmax(0,1fr)] gap-4 @lg:grid-cols-2">
          <ColorField label={t("ui.settings.appearance.bar.panel")} value={bar.panel} alpha onChange={set("panel")} />
          <ColorField label={t("ui.settings.appearance.bar.menu")} value={bar.menu} alpha onChange={set("menu")} />
          <ColorField label={t("ui.settings.appearance.bar.accent")} value={bar.accent} onChange={set("accent")} />
          <ColorField label={t("ui.settings.appearance.bar.icon")} value={bar.icon} onChange={set("icon")} />
          <ColorField label={t("ui.settings.appearance.bar.loud")} value={bar.loud} onChange={set("loud")} />
          <Select
            label={t("ui.settings.style.font")}
            value={bar.fontFamily}
            options={fonts.map((font) => ({ value: font, label: font }))}
            onChange={set("fontFamily")}
          />
          <NumberField
            label={t("ui.settings.style.size")}
            value={bar.size}
            min={limits.barSizeMin}
            max={limits.barSizeMax}
            step={0.05}
            scale={100}
            unit="%"
            onChange={set("size")}
          />
          <NumberField
            label={t("ui.settings.appearance.bar.hideDelay")}
            value={bar.hideDelay}
            min={limits.hideDelayMin}
            max={limits.hideDelayMax}
            step={0.5}
            unit="s"
            onChange={set("hideDelay")}
            help={<HelpTip topic={t("ui.settings.appearance.bar.hideDelay")}>{t("ui.settings.appearance.bar.hideDelayHelp")}</HelpTip>}
          />
        </div>
      </div>
    </Card>
  );
}

interface EditCardProps {
  view: SectionProps["view"];
  edit: EditSettings;
  /** font is the line's font: the box writes in it. */
  font: string;
  onChange: (edit: EditSettings) => void;
}

/** EditCard edits the box that corrects a line, beside a sample of it. */
function EditCard({ view, edit, font, onChange }: EditCardProps) {
  const options = view.options;
  const limits = options.limits;
  const set = fieldSetter(edit, onChange);
  return (
    <Card
      title={t("ui.settings.appearance.edit.title")}
      aside={
        <RestoreButton
          onRestore={() => onChange(options.appearanceDefaults.edit)}
          help={<HelpTip topic={t("ui.settings.appearance.edit.title")}>{t("ui.settings.appearance.edit.help")}</HelpTip>}
        />
      }
    >
      <div className="@container flex flex-col gap-4">
        <figure className="flex flex-col gap-2">
          <div className="flex justify-center rounded-card border border-line bg-sunken p-6">
            <div
              role="img"
              aria-label={t("ui.settings.appearance.edit.alt")}
              className="flex min-w-60 flex-col items-center gap-1 border border-line-strong px-2 pt-2 pb-1"
              style={{ background: edit.background }}
            >
              <span style={{ color: edit.text, fontFamily: font, fontSize: sampleSize }}>{t("ui.settings.appearance.edit.sample")}</span>
              <span style={{ color: edit.hint, fontSize: edit.hintSize }}>{options.editHint}</span>
            </div>
          </div>
          <figcaption className="text-xs text-fg-muted">{t("ui.settings.appearance.edit.caption")}</figcaption>
        </figure>
        <div className="grid grid-cols-[minmax(0,1fr)] gap-4 @lg:grid-cols-2">
          <ColorField label={t("ui.settings.appearance.edit.background")} value={edit.background} onChange={set("background")} />
          <ColorField label={t("ui.settings.appearance.edit.text")} value={edit.text} onChange={set("text")} />
          <ColorField label={t("ui.settings.appearance.edit.hint")} value={edit.hint} onChange={set("hint")} />
          <NumberField
            label={t("ui.settings.appearance.edit.hintSize")}
            value={edit.hintSize}
            min={limits.hintSizeMin}
            max={limits.hintSizeMax}
            step={1}
            unit="px"
            onChange={set("hintSize")}
          />
        </div>
      </div>
    </Card>
  );
}
