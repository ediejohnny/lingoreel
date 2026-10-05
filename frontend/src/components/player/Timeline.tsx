import { useState } from "react";
import { progress } from "../../format";

interface TimelineProps {
  position: number;
  duration: number;
  positionText: string;
  durationText: string;
  disabled: boolean;
  onSeek: (seconds: number) => void;
}

/**
 * Timeline shows where the video is and seeks with a click or a drag; while
 * dragging, the bar follows the hand and the video moves on release.
 */
export function Timeline({ position, duration, positionText, durationText, disabled, onSeek }: TimelineProps) {
  const [dragging, setDragging] = useState<number | null>(null);
  const shown = dragging ?? position;
  const share = progress(shown, duration) * 100;
  const release = () => {
    if (dragging !== null) {
      onSeek(dragging);
      setDragging(null);
    }
  };
  return (
    <div className="flex items-center gap-3">
      <span className="w-16 text-right text-sm text-fg-secondary tabular-nums">{positionText}</span>
      <input
        type="range"
        aria-label="Posição do vídeo"
        aria-valuetext={positionText}
        min={0}
        max={Math.max(duration, 0)}
        step={0.1}
        value={shown}
        disabled={disabled || duration <= 0}
        onChange={(event) => setDragging(Number(event.target.value))}
        onPointerUp={release}
        onKeyUp={release}
        style={{ background: `linear-gradient(to right, var(--sp-accent) ${share}%, var(--sp-border) ${share}%)` }}
        className="h-1.5 flex-1 cursor-pointer appearance-none rounded-control accent-[var(--sp-accent)] disabled:cursor-not-allowed"
      />
      <span className="w-16 text-sm text-fg-muted tabular-nums">{durationText}</span>
    </div>
  );
}
