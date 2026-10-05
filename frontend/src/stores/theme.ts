import { useEffect } from "react";
import { t } from "../i18n";
import { interfaceVariableNames, interfaceVariables } from "../interface";
import type { InterfaceSettings } from "../services/backend";

export type Theme = "system" | "light" | "dark";

const themes: Theme[] = ["system", "light", "dark"];

/** asTheme accepts only the known themes; anything else follows the system. */
export function asTheme(value: string): Theme {
  return (themes as string[]).includes(value) ? (value as Theme) : "system";
}

/** nextTheme cycles system → light → dark. */
export function nextTheme(theme: Theme): Theme {
  return themes[(themes.indexOf(theme) + 1) % themes.length] ?? "system";
}

/** themeLabel names a theme in the interface. */
export function themeLabel(theme: Theme): string {
  return { system: t("ui.theme.system"), light: t("ui.theme.light"), dark: t("ui.theme.dark") }[theme];
}

const darkQuery = "(prefers-color-scheme: dark)";

/**
 * useAppliedTheme applies a theme and the app's own look over it to the page
 * at once, without a restart of the window, and follows Windows while the
 * theme is "system".
 */
export function useAppliedTheme(theme: Theme, look: InterfaceSettings) {
  useEffect(() => {
    const media = window.matchMedia(darkQuery);
    const apply = () => {
      const dark = theme === "dark" || (theme === "system" && media.matches);
      const root = document.documentElement;
      root.dataset.theme = dark ? "dark" : "light";
      const variables = interfaceVariables(look, dark ? "dark" : "light");
      for (const name of interfaceVariableNames) {
        const value = variables[name];
        if (value === undefined) {
          root.style.removeProperty(name);
        } else {
          root.style.setProperty(name, value);
        }
      }
    };
    apply();
    media.addEventListener("change", apply);
    return () => media.removeEventListener("change", apply);
  }, [theme, look]);
}
