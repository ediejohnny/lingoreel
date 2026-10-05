import { useEffect, useRef, useState } from "react";
import { t } from "../../i18n";
import {
  backend,
  messageOf,
  type AppearanceSettings,
  type BarSettings,
  type EditSettings,
  type InterfaceSettings,
} from "../../services/backend";
import { themeLabel, type Theme } from "../../stores/theme";
import { Banner, Card } from "../ui/Feedback";
import { Toggle } from "../ui/Field";
import { HelpTip } from "../ui/HelpTip";
import { ColorField, NumberField, Segmented } from "../ui/Inputs";
import { Select } from "../ui/Select";
import { RestoreButton } from "./RestoreButton";
import type { SectionProps } from "./section";

/** waitBeforeApplying lets a slider move before the look is applied to the video. */
const waitBeforeApplying = 300;

/** waitBeforeDrawing lets a slider move before the bar's picture is asked again. */
const waitBeforeDrawing = 150;

/** sampleSize is the sample line's size in the correction box's sample, in pixels. */
const sampleSize = 24;

/** fallbackAccent is the light theme's blue, for an accent when the screen's cannot be read. */
const fallbackAccent = "#1F66D1";

interface AppearanceSectionProps extends Pick<SectionProps, "view" | "draft"> {
  theme: Theme;
  onTheme: (theme: Theme) => void;
  /** look is the app's own accent, font and text size; onLook applies and keeps another. */
  look: InterfaceSettings;
  onLook: (look: InterfaceSettings) => void;
}

/**
 * AppearanceSection holds the app's theme and own look, and the look of the
 * controls over the video: the control bar with its menus, and the box that
 * corrects a line. Like the theme, they are not part of the profile: each
 * change applies at once, to the video being watched too.
 */
export function AppearanceSection({ view, draft, theme, onTheme, look, onLook }: AppearanceSectionProps) {
  const [controls, setControls] = useState<AppearanceSettings>(view.appearance);
  const applied = useRef(JSON.stringify(view.appearance));
  const [problem, setProblem] = useState("");

  useEffect(() => {
    const next = JSON.stringify(controls);
    if (next === applied.current) {
      return;
    }
    let current = true;
    const timer = window.setTimeout(() => {
      backend
        .setAppearance(controls)
        .then(() => {
          applied.current = next;
          if (current) {
            setProblem("");
          }
        })
        .catch((failure) => current && setProblem(messageOf(failure)));
    }, waitBeforeApplying);
    return () => {
      current = false;
      window.clearTimeout(timer);
    };
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
function AppCard({ view, theme, onTheme, look, onLook }: Omit<AppearanceSectionProps, "draft">) {
  const themes: Theme[] = ["system", "light", "dark"];
  const options = view.options;
  const limits = options.limits;
  const set =
    <K extends keyof InterfaceSettings>(key: K) =>
    (value: InterfaceSettings[K]) =>
      onLook({ ...look, [key]: value });
  const fonts = look.fontFamily === "" || options.fonts.includes(look.fontFamily) ? options.fonts : [look.fontFamily, ...options.fonts];
  // A new accent starts from the one on the screen.
  const ownAccent = (on: boolean) => {
    const shown = getComputedStyle(document.documentElement).getPropertyValue("--sp-accent").trim();
    set("accent")(on ? (/^#[0-9a-f]{6}$/i.test(shown) ? shown.toUpperCase() : fallbackAccent) : "");
  };
  return (
    <Card
      title={t("ui.settings.appearance.title")}
      aside={
        <div className="flex items-center gap-1">
          <RestoreButton onRestore={() => onLook(options.interfaceDefaults)} />
          <HelpTip topic={t("ui.settings.appearance.title")}>{t("ui.settings.appearance.app.help")}</HelpTip>
        </div>
      }
    >
      <div className="@container flex flex-col gap-4">
        <Segmented
          label={t("ui.settings.appearance.theme")}
          value={theme}
          options={themes.map((value) => ({ value, label: themeLabel(value) }))}
          onChange={(value) => onTheme(value as Theme)}
          help={<HelpTip topic={t("ui.settings.appearance.topic")}>{t("ui.settings.appearance.help")}</HelpTip>}
        />
        <div className="grid gap-4 @lg:grid-cols-2">
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

/** BarCard edits the control bar and its menus, beside a picture of it. */
function BarCard({ view, bar, onChange }: BarCardProps) {
  const options = view.options;
  const limits = options.limits;
  const set =
    <K extends keyof BarSettings>(key: K) =>
    (value: BarSettings[K]) =>
      onChange({ ...bar, [key]: value });
  const fonts = options.fonts.includes(bar.fontFamily) ? options.fonts : [bar.fontFamily, ...options.fonts];
  return (
    <Card
      title={t("ui.settings.appearance.bar.title")}
      aside={
        <div className="flex items-center gap-1">
          <RestoreButton onRestore={() => onChange(options.appearanceDefaults.bar)} />
          <HelpTip topic={t("ui.settings.appearance.bar.title")}>{t("ui.settings.appearance.bar.help")}</HelpTip>
        </div>
      }
    >
      <div className="@container flex flex-col gap-4">
        <BarPreview bar={bar} />
        <div className="grid gap-4 @lg:grid-cols-2">
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

/** BarPreview shows the bar over a sample picture, drawn by the same code as over the video. */
function BarPreview({ bar }: { bar: BarSettings }) {
  const box = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0);
  const [picture, setPicture] = useState("");
  const [problem, setProblem] = useState("");

  useEffect(() => {
    const element = box.current;
    if (!element) {
      return;
    }
    const observer = new ResizeObserver(([entry]) => entry && setWidth(Math.round(entry.contentRect.width)));
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (width < 16) {
      return;
    }
    let current = true;
    const timer = window.setTimeout(() => {
      const scale = window.devicePixelRatio || 1;
      const pixels = Math.round(width * scale);
      backend
        .previewBar({ bar, width: pixels, height: Math.round((pixels * 9) / 16), scale })
        .then((next) => {
          if (current) {
            setPicture(next);
            setProblem("");
          }
        })
        .catch((failure) => current && setProblem(messageOf(failure)));
    }, waitBeforeDrawing);
    return () => {
      current = false;
      window.clearTimeout(timer);
    };
  }, [bar, width]);

  return (
    <figure className="flex flex-col gap-2">
      <div ref={box} className="relative aspect-video w-full overflow-hidden rounded-card border border-line bg-sunken">
        {picture && <img src={picture} alt={t("ui.settings.appearance.bar.alt")} className="size-full" />}
      </div>
      <figcaption className={`text-xs ${problem ? "text-error" : "text-fg-muted"}`}>
        {problem || t("ui.settings.appearance.bar.caption")}
      </figcaption>
    </figure>
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
  const set =
    <K extends keyof EditSettings>(key: K) =>
    (value: EditSettings[K]) =>
      onChange({ ...edit, [key]: value });
  return (
    <Card
      title={t("ui.settings.appearance.edit.title")}
      aside={
        <div className="flex items-center gap-1">
          <RestoreButton onRestore={() => onChange(options.appearanceDefaults.edit)} />
          <HelpTip topic={t("ui.settings.appearance.edit.title")}>{t("ui.settings.appearance.edit.help")}</HelpTip>
        </div>
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
        <div className="grid gap-4 @lg:grid-cols-2">
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
