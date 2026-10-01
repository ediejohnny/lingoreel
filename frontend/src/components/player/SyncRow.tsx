import { Minus, Plus, RotateCcw } from "lucide-react";
import { IconButton } from "../ui/Button";

interface SyncRowProps {
  name: string;
  offset: string;
  shortcut: string;
  onEarlier: () => void;
  onLater: () => void;
  onReset: () => void;
}

/** SyncRow moves one subtitle line earlier or later, as its shortcut does. */
export function SyncRow({ name, offset, shortcut, onEarlier, onLater, onReset }: SyncRowProps) {
  return (
    <div className="flex items-center gap-2">
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-fg">{name}</p>
        <p className="text-xs text-fg-muted">{shortcut}</p>
      </div>
      <IconButton icon={Minus} label={`${name}: mais cedo`} onClick={onEarlier} />
      <span className="w-20 text-center text-sm text-fg tabular-nums" aria-live="polite">
        {offset}
      </span>
      <IconButton icon={Plus} label={`${name}: mais tarde`} onClick={onLater} />
      <IconButton icon={RotateCcw} label={`${name}: voltar ao tempo do arquivo`} onClick={onReset} />
    </div>
  );
}
