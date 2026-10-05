import { CircleHelp } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";

interface HelpTipProps {
  /** topic names what the help is about, for screen readers. */
  topic: string;
  children: string;
}

/**
 * HelpTip is the (?) next to a control whose effect is not obvious
 * on its own: it opens with a click, Enter or Space and closes with Esc
 * or a click elsewhere.
 */
export function HelpTip({ topic, children }: HelpTipProps) {
  const [open, setOpen] = useState(false);
  const id = useId();
  const root = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    const close = (event: Event) => {
      if (event instanceof KeyboardEvent ? event.key === "Escape" : !root.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("keydown", close);
    document.addEventListener("mousedown", close);
    return () => {
      document.removeEventListener("keydown", close);
      document.removeEventListener("mousedown", close);
    };
  }, [open]);

  return (
    <span ref={root} className="relative inline-flex">
      <button
        type="button"
        aria-label={`Ajuda: ${topic}`}
        aria-expanded={open}
        aria-controls={id}
        onClick={() => setOpen((value) => !value)}
        className="inline-flex size-7 items-center justify-center rounded-full text-fg-muted hover:bg-sunken hover:text-fg"
      >
        <CircleHelp aria-hidden className="size-4" />
      </button>
      {open && (
        <span
          id={id}
          role="note"
          className="absolute top-8 left-1/2 z-20 w-72 -translate-x-1/2 rounded-card border border-line bg-raised p-3 text-sm leading-relaxed text-fg-secondary shadow-card"
        >
          {children}
        </span>
      )}
    </span>
  );
}
