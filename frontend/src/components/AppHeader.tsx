import { Monitor, Moon, Sun } from "lucide-react";
import type { ReactNode } from "react";
import { nextTheme, themeLabel, type Theme } from "../stores/theme";
import { IconButton } from "./ui/Button";

const themeIcons = { system: Monitor, light: Sun, dark: Moon };

interface AppHeaderProps {
  theme: Theme;
  onTheme: (theme: Theme) => void;
  /** leading goes before the title, such as a back button. */
  leading?: ReactNode;
  subtitle?: string;
}

/** AppHeader is the one bar at the top: where you are and the theme. */
export function AppHeader({ theme, onTheme, leading, subtitle }: AppHeaderProps) {
  const next = nextTheme(theme);
  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b border-line bg-surface px-4">
      {leading}
      <div className="flex min-w-0 flex-1 items-baseline gap-3">
        <span className="font-semibold text-fg">LingoReel</span>
        {subtitle && <span className="truncate text-sm text-fg-muted">{subtitle}</span>}
      </div>
      <IconButton
        icon={themeIcons[theme]}
        label={`${themeLabel(theme)}. Mudar para ${themeLabel(next).toLowerCase()}`}
        onClick={() => onTheme(next)}
      />
    </header>
  );
}
