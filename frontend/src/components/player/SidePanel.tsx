import { useRef, useState, type ReactNode } from "react";
import { panelWidth } from "../../stage";

export type PanelTab = "playback" | "settings";

interface SidePanelProps {
  width: number;
  tab: PanelTab;
  onTab: (tab: PanelTab) => void;
  /** onWidth gets the width chosen by dragging the panel's edge, once the drag ends. */
  onWidth: (width: number) => void;
  playback: ReactNode;
  settings: ReactNode;
}

const tabs: { id: PanelTab; label: string }[] = [
  { id: "playback", label: "Reprodução" },
  { id: "settings", label: "Configurações" },
];

/**
 * SidePanel is the panel beside the video: the playback controls the bar
 * over the video has no room for, and the settings. Its left edge drags to
 * make it wider, up to half the window.
 */
export function SidePanel({ width, tab, onTab, onWidth, playback, settings }: SidePanelProps) {
  const [dragged, setDragged] = useState<number | null>(null);
  const dragging = useRef(false);
  const shown = dragged ?? panelWidth(width, window.innerWidth);
  const widthAt = (clientX: number) => panelWidth(window.innerWidth - clientX, window.innerWidth);

  return (
    <aside aria-label="Painel" style={{ width: shown }} className="relative flex min-h-0 shrink-0 flex-col border-l border-line bg-bg">
      <div
        role="separator"
        aria-orientation="vertical"
        aria-label="Largura do painel"
        className="absolute inset-y-0 -left-1 z-10 w-2 cursor-col-resize hover:bg-accent-soft"
        onPointerDown={(event) => {
          dragging.current = true;
          event.currentTarget.setPointerCapture(event.pointerId);
          setDragged(widthAt(event.clientX));
        }}
        onPointerMove={(event) => dragging.current && setDragged(widthAt(event.clientX))}
        onPointerUp={(event) => {
          if (!dragging.current) {
            return;
          }
          dragging.current = false;
          const chosen = widthAt(event.clientX);
          setDragged(null);
          onWidth(chosen);
        }}
      />
      <div role="tablist" aria-label="Seções do painel" className="flex shrink-0 gap-1 border-b border-line bg-surface px-3 pt-2">
        {tabs.map((item) => (
          <button
            key={item.id}
            type="button"
            role="tab"
            aria-selected={tab === item.id}
            onClick={() => onTab(item.id)}
            className={`-mb-px rounded-t-control border-b-2 px-3 py-2 text-sm ${
              tab === item.id ? "border-accent font-medium text-accent" : "border-transparent text-fg-secondary hover:text-fg"
            }`}
          >
            {item.label}
          </button>
        ))}
      </div>
      <div role="tabpanel" className="flex min-h-0 flex-1 flex-col">
        {tab === "settings" ? settings : playback}
      </div>
    </aside>
  );
}
