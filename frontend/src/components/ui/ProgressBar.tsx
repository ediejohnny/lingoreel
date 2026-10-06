interface ProgressBarProps {
  label: string;
  /** percent is how far the wait went, from 0 to 100. */
  percent: number;
}

/** ProgressBar shows how far a wait has gone: a bar that fills and its percent, so the app never looks stuck. */
export function ProgressBar({ label, percent }: ProgressBarProps) {
  const value = Math.max(0, Math.min(100, Math.round(Number.isFinite(percent) ? percent : 0)));
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-baseline justify-between gap-3 text-sm text-fg">
        <span>{label}</span>
        <span className="text-fg-muted tabular-nums">{value}%</span>
      </div>
      <div
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={value}
        className="h-2 overflow-hidden rounded-control bg-line"
      >
        <div className="h-full bg-accent transition-[width] duration-300" style={{ width: `${value}%` }} />
      </div>
    </div>
  );
}
