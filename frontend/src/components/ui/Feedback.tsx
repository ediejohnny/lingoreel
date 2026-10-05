import { CircleAlert, Info, TriangleAlert, X } from "lucide-react";
import type { ReactNode } from "react";

type Tone = "error" | "warning" | "info";

const tones: Record<Tone, { box: string; icon: typeof Info }> = {
  error: { box: "border-error/40 bg-error-soft text-fg", icon: CircleAlert },
  warning: { box: "border-warning/40 bg-warning-soft text-fg", icon: TriangleAlert },
  info: { box: "border-line bg-accent-soft text-fg", icon: Info },
};

interface BannerProps {
  tone: Tone;
  children: ReactNode;
  /** action is a button such as "Tentar novamente". */
  action?: ReactNode;
  onClose?: () => void;
}

/** Banner states a problem or a fact and offers what to do; errors stay until closed. */
export function Banner({ tone, children, action, onClose }: BannerProps) {
  const { box, icon: Icon } = tones[tone];
  return (
    <div role={tone === "error" ? "alert" : "status"} className={`flex items-start gap-3 rounded-card border px-4 py-3 text-sm ${box}`}>
      <Icon
        aria-hidden
        className={`mt-0.5 size-4 shrink-0 ${tone === "error" ? "text-error" : tone === "warning" ? "text-warning" : "text-accent"}`}
      />
      <div className="flex-1 leading-relaxed">{children}</div>
      {action}
      {onClose && (
        <button type="button" aria-label="Fechar aviso" onClick={onClose} className="rounded-control p-1 text-fg-muted hover:text-fg">
          <X aria-hidden className="size-4" />
        </button>
      )}
    </div>
  );
}

interface CardProps {
  title?: string;
  /** aside goes at the right of the title, such as a (?) help. */
  aside?: ReactNode;
  children: ReactNode;
  className?: string;
}

/** Card groups related controls on a surface. */
export function Card({ title, aside, children, className = "" }: CardProps) {
  return (
    <section className={`rounded-card border border-line bg-surface p-5 shadow-card ${className}`}>
      {title && (
        <header className="mb-4 flex items-center gap-1">
          <h2 className="text-sm font-semibold tracking-wide text-fg-secondary uppercase">{title}</h2>
          {aside}
        </header>
      )}
      {children}
    </section>
  );
}
