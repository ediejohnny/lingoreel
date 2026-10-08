import { ArrowLeft, ArrowRight, Check, Download, ExternalLink } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { FFmpegRow } from "../components/settings/FFmpegRow";
import { Button } from "../components/ui/Button";
import { Banner } from "../components/ui/Feedback";
import { HelpTip } from "../components/ui/HelpTip";
import { Select } from "../components/ui/Select";
import { setLanguage, t } from "../i18n";
import { backend, messageOf, type Players, type VoiceSettings, type WelcomeView } from "../services/backend";
import { stepAfter, welcomeSteps, type WelcomeStep } from "../welcome";

interface WelcomePageProps {
  /** onDone is told once the welcome was answered or skipped. */
  onDone: () => void;
}

/** modeDescription says what a study mode does, by its value. */
function modeDescription(mode: string): string {
  switch (mode) {
    case "dual":
      return t("ui.welcome.mode.dual");
    case "on-demand":
      return t("ui.welcome.mode.onDemand");
    case "always":
      return t("ui.welcome.mode.always");
  }
  return "";
}

/**
 * WelcomePage asks, on the first run, what the app needs to fit the person: the app's language, the language they
 * speak and the one they study, how the translation shows, and the player and FFmpeg. Each step can be skipped, and
 * skipping keeps what Windows tells; opened again from the settings, it shows the active profile's choices.
 */
