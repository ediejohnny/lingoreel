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

/** StyleEditor edits how one subtitle line looks (RF-054 to RF-063). */
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
          <span className="text-sm font-medium text-fg">Estilo pronto</span>
          <HelpTip topic="Estilo pronto">
            Troca as cores, o contorno, a sombra e o fundo de uma vez. A fonte, o tamanho e a posição continuam como estão; depois você pode
            ajustar cada detalhe.
          </HelpTip>
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

      <fieldset className="grid gap-4 sm:grid-cols-2">
        <legend className="mb-2 text-sm font-semibold text-fg-secondary">Texto</legend>
        <Select
          label="Fonte"
          value={style.fontFamily}
          options={fonts.map((font) => ({ value: font, label: font }))}
          onChange={set("fontFamily")}
        />
        <NumberField
          label="Tamanho"
          value={style.fontSize}
          min={limits.fontSizeMin}
          max={limits.fontSizeMax}
          step={1}
          unit="px"
          onChange={set("fontSize")}
        />
        <Segmented
          label="Tamanho da fonte"
          value={style.responsive ? "responsive" : "fixed"}
          options={[
            { value: "responsive", label: "Acompanha o vídeo" },
            { value: "fixed", label: "Fixo" },
          ]}
          onChange={(value) => set("responsive")(value === "responsive")}
          help={
            <HelpTip topic="Tamanho da fonte">
              “Acompanha o vídeo”: o tamanho vale para um vídeo de 1080 linhas e cresce ou diminui com a janela. “Fixo”: o texto fica do
              mesmo tamanho, qualquer que seja a janela.
            </HelpTip>
          }
        />
        <div className="flex items-end gap-6 pb-2">
          <Toggle label="Negrito" checked={style.bold} onChange={set("bold")} />
          <Toggle label="Itálico" checked={style.italic} onChange={set("italic")} />
        </div>
        <ColorField label="Cor" value={style.color} onChange={set("color")} />
        <NumberField label="Opacidade" value={style.opacity} min={0} max={1} step={0.05} scale={100} unit="%" onChange={set("opacity")} />
      </fieldset>

      <fieldset className="grid gap-4 sm:grid-cols-2">
        <legend className="mb-2 text-sm font-semibold text-fg-secondary">Contorno e sombra</legend>
        <Toggle label="Contorno" checked={style.outline} onChange={set("outline")} />
        <span />
        <ColorField label="Cor do contorno" value={style.outlineColor} alpha disabled={!style.outline} onChange={set("outlineColor")} />
        <NumberField
          label="Espessura"
          value={style.outlineWidth}
          min={0}
          max={limits.outlineMax}
          step={0.5}
          unit="px"
          disabled={!style.outline}
          onChange={set("outlineWidth")}
        />
        <Toggle label="Sombra" checked={style.shadow} onChange={set("shadow")} />
        <span />
        <ColorField label="Cor da sombra" value={style.shadowColor} alpha disabled={!style.shadow} onChange={set("shadowColor")} />
        <NumberField
          label="Desfoque"
          value={style.shadowBlur}
          min={0}
          max={limits.shadowBlurMax}
          step={0.5}
          unit="px"
          disabled={!style.shadow}
          onChange={set("shadowBlur")}
        />
        <NumberField
          label="Deslocamento horizontal"
          value={style.shadowX}
          min={-limits.shadowOffsetMax}
          max={limits.shadowOffsetMax}
          step={0.5}
          unit="px"
          disabled={!style.shadow}
          onChange={set("shadowX")}
        />
        <NumberField
          label="Deslocamento vertical"
          value={style.shadowY}
          min={-limits.shadowOffsetMax}
          max={limits.shadowOffsetMax}
          step={0.5}
          unit="px"
          disabled={!style.shadow}
          onChange={set("shadowY")}
        />
      </fieldset>

      <fieldset className="grid gap-4 sm:grid-cols-2">
        <legend className="mb-2 text-sm font-semibold text-fg-secondary">Fundo</legend>
        <Toggle
          label="Fundo atrás do texto"
          checked={style.background}
          onChange={set("background")}
          help={<HelpTip topic="Fundo">Uma caixa semitransparente atrás da legenda, que ajuda a ler sobre cenas claras.</HelpTip>}
        />
        <span />
        <ColorField
          label="Cor do fundo"
          value={style.backgroundColor}
          alpha
          disabled={!style.background}
          onChange={set("backgroundColor")}
        />
        <NumberField
          label="Margem do fundo"
          value={style.backgroundMargin}
          min={0}
          max={limits.paddingMax}
          step={1}
          unit="px"
          disabled={!style.background}
          onChange={set("backgroundMargin")}
        />
      </fieldset>

      <fieldset className="grid gap-4 sm:grid-cols-2">
        <legend className="mb-2 text-sm font-semibold text-fg-secondary">Posição</legend>
        <NumberField
          label="Altura da base"
          value={style.position}
          min={0.01}
          max={1}
          step={0.01}
          scale={100}
          unit="%"
          onChange={set("position")}
          help={
            <HelpTip topic="Altura da base">
              Onde fica a parte de baixo da legenda, contada do alto do vídeo: 82% é perto do pé da imagem.
            </HelpTip>
          }
        />
        <NumberField
          label="Máximo de linhas"
          value={style.maxLines}
          min={limits.linesMin}
          max={limits.linesMax}
          step={1}
          onChange={set("maxLines")}
        />
        <Segmented
          label="Alinhamento"
          value={style.alignment}
          options={[
            { value: "left", label: "Esquerda" },
            { value: "center", label: "Centro" },
            { value: "right", label: "Direita" },
          ]}
          onChange={set("alignment")}
        />
      </fieldset>
    </div>
  );
}
