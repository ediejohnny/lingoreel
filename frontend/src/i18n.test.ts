import { afterEach, describe, expect, it } from "vitest";
import { fallback, language, onLanguage, setLanguage, t } from "./i18n";

describe("t", () => {
  it("gives a key's text", () => {
    expect(t("ui.app.name")).toBe("LingoReel");
  });

  it("fills in the parameters", () => {
    expect(t("ui.app.theme", { current: "Tema claro", next: "tema escuro" })).toBe("Tema claro. Mudar para tema escuro");
    expect(t("ui.passage.progressGoal", { start: "0:10", end: "0:20", lap: 2, goal: 5 })).toBe("Trecho 0:10–0:20 · repetição 2 de 5");
  });

  it("shows a key no catalog has as itself", () => {
    expect(t("ui.no.such.key")).toBe("ui.no.such.key");
  });

  it("picks the plural form by the count", () => {
    expect(t("ui.karaoke.repeat.times", { count: 1 })).toBe("1 vez");
    expect(t("ui.karaoke.repeat.times", { count: 2 })).toBe("2 vezes");
  });
});

describe("setLanguage", () => {
  afterEach(() => setLanguage(fallback));

  it("changes the language at once and tells the screens, once per change", () => {
    let told = 0;
    const stop = onLanguage(() => told++);
    setLanguage("en-US");
    expect(language()).toBe("en-US");
    expect(t("ui.settings.language.saved")).toBe("Language saved. The app speaks it now.");
    setLanguage("en-US");
    expect(told).toBe(1);
    // A language without a catalog speaks the fallback's.
    setLanguage("xx");
    expect(language()).toBe(fallback);
    expect(told).toBe(2);
    stop();
    setLanguage("en-US");
    expect(told).toBe(2);
  });
});
