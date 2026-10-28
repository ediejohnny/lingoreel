import { t } from "./i18n";

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
