import { t } from "../../i18n";
import type { SettingsOptions, StyleSettings } from "../../services/backend";
import { Toggle } from "../ui/Field";
import { HelpTip } from "../ui/HelpTip";
import { ColorField, NumberField, Segmented } from "../ui/Inputs";
import { Select } from "../ui/Select";

interface StyleEditorProps {
  style: StyleSettings;
  options: SettingsOptions;
  onChange: (style: StyleSettings) => void;
  onPreset: (preset: string) => void;
}

/** StyleEditor edits how one subtitle line looks. */
export function StyleEditor({ style, options, onChange, onPreset }: StyleEditorProps) {
  const set =
    <K extends keyof StyleSettings>(key: K) =>
    (value: StyleSettings[K]) =>
      onChange({ ...style, [key]: value });
  const limits = options.limits;
  const fonts = options.fonts.includes(style.fontFamily) ? options.fonts : [style.fontFamily, ...options.fonts];

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-1.5">
        <div className="flex items-center gap-1">
          <span className="text-sm font-medium text-fg">{t("ui.settings.style.preset.label")}</span>
          <HelpTip topic={t("ui.settings.style.preset.topic")}>{t("ui.settings.style.preset.help")}</HelpTip>
        </div>
        <div className="flex flex-wrap gap-2">
          {options.presets.map((preset) => (
            <button
              key={preset.value}
              type="button"
              onClick={() => onPreset(preset.value)}
              className="h-9 rounded-control border border-line-strong bg-surface px-3 text-sm text-fg hover:bg-sunken"
            >
              {preset.label}
            </button>
          ))}
        </div>
      </div>

      <fieldset className="grid gap-4 @lg:grid-cols-2">
        <legend className="mb-2 text-sm font-semibold text-fg-secondary">{t("ui.settings.style.text")}</legend>
        <Select
          label={t("ui.settings.style.font")}
          value={style.fontFamily}
          options={fonts.map((font) => ({ value: font, label: font }))}
          onChange={set("fontFamily")}
        />
        <NumberField
          label={t("ui.settings.style.size")}
          value={style.fontSize}
          min={limits.fontSizeMin}
          max={limits.fontSizeMax}
          step={1}
          unit="px"
          onChange={set("fontSize")}
        />
        <Segmented
          label={t("ui.settings.style.sizeMode.label")}
          value={style.responsive ? "responsive" : "fixed"}
          options={[
            { value: "responsive", label: t("ui.settings.style.sizeMode.responsive") },
            { value: "fixed", label: t("ui.settings.style.sizeMode.fixed") },
          ]}
          onChange={(value) => set("responsive")(value === "responsive")}
          help={<HelpTip topic={t("ui.settings.style.sizeMode.topic")}>{t("ui.settings.style.sizeMode.help")}</HelpTip>}
        />
        <div className="flex items-end gap-6 pb-2">
          <Toggle label={t("ui.settings.style.bold")} checked={style.bold} onChange={set("bold")} />
          <Toggle label={t("ui.settings.style.italic")} checked={style.italic} onChange={set("italic")} />
        </div>
        <ColorField label={t("ui.settings.style.color")} value={style.color} onChange={set("color")} />
        <NumberField
          label={t("ui.settings.style.opacity")}
          value={style.opacity}
          min={0}
          max={1}
          step={0.05}
          scale={100}
          unit="%"
          onChange={set("opacity")}
        />
      </fieldset>

      <fieldset className="grid gap-4 @lg:grid-cols-2">
        <legend className="mb-2 text-sm font-semibold text-fg-secondary">{t("ui.settings.style.outlineShadow")}</legend>
        <Toggle label={t("ui.settings.style.outline")} checked={style.outline} onChange={set("outline")} />
        <span />
        <ColorField
          label={t("ui.settings.style.outlineColor")}
          value={style.outlineColor}
          alpha
          disabled={!style.outline}
          onChange={set("outlineColor")}
        />
        <NumberField
          label={t("ui.settings.style.outlineWidth")}
          value={style.outlineWidth}
          min={0}
          max={limits.outlineMax}
          step={0.5}
          unit="px"
          disabled={!style.outline}
          onChange={set("outlineWidth")}
        />
        <Toggle label={t("ui.settings.style.shadow")} checked={style.shadow} onChange={set("shadow")} />
        <span />
        <ColorField
          label={t("ui.settings.style.shadowColor")}
          value={style.shadowColor}
          alpha
          disabled={!style.shadow}
          onChange={set("shadowColor")}
        />
        <NumberField
          label={t("ui.settings.style.shadowBlur")}
          value={style.shadowBlur}
          min={0}
          max={limits.shadowBlurMax}
          step={0.5}
          unit="px"
          disabled={!style.shadow}
          onChange={set("shadowBlur")}
        />
        <NumberField
          label={t("ui.settings.style.shadowX")}
          value={style.shadowX}
          min={-limits.shadowOffsetMax}
          max={limits.shadowOffsetMax}
          step={0.5}
          unit="px"
          disabled={!style.shadow}
          onChange={set("shadowX")}
        />
        <NumberField
          label={t("ui.settings.style.shadowY")}
          value={style.shadowY}
          min={-limits.shadowOffsetMax}
          max={limits.shadowOffsetMax}
          step={0.5}
          unit="px"
          disabled={!style.shadow}
          onChange={set("shadowY")}
        />
      </fieldset>

      <fieldset className="grid gap-4 @lg:grid-cols-2">
        <legend className="mb-2 text-sm font-semibold text-fg-secondary">{t("ui.settings.style.background.title")}</legend>
        <Toggle
          label={t("ui.settings.style.background.label")}
          checked={style.background}
          onChange={set("background")}
          help={<HelpTip topic={t("ui.settings.style.background.topic")}>{t("ui.settings.style.background.help")}</HelpTip>}
        />
        <span />
        <ColorField
          label={t("ui.settings.style.background.color")}
          value={style.backgroundColor}
          alpha
          disabled={!style.background}
          onChange={set("backgroundColor")}
        />
        <NumberField
          label={t("ui.settings.style.background.margin")}
          value={style.backgroundMargin}
          min={0}
          max={limits.paddingMax}
          step={1}
          unit="px"
          disabled={!style.background}
          onChange={set("backgroundMargin")}
        />
      </fieldset>

      <fieldset className="grid gap-4 @lg:grid-cols-2">
        <legend className="mb-2 text-sm font-semibold text-fg-secondary">{t("ui.settings.style.position.title")}</legend>
        <NumberField
          label={t("ui.settings.style.position.base")}
          value={style.position}
          min={0.01}
          max={1}
          step={0.01}
          scale={100}
          unit="%"
          onChange={set("position")}
          help={<HelpTip topic={t("ui.settings.style.position.topic")}>{t("ui.settings.style.position.help")}</HelpTip>}
        />
        <NumberField
          label={t("ui.settings.style.position.lines")}
          value={style.maxLines}
          min={limits.linesMin}
          max={limits.linesMax}
          step={1}
          onChange={set("maxLines")}
        />
        <Segmented
          label={t("ui.settings.style.position.align")}
          value={style.alignment}
          options={[
            { value: "left", label: t("ui.settings.style.position.left") },
            { value: "center", label: t("ui.settings.style.position.center") },
            { value: "right", label: t("ui.settings.style.position.right") },
          ]}
          onChange={set("alignment")}
        />
      </fieldset>
    </div>
  );
}
