import { useId, useState } from "react";
import type { ReactNode } from "react";
import { clampNumber, joinColor, splitColor } from "../../settings";

interface NumberFieldProps {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  onChange: (value: number) => void;
  /** scale shows the value multiplied, such as 100 for shares shown as %. */
  scale?: number;
  unit?: string;
  help?: ReactNode;
  disabled?: boolean;
}

/** NumberField is a slider with the number beside it, typed or dragged. */
export function NumberField({ label, value, min, max, step, onChange, scale = 1, unit = "", help, disabled = false }: NumberFieldProps) {
  const id = useId();
  const shown = round(value * scale, step * scale);
  // editing is the text being typed; otherwise the field shows the value.
  const [editing, setEditing] = useState<string | null>(null);
  const text = editing ?? String(shown).replace(".", ",");
  const commit = () => {
    if (editing !== null) {
      onChange(clampNumber(editing, min * scale, max * scale, shown) / scale);
    }
    setEditing(null);
  };
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center gap-1">
        <label htmlFor={id} className="text-sm font-medium text-fg">
          {label}
        </label>
        {help}
      </div>
      <div className="flex items-center gap-3">
        <input
          type="range"
          aria-labelledby={id}
          min={min * scale}
          max={max * scale}
          step={step * scale}
          value={shown}
          disabled={disabled}
          onChange={(event) => onChange(Number(event.target.value) / scale)}
          className="h-2 min-w-0 flex-1 accent-[var(--sp-accent)] disabled:opacity-55"
        />
        <div className="flex items-center gap-1">
          <input
            id={id}
            value={text}
            inputMode="decimal"
            disabled={disabled}
            onChange={(event) => setEditing(event.target.value)}
            onBlur={commit}
            onKeyDown={(event) => event.key === "Enter" && commit()}
            className="h-9 w-16 rounded-control border border-line-strong bg-surface px-2 text-right text-sm text-fg tabular-nums disabled:opacity-55"
          />
          {unit && <span className="w-6 text-sm text-fg-muted">{unit}</span>}
        </div>
      </div>
    </div>
  );
}

/** round keeps the precision of the step, so 0.1 + 0.2 shows 0.3. */
function round(value: number, step: number): number {
  const decimals = Math.max(0, -Math.floor(Math.log10(step > 0 ? step : 1)));
  return Number(value.toFixed(Math.min(decimals, 6)));
}

interface ColorFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  /** alpha adds an opacity slider, for shadows and backgrounds. */
  alpha?: boolean;
  help?: ReactNode;
  disabled?: boolean;
}

/** ColorField picks a color, and its opacity when asked. */
export function ColorField({ label, value, onChange, alpha = false, help, disabled = false }: ColorFieldProps) {
  const id = useId();
  const { hex, alpha: opacity } = splitColor(value);
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center gap-1">
        <label htmlFor={id} className="text-sm font-medium text-fg">
          {label}
        </label>
        {help}
      </div>
      <div className="flex items-center gap-3">
        <input
          id={id}
          type="color"
          value={hex.toLowerCase()}
          disabled={disabled}
          onChange={(event) => onChange(joinColor(event.target.value, opacity))}
          className="h-9 w-14 cursor-pointer rounded-control border border-line-strong bg-surface p-1 disabled:opacity-55"
        />
        <span className="font-mono text-xs text-fg-muted">{hex}</span>
        {alpha && (
          <label className="flex flex-1 items-center gap-2 text-xs text-fg-muted">
            Opacidade
            <input
              type="range"
              min={0}
              max={100}
              step={5}
              value={Math.round(opacity * 100)}
              disabled={disabled}
              onChange={(event) => onChange(joinColor(hex, Number(event.target.value) / 100))}
              className="h-2 min-w-0 flex-1 accent-[var(--sp-accent)]"
            />
            <span className="w-9 text-right tabular-nums">{Math.round(opacity * 100)}%</span>
          </label>
        )}
      </div>
    </div>
  );
}

interface SegmentedProps {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
  help?: ReactNode;
}

/** Segmented is a small set of buttons where one is chosen. */
export function Segmented({ label, value, options, onChange, help }: SegmentedProps) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center gap-1">
        <span className="text-sm font-medium text-fg">{label}</span>
        {help}
      </div>
      <div role="radiogroup" aria-label={label} className="inline-flex w-fit rounded-control border border-line-strong p-0.5">
        {options.map((option) => (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={option.value === value}
            onClick={() => onChange(option.value)}
            className={`h-8 rounded-control px-3 text-sm transition-colors duration-[var(--duration-fast)] ${
              option.value === value ? "bg-accent text-on-accent" : "text-fg-secondary hover:bg-sunken hover:text-fg"
            }`}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}
