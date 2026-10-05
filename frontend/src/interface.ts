// The app's own look over its theme: the accent, the font and the text size
// the user chose, as the CSS variables of styles/app.css that apply them.

import type { InterfaceSettings } from "./services/backend";

/** Shade is the theme in effect: "system" already resolved. */
export type Shade = "light" | "dark";

/** interfaceVariableNames are the variables a look may set; one it leaves out keeps the theme's own. */
export const interfaceVariableNames = [
  "--sp-accent",
  "--sp-accent-hover",
  "--sp-accent-soft",
  "--sp-on-accent",
  "--sp-focus",
  "--sp-font",
  "--sp-text-size",
] as const;

/** defaultFont is the app's own font, behind a chosen family that lacks a letter. */
const defaultFont = '"Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif';

type RGB = [number, number, number];

const white: RGB = [255, 255, 255];
const black: RGB = [0, 0, 0];
/** onAccentDark is the dark theme's text on its accent; darkSurface its cards. */
const onAccentDark = "#0b1220";
const darkSurface: RGB = [0x15, 0x1a, 0x21];

/** How far the derived colors move from the accent, as the themes' own blues do. */
const hoverDarker = 0.15;
const hoverLighter = 0.2;
const softLight = 0.1;
const softDark = 0.18;

/**
 * interfaceVariables gives the CSS variables of a look in a theme. Without a
 * chosen accent the theme keeps its own; with one, its hover is darker in the
 * light theme and lighter in the dark one, and the text on it is the one that
 * reads best (WCAG contrast).
 */
export function interfaceVariables(look: InterfaceSettings, shade: Shade): Record<string, string> {
  const variables: Record<string, string> = { "--sp-text-size": String(look.textSize) };
  const accent = parseHex(look.accent);
  if (accent) {
    const hover = shade === "light" ? mix(accent, black, hoverDarker) : mix(accent, white, hoverLighter);
    variables["--sp-accent"] = hex(accent);
    variables["--sp-accent-hover"] = hex(hover);
    variables["--sp-accent-soft"] = hex(shade === "light" ? mix(white, accent, softLight) : mix(darkSurface, accent, softDark));
    variables["--sp-on-accent"] = contrast(accent, white) >= contrast(accent, parseHex(onAccentDark) ?? black) ? "#ffffff" : onAccentDark;
    variables["--sp-focus"] = hex(shade === "light" ? accent : hover);
  }
  // A family name never closes its quotes or escapes from them.
  const family = look.fontFamily.replace(/["\\\r\n]/g, "").trim();
  if (family !== "") {
    variables["--sp-font"] = `"${family}", ${defaultFont}`;
  }
  return variables;
}

/** parseHex reads "#RRGGBB" (an alpha after it is ignored); anything else is no color. */
function parseHex(value: string): RGB | null {
  const match = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})(?:[0-9a-f]{2})?$/i.exec(value.trim());
  if (!match) {
    return null;
  }
  return [parseInt(match[1] ?? "", 16), parseInt(match[2] ?? "", 16), parseInt(match[3] ?? "", 16)];
}

function hex(color: RGB): string {
  return "#" + color.map((channel) => channel.toString(16).padStart(2, "0")).join("");
}

/** mix moves a color toward another by a share (0 keeps it, 1 is the other). */
function mix(from: RGB, to: RGB, share: number): RGB {
  return from.map((channel, i) => Math.round(channel + ((to[i] ?? channel) - channel) * share)) as RGB;
}

/** luminance is WCAG's relative luminance. */
function luminance(color: RGB): number {
  const [r, g, b] = color.map((channel) => {
    const value = channel / 255;
    return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  }) as RGB;
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** contrast is WCAG's contrast ratio of two colors. */
function contrast(a: RGB, b: RGB): number {
  const [light, dark] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number];
  return (light + 0.05) / (dark + 0.05);
}
