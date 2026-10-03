import { Keyboard, RotateCcw, X } from "lucide-react";
import { useEffect, useState } from "react";
import { backend, messageOf } from "../../services/backend";
import { captureOf } from "../../settings";
import { IconButton } from "../ui/Button";

/** noShortcut is what a command without keys shows. */
const noShortcut = "Sem atalho";

interface ShortcutCaptureProps {
  /** name is the command, for screen readers. */
  name: string;
  label: string;
  /** taken names the command already on a chord, or undefined when free. */
  taken: (chord: string) => string | undefined;
  onChange: (chord: string, label: string) => void;
  /** onClear leaves the command without a shortcut; undefined when it has none. */
  onClear?: () => void;
  /** onDefault puts the command's default back; undefined when it is already the default. */
  onDefault?: () => void;
}

/**
 * ShortcutCapture shows a command's keys; clicking it listens to the next
 * keys pressed (Esc alone gives up). The Go side checks the chord, and a
 * chord another command uses is refused here, naming that command.
 */
export function ShortcutCapture({ name, label, taken, onChange, onClear, onDefault }: ShortcutCaptureProps) {
  const [listening, setListening] = useState(false);
  const [problem, setProblem] = useState("");

  useEffect(() => {
    if (!listening) {
      return;
    }
    const take = (chord: string) => {
      setListening(false);
      backend
        .checkShortcut(chord)
        .then((checked) => {
          const owner = taken(checked.chord);
          if (owner) {
            setProblem(`“${checked.label}” já é o atalho de “${owner}”.`);
            return;
          }
          setProblem("");
          onChange(checked.chord, checked.label);
        })
        .catch((failure) => setProblem(messageOf(failure)));
    };
    const down = (event: KeyboardEvent) => {
      event.preventDefault();
      event.stopPropagation();
      const capture = captureOf(event);
      switch (capture.kind) {
        case "wait":
          return;
        case "cancel":
          setListening(false);
          return;
        case "unsupported":
          setProblem("A tecla do Windows não pode fazer parte de um atalho.");
          setListening(false);
          return;
      }
      // The right Alt is taken on its release, which is also kept from
      // opening the window menu.
      if (capture.chord !== "AltRight") {
        take(capture.chord);
      }
    };
    const up = (event: KeyboardEvent) => {
      event.preventDefault();
      event.stopPropagation();
      if (event.code === "AltRight") {
        take("AltRight");
      }
    };
    // Captured before anything else, so no key reaches the rest of the page.
    window.addEventListener("keydown", down, true);
    window.addEventListener("keyup", up, true);
    return () => {
      window.removeEventListener("keydown", down, true);
      window.removeEventListener("keyup", up, true);
    };
  }, [listening, taken, onChange]);

  return (
    <div className="flex flex-col items-end gap-1">
      <div className="flex items-center gap-1">
        <button
          type="button"
          aria-label={`${name}: ${label || noShortcut}. Clique e pressione as novas teclas`}
          aria-pressed={listening}
          onClick={() => {
            setProblem("");
            setListening((value) => !value);
          }}
          onBlur={() => setListening(false)}
          className={`inline-flex h-9 min-w-36 items-center justify-center gap-2 rounded-control border px-3 text-sm font-medium ${
            listening ? "border-accent bg-accent-soft text-accent" : "border-line-strong bg-surface text-fg hover:bg-sunken"
          }`}
        >
          <Keyboard aria-hidden className="size-4" />
          {listening ? (
            "Pressione as teclas…"
          ) : label ? (
            <kbd className="font-sans">{label}</kbd>
          ) : (
            <span className="font-normal text-fg-muted">{noShortcut}</span>
          )}
        </button>
        {onClear ? <IconButton icon={X} label={`Deixar “${name}” sem atalho`} onClick={onClear} /> : <span className="size-9" />}
        {onDefault ? (
          <IconButton icon={RotateCcw} label={`Voltar ao atalho padrão de “${name}”`} onClick={onDefault} />
        ) : (
          <span className="size-9" />
        )}
      </div>
      {problem && (
        <p role="alert" className="max-w-xs text-right text-xs text-error">
          {problem}
        </p>
      )}
    </div>
  );
}
