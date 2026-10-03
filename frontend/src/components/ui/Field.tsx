import { useId } from "react";
import type { ReactNode } from "react";

interface ToggleProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  /** help goes next to the label, such as a (?) help. */
  help?: ReactNode;
  disabled?: boolean;
}

/** Toggle is a labelled checkbox with room for a (?) help. */
export function Toggle({ label, checked, onChange, help, disabled = false }: ToggleProps) {
  const id = useId();
  return (
    <div className="flex items-center gap-2">
      <input
        id={id}
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        className="size-4 accent-[var(--sp-accent)] disabled:opacity-55"
      />
      <label htmlFor={id} className="text-sm text-fg">
        {label}
      </label>
      {help}
    </div>
  );
}

interface TextFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  help?: ReactNode;
  /** action goes after the field, such as a "Marcar" button. */
  action?: ReactNode;
  /** inputMode is "decimal" for times, the default, or "text" for names. */
  inputMode?: "decimal" | "text";
}

/** TextField is a labelled one-line text input. */
export function TextField({ label, value, onChange, placeholder, help, action, inputMode = "decimal" }: TextFieldProps) {
  const id = useId();
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center gap-1">
        <label htmlFor={id} className="text-sm font-medium text-fg">
          {label}
        </label>
        {help}
      </div>
      <div className="flex gap-2">
        <input
          id={id}
          value={value}
          placeholder={placeholder}
          inputMode={inputMode}
          onChange={(event) => onChange(event.target.value)}
          className="h-10 w-full min-w-0 rounded-control border border-line-strong bg-surface px-3 text-sm text-fg"
        />
        {action}
      </div>
    </div>
  );
}
