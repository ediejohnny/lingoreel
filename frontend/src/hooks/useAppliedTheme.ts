import { useEffect } from "react";
import { interfaceVariableNames, interfaceVariables } from "../interface";
import type { InterfaceSettings } from "../services/backend";
import type { Theme } from "../theme";

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
