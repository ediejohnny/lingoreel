import { ChevronDown } from "./icons";
import { useId } from "react";
import type { ReactNode } from "react";

export interface Option {
  value: string;
  label: string;
  disabled?: boolean;
}

interface SelectProps {
  label: string;
  value: string;
  options: Option[];
  onChange: (value: string) => void;
  /** help goes next to the label, such as a (?) help. */
  help?: ReactNode;
  /** note explains the chosen option below the field. */
  note?: string;
  warning?: boolean;
  disabled?: boolean;
}

/** Select is a labelled native list, keyboard friendly by default. */
export function Select({ label, value, options, onChange, help, note, warning = false, disabled = false }: SelectProps) {
  const id = useId();
  const noteId = `${id}-note`;
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center gap-1">
        <label htmlFor={id} className="text-sm font-medium text-fg">
          {label}
        </label>
        {help}
      </div>
      <div className="relative">
        <select
          id={id}
          value={value}
          disabled={disabled}
          aria-describedby={note ? noteId : undefined}
          onChange={(event) => onChange(event.target.value)}
          className="h-10 w-full appearance-none rounded-control border border-line-strong bg-surface pr-9 pl-3 text-sm text-fg disabled:opacity-55"
        >
          {options.map((option) => (
            <option key={option.value} value={option.value} disabled={option.disabled}>
              {option.label}
            </option>
          ))}
        </select>
        <ChevronDown aria-hidden className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-fg-muted" />
      </div>
      {note && (
        <p id={noteId} className={`text-xs leading-relaxed ${warning ? "text-warning" : "text-fg-muted"}`}>
          {note}
        </p>
      )}
    </div>
  );
}
