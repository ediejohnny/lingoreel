import { useEffect } from "react";

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
  return { system: "Tema do Windows", light: "Tema claro", dark: "Tema escuro" }[theme];
}

const darkQuery = "(prefers-color-scheme: dark)";

/**
 * useAppliedTheme applies a theme to the page at once, without a restart
 * of the window, and follows Windows while it is "system".
 */
export function useAppliedTheme(theme: Theme) {
  useEffect(() => {
    const media = window.matchMedia(darkQuery);
    const apply = () => {
      const dark = theme === "dark" || (theme === "system" && media.matches);
      document.documentElement.dataset.theme = dark ? "dark" : "light";
    };
    apply();
    media.addEventListener("change", apply);
    return () => media.removeEventListener("change", apply);
  }, [theme]);
}
