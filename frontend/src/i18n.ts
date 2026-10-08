// The interface's texts, from the same catalogs the Go side reads: one
// folder per language under internal/presentation/i18n/locales, whose JSON
// files hold the texts by key. A text has named parameters ("Volume
// {percent}%") or plural forms picked by its "count", with the language's
// own rules (Intl.PluralRules).

import { useSyncExternalStore } from "react";

type Forms = Partial<Record<Intl.LDMLPluralRule, string>> & { other: string };
type Entry = string | Forms;
type Catalog = Record<string, Entry>;

/** fallback is the complete catalog every other one falls back to. */
export const fallback = "pt-BR";

const files = import.meta.glob<Catalog>("../../internal/presentation/i18n/locales/*/*.json", { eager: true, import: "default" });

/** catalogs merges each language's files. */
const catalogs: Record<string, Catalog> = {};
for (const [file, entries] of Object.entries(files)) {
  const tag = file.split("/").slice(-2)[0] ?? fallback;
  catalogs[tag] = { ...catalogs[tag], ...entries };
}

let current = fallback;

/** listeners hear the language change: the screens draw themselves again in it. */
const listeners = new Set<() => void>();

/** languages lists the languages with a catalog. */
export function languages(): string[] {
  return Object.keys(catalogs).sort();
}

/** setLanguage makes a language the interface's, at once: its catalog, or the fallback's when it has none. */
export function setLanguage(tag: string): void {
  const next = catalogs[tag] ? tag : fallback;
  if (next === current) {
    return;
  }
  current = next;
  if (typeof document !== "undefined") {
    document.documentElement.lang = current;
  }
  listeners.forEach((listener) => listener());
}

/** language is the interface's language. */
export function language(): string {
  return current;
}

/** onLanguage calls a listener at each change of language; it returns what stops it. */
export function onLanguage(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** useLanguage is the interface's language for a component, which draws itself again when it changes. */
export function useLanguage(): string {
  return useSyncExternalStore(onLanguage, language);
}

/** decimal is the interface's decimal separator: "," in Brazilian Portuguese, "." in English. */
export function decimal(): string {
  return new Intl.NumberFormat(current).formatToParts(1.5).find((part) => part.type === "decimal")?.value ?? ".";
}

export type Params = Record<string, string | number>;

/**
 * t is a key's text with its parameters filled in. Keys are always written
 * whole, never built, so a test finds each one in the catalog; a key no
 * catalog has shows as itself.
 */
export function t(key: string, params: Params = {}): string {
  const entry = catalogs[current]?.[key] ?? catalogs[fallback]?.[key];
  if (entry === undefined) {
    return key;
  }
  let text: string;
  if (typeof entry === "string") {
    text = entry;
  } else {
    const count = Number(params.count ?? 0);
    text = entry[new Intl.PluralRules(current).select(count)] ?? entry.other;
  }
  return text.replace(/\{([a-zA-Z]+)\}/g, (whole, name: string) => (name in params ? String(params[name]) : whole));
}
