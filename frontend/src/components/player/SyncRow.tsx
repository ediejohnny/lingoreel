import { Minus, Plus, RotateCcw } from "lucide-react";
import { t } from "../../i18n";
import { IconButton } from "../ui/Button";

interface SyncRowProps {
  name: string;
  offset: string;
  /** voice is what keeping the line on the voice adds to the offset ("+0,800 s"); empty when nothing. */
  voice?: string;
  shortcut: string;
  onEarlier: () => void;
  onLater: () => void;
  onReset: () => void;
}

/** SyncRow moves one subtitle line earlier or later, as its shortcut does, and tells what the voice added. */
export function SyncRow({ name, offset, voice, shortcut, onEarlier, onLater, onReset }: SyncRowProps) {
  return (
    // In a narrow panel the buttons go below the name instead of squeezing it.
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
      <div className="min-w-40 flex-1">
        <p className="truncate text-sm font-medium text-fg">{name}</p>
        <p className="text-xs text-fg-muted">{shortcut}</p>
        {voice && <p className="text-xs text-accent">{t("ui.sync.voice", { shift: voice })}</p>}
      </div>
      <div className="ml-auto flex items-center gap-2">
        <IconButton icon={Minus} label={t("ui.sync.earlier", { name })} onClick={onEarlier} />
        <span className="w-20 text-center text-sm text-fg tabular-nums" aria-live="polite">
          {offset}
        </span>
        <IconButton icon={Plus} label={t("ui.sync.later", { name })} onClick={onLater} />
        <IconButton icon={RotateCcw} label={t("ui.sync.reset", { name })} onClick={onReset} />
      </div>
    </div>
  );
}