export function WelcomePage({ onDone }: WelcomePageProps) {
  const [view, setView] = useState<WelcomeView | null>(null);
  const [step, setStep] = useState<WelcomeStep>("language");
  const [spoken, setSpoken] = useState("");
  const [studied, setStudied] = useState("");
  const [mode, setMode] = useState("");
  const [preferred, setPreferred] = useState("");
  const [players, setPlayers] = useState<Players | null>(null);
  const [voice, setVoice] = useState<VoiceSettings | null>(null);
  const [installing, setInstalling] = useState("");
  const [error, setError] = useState("");
  const [finishing, setFinishing] = useState(false);

  // The view comes again in each language: its names are the app's.
  const load = useCallback(async () => {
    const next = await backend.welcome();
    setView(next);
    return next;
  }, []);

  useEffect(() => {
    backend
      .welcome()
      .then((next) => {
        setView(next);
        setSpoken(next.spoken);
        setStudied(next.studied);
        setMode(next.mode);
        setPreferred(next.preferredPlayer);
      })
      .catch((failure) => setError(messageOf(failure)));
  }, []);

  // The players and the FFmpeg, read when their step shows and after each change.
  const readTools = useCallback(() => Promise.all([backend.players(), backend.settings()]), []);
  const refreshTools = useCallback(async () => {
    const [found, settings] = await readTools();
    setPlayers(found);
    setVoice(settings.voice);
  }, [readTools]);
  useEffect(() => {
    if (step !== "tools") {
      return;
    }
    let current = true;
    readTools()
      .then(([found, settings]) => {
        if (current) {
          setPlayers(found);
          setVoice(settings.voice);
        }
      })
      .catch((failure) => current && setError(messageOf(failure)));
    return () => {
      current = false;
    };
  }, [step, readTools]);
  useEffect(
    () =>
      backend.onInstall((progress) => {
        if (progress.id === "ffmpeg") {
          return; // the FFmpeg row follows its own
        }
        setInstalling("");
        setError(progress.error);
        refreshTools().catch(() => undefined);
      }),
    [refreshTools],
  );

  const skip = useCallback(async () => {
    try {
      await backend.skipWelcome();
      onDone();
    } catch (failure) {
      setError(messageOf(failure));
    }
  }, [onDone]);

  // Esc skips, as the button does.
  useEffect(() => {
    const listener = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !event.defaultPrevented) {
        event.preventDefault();
        void skip();
      }
    };
    window.addEventListener("keydown", listener);
    return () => window.removeEventListener("keydown", listener);
  }, [skip]);

  const finish = async () => {
    setFinishing(true);
    setError("");
    try {
      await backend.finishWelcome({ spoken, studied, mode, preferredPlayer: preferred });
      onDone();
    } catch (failure) {
      setError(messageOf(failure));
    } finally {
      setFinishing(false);
    }
  };

  const chooseLanguage = async (tag: string) => {
    setError("");
    try {
      setLanguage(await backend.setLanguage(tag));
      await load();
    } catch (failure) {
      setError(messageOf(failure));
    }
  };

  // The FFmpeg row saves at once and then reads the tools again.
  const run = async (action: () => Promise<unknown>) => {
    setError("");
    try {
      await action();
      await refreshTools();
    } catch (failure) {
      setError(messageOf(failure));
    }
  };

  if (!view) {
    return (
      <main className="flex flex-1 items-center justify-center p-6">
        {error ? <Banner tone="error">{error}</Banner> : <p className="text-sm text-fg-muted">{t("ui.settings.loading")}</p>}
      </main>
    );
  }

  const index = welcomeSteps.indexOf(step);
  const last = index === welcomeSteps.length - 1;
  const supported = players?.installed.filter((item) => item.supported) ?? [];

  return (
    <main className="flex min-h-0 flex-1 justify-center overflow-y-auto p-4 sm:p-8">
      <div className="flex w-full max-w-2xl flex-col gap-5">
        <header className="flex flex-col gap-1">
          <p className="text-xs font-medium tracking-wide text-fg-muted uppercase">
            {t("ui.welcome.progress", { step: index + 1, total: welcomeSteps.length })}
          </p>
          <h1 className="text-xl font-semibold text-fg">
            {step === "language"
              ? t("ui.welcome.language.title")
              : step === "languages"
                ? t("ui.welcome.languages.title")
                : step === "mode"
                  ? t("ui.welcome.mode.title")
                  : t("ui.welcome.tools.title")}
          </h1>
          <p className="text-sm leading-relaxed text-fg-secondary">
            {step === "language"
              ? view.pending
                ? t("ui.welcome.language.intro")
                : t("ui.welcome.language.again")
              : step === "languages"
                ? t("ui.welcome.languages.intro")
                : step === "mode"
                  ? t("ui.welcome.mode.intro")
                  : t("ui.welcome.tools.intro")}
          </p>
        </header>

        <section className="flex flex-col gap-5 rounded-card border border-line bg-surface p-5 shadow-card">
          {step === "language" && (
            <Select
              label={t("ui.settings.language.label")}
              value={view.appLanguage}
              options={[
                { value: "", label: t("ui.settings.language.windows") },
                ...view.appLanguages.map((language) => ({ value: language.value, label: language.label })),
              ]}
              onChange={(tag) => void chooseLanguage(tag)}
              note={t("ui.welcome.language.note")}
            />
          )}

          {step === "languages" && (
            <>
              <Select
                label={t("ui.welcome.languages.spoken")}
                value={spoken}
                options={view.languages}
                onChange={setSpoken}
                note={t("ui.welcome.languages.spokenNote")}
              />
              <Select
                label={t("ui.welcome.languages.studied")}
                value={studied}
                options={view.languages}
                onChange={setStudied}
                note={spoken === studied ? t("ui.welcome.languages.same") : t("ui.welcome.languages.studiedNote")}
                warning={spoken === studied}
              />
              {!view.pending && <p className="text-xs leading-relaxed text-fg-muted">{t("ui.welcome.languages.profile")}</p>}
            </>
          )}

          {step === "mode" && (
            <div role="radiogroup" aria-label={t("ui.welcome.mode.title")} className="flex flex-col gap-2">
              {view.modes.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  role="radio"
                  aria-checked={mode === option.value}
                  onClick={() => setMode(option.value)}
                  className={`flex flex-col gap-1 rounded-control border px-4 py-3 text-left transition-colors duration-[var(--duration-fast)] ${
                    mode === option.value ? "border-accent bg-accent-soft" : "border-line hover:bg-sunken"
                  }`}
                >
                  <span className={`text-sm font-semibold ${mode === option.value ? "text-accent" : "text-fg"}`}>
                    {option.label}
                    {option.value === "on-demand" && (
                      <span className="ml-2 text-xs font-normal text-fg-muted">{t("ui.welcome.mode.recommended")}</span>
                    )}
                  </span>
                  <span className="text-sm leading-relaxed text-fg-secondary">{modeDescription(option.value)}</span>
                </button>
              ))}
            </div>
          )}

          {step === "tools" && (
            <>
              <div className="flex flex-col gap-3">
                <div className="flex items-center gap-1">
                  <h2 className="text-sm font-medium text-fg">{t("ui.welcome.tools.player")}</h2>
                  <HelpTip topic={t("ui.welcome.tools.player")}>{t("ui.welcome.tools.playerHelp")}</HelpTip>
                </div>
                {players === null ? (
                  <p className="text-sm text-fg-muted">{t("ui.welcome.tools.looking")}</p>
                ) : supported.length > 0 ? (
                  <Select
                    label={t("ui.settings.players.preferred.label")}
                    value={preferred}
                    options={[
                      { value: "", label: t("ui.settings.players.preferred.auto") },
                      ...supported.map((item) => ({ value: item.id, label: item.name })),
                    ]}
                    onChange={setPreferred}
                    note={t("ui.welcome.tools.found", { names: supported.map((item) => item.name).join(", ") })}
                  />
                ) : (
                  <>
                    <Banner tone="warning">{t("ui.install.none")}</Banner>
                    {players.recommendations.map((item) => (
                      <article key={item.id} className="flex flex-col gap-3 rounded-card border border-line bg-raised p-4">
                        <header className="flex items-center gap-1">
                          <h3 className="font-semibold text-fg">{item.name}</h3>
                          <HelpTip topic={t("ui.install.why", { name: item.name })}>{item.benefit}</HelpTip>
                        </header>
                        <div className="flex flex-wrap gap-2">
                          {item.autoInstall && (
                            <Button
                              variant="primary"
                              icon={Download}
                              loading={installing === item.id}
                              onClick={() => {
                                setInstalling(item.id);
                                setError("");
                                backend.installPlayer(item.id).catch((failure) => {
                                  setInstalling("");
                                  setError(messageOf(failure));
                                });
                              }}
                            >
                              {installing === item.id ? t("ui.install.installing") : t("ui.install.auto")}
                            </Button>
                          )}
                          <Button icon={ExternalLink} onClick={() => void run(() => backend.openDownloadPage(item.id))}>
                            {t("ui.install.download")}
                          </Button>
                        </div>
                      </article>
                    ))}
                  </>
                )}
              </div>
              {voice && (
                <FFmpegRow
                  voice={voice}
                  run={run}
                  disabled={false}
                  found={(path) => t("ui.settings.voice.ffmpeg.found", { path })}
                  missing={t("ui.welcome.tools.ffmpegMissing")}
                  help={t("ui.welcome.tools.ffmpegHelp")}
                />
              )}
            </>
          )}
        </section>

        {error && (
          <Banner tone="error" onClose={() => setError("")}>
            {error}
          </Banner>
        )}

        <footer className="flex flex-wrap items-center gap-2">
          <Button variant="ghost" onClick={() => void skip()}>
            {view.pending ? t("ui.welcome.skip") : t("ui.common.cancel")}
          </Button>
          <span className="flex-1" />
          {index > 0 && (
            <Button icon={ArrowLeft} onClick={() => setStep(stepAfter(step, "back"))}>
              {t("ui.welcome.back")}
            </Button>
          )}
          {last ? (
            <Button variant="primary" icon={Check} loading={finishing} disabled={spoken === studied} onClick={() => void finish()}>
              {t("ui.welcome.finish")}
            </Button>
          ) : (
            <Button variant="primary" icon={ArrowRight} onClick={() => setStep(stepAfter(step, "next"))}>
              {t("ui.welcome.next")}
            </Button>
          )}
        </footer>
      </div>
    </main>
  );
}
