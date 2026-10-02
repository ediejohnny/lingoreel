import type { LucideIcon } from "lucide-react";
import { LoaderCircle } from "lucide-react";
import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "secondary" | "ghost";

const variants: Record<Variant, string> = {
  primary: "bg-accent text-on-accent hover:bg-accent-hover border-transparent",
  secondary: "bg-surface text-fg border-line-strong hover:bg-sunken",
  ghost: "bg-transparent text-fg-secondary border-transparent hover:bg-sunken hover:text-fg",
};

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  icon?: LucideIcon;
  loading?: boolean;
  size?: "md" | "lg";
  children: ReactNode;
}

/** Button with an optional icon; disabled and loading look unavailable but stay readable. */
export function Button({
  variant = "secondary",
  icon: Icon,
  loading = false,
  size = "md",
  children,
  disabled,
  className = "",
  ...rest
}: ButtonProps) {
  const sizing = size === "lg" ? "h-11 px-5 text-base" : "h-9 px-3.5 text-sm";
  return (
    <button
      type="button"
      {...rest}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={`inline-flex items-center justify-center gap-2 rounded-control border font-medium transition-colors duration-[var(--duration-fast)] disabled:cursor-not-allowed disabled:opacity-55 ${sizing} ${variants[variant]} ${className}`}
    >
      {loading ? <LoaderCircle aria-hidden className="size-4 animate-spin" /> : Icon && <Icon aria-hidden className="size-4" />}
      {children}
    </button>
  );
}

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon: LucideIcon;
  /** label names the action for screen readers and the tooltip. */
  label: string;
  size?: "md" | "lg";
  active?: boolean;
}

/** IconButton has a comfortable click area larger than its icon (DESIGN §48). */
export function IconButton({ icon: Icon, label, size = "md", active = false, className = "", ...rest }: IconButtonProps) {
  const sizing = size === "lg" ? "size-12" : "size-9";
  const tone = active ? "bg-accent-soft text-accent" : "text-fg-secondary hover:bg-sunken hover:text-fg";
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      {...rest}
      className={`inline-flex items-center justify-center rounded-control transition-colors duration-[var(--duration-fast)] disabled:cursor-not-allowed disabled:opacity-45 ${sizing} ${tone} ${className}`}
    >
      <Icon aria-hidden className={size === "lg" ? "size-6" : "size-[18px]"} />
    </button>
  );
}
