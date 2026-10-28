import { Monitor, Moon, Sun } from "lucide-react";
import type { ReactNode } from "react";
import { t } from "../i18n";
import { nextTheme, themeLabel, type Theme } from "../theme";
import { Logo } from "./Logo";
import { IconButton } from "./ui/Button";

const themeIcons = { system: Monitor, light: Sun, dark: Moon };

interface AppHeaderProps {
  theme: Theme;
  onTheme: (theme: Theme) => void;
  /** leading goes before the title, such as a back button. */
  leading?: ReactNode;
  subtitle?: string;
  /** actions go before the theme button, such as the settings. */
  actions?: ReactNode;
}

/** AppHeader is the one bar at the top: where you are and the theme. */
export function AppHeader({ theme, onTheme, leading, subtitle, actions }: AppHeaderProps) {
  const next = nextTheme(theme);
  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b border-line bg-surface px-4">
      {leading}
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <span className="flex shrink-0 items-center gap-2">
          <Logo />
          <span className="font-semibold text-fg">{t("ui.app.name")}</span>
        </span>
        {subtitle && <span className="truncate text-sm text-fg-muted">{subtitle}</span>}
      </div>
      <div className="flex items-center gap-2">
        {actions && (
          <>
            {actions}
            <span aria-hidden className="mx-1 h-6 w-px bg-line" />
          </>
        )}
        <IconButton
          icon={themeIcons[theme]}
          label={t("ui.app.theme", { current: themeLabel(theme), next: themeLabel(next).toLowerCase() })}
          onClick={() => onTheme(next)}
        />
      </div>
    </header>
  );
}
