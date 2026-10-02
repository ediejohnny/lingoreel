import { Save, Undo2 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { AppearanceSection, AdvancedSection, PlaybackSection } from "../components/settings/OtherSections";
import { GeneralSection } from "../components/settings/GeneralSection";
import { PlayersSection } from "../components/settings/PlayersSection";
import { ShortcutsSection } from "../components/settings/ShortcutsSection";
import { StudySection } from "../components/settings/StudySection";
import { SubtitlesSection } from "../components/settings/SubtitlesSection";
import type { SectionProps } from "../components/settings/section";
import { Button } from "../components/ui/Button";
import { Banner } from "../components/ui/Feedback";
import { backend, messageOf, type ProfileSettings, type SettingsView } from "../services/backend";
import { sameSettings } from "../settings";
import type { Theme } from "../stores/theme";

/** The sections, grouped by subject (DESIGN §32). */
const sections = [
  { id: "general", label: "Geral" },
  { id: "playback", label: "Reprodução" },
  { id: "subtitles", label: "Legendas" },
  { id: "study", label: "Estudo" },
  { id: "shortcuts", label: "Atalhos" },
  { id: "players", label: "Player" },
  { id: "appearance", label: "Aparência" },
  { id: "advanced", label: "Avançado" },
] as const;

type SectionId = (typeof sections)[number]["id"];

interface SettingsPageProps {
  theme: Theme;
  onTheme: (theme: Theme) => void;
}

/**
 * SettingsPage edits the active profile as a draft: the preview follows
 * every change, and nothing reaches the profile or the video until "Salvar"
 * (DESIGN §34). Choices outside the profile (theme, profiles, players) apply
 * at once.
 */
export function SettingsPage({ theme, onTheme }: SettingsPageProps) {
  const [view, setView] = useState<SettingsView | null>(null);
  const [draft, setDraft] = useState<ProfileSettings | null>(null);
  const [section, setSection] = useState<SectionId>("general");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [saving, setSaving] = useState(false);

  const reload = useCallback(async () => {
    const next = await backend.settings();
    setView(next);
    setDraft(next.profile);
  }, []);

  useEffect(() => {
    let current = true;
    backend
      .settings()
      .then((next) => {
        if (current) {
          setView(next);
          setDraft(next.profile);
        }
      })
      .catch((failure) => current && setError(messageOf(failure)));
    return () => {
      current = false;
    };
  }, []);

  const run = useCallback(
    async (action: () => Promise<unknown>, done?: string) => {
      setError("");
      setNotice("");
      try {
        await action();
        await reload();
        if (done) {
          setNotice(done);
        }
      } catch (failure) {
        setError(messageOf(failure));
      }
    },
    [reload],
  );

  if (!view || !draft) {
    return (
      <main className="flex flex-1 items-center justify-center p-6">
        {error ? <Banner tone="error">{error}</Banner> : <p className="text-sm text-fg-muted">Carregando as configurações…</p>}
      </main>
    );
  }

  const dirty = !sameSettings(draft, view.profile);
  const save = async () => {
    setSaving(true);
    await run(() => backend.saveProfile(draft), "Configurações salvas. O vídeo aberto já usa as novas escolhas.");
    setSaving(false);
  };
  const props: SectionProps = { view, draft, setDraft, dirty, run };

  return (
    <div className="flex min-h-0 flex-1">
      <nav aria-label="Seções das configurações" className="w-48 shrink-0 border-r border-line bg-surface p-3">
        <ul className="flex flex-col gap-0.5">
          {sections.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                aria-current={item.id === section ? "page" : undefined}
                onClick={() => setSection(item.id)}
                className={`w-full rounded-control px-3 py-2 text-left text-sm transition-colors duration-[var(--duration-fast)] ${
                  item.id === section ? "bg-accent-soft font-medium text-accent" : "text-fg-secondary hover:bg-sunken hover:text-fg"
                }`}
              >
                {item.label}
              </button>
            </li>
          ))}
        </ul>
      </nav>
      <div className="flex min-w-0 flex-1 flex-col">
        <main className="flex-1 overflow-y-auto p-6">
          <div className="mx-auto flex max-w-4xl flex-col gap-5">
            {error && (
              <Banner tone="error" onClose={() => setError("")}>
                {error}
              </Banner>
            )}
            {notice && !error && (
              <Banner tone="info" onClose={() => setNotice("")}>
                {notice}
              </Banner>
            )}
            {section === "general" && <GeneralSection {...props} />}
            {section === "playback" && <PlaybackSection {...props} />}
            {section === "subtitles" && <SubtitlesSection {...props} />}
            {section === "study" && <StudySection {...props} />}
            {section === "shortcuts" && <ShortcutsSection {...props} />}
            {section === "players" && <PlayersSection {...props} />}
            {section === "appearance" && <AppearanceSection theme={theme} onTheme={onTheme} />}
            {section === "advanced" && <AdvancedSection {...props} />}
          </div>
        </main>
        {dirty && (
          <footer className="flex shrink-0 items-center gap-3 border-t border-line bg-surface px-6 py-3" role="status">
            <span className="flex-1 text-sm text-fg">
              <span aria-hidden className="mr-2 inline-block size-2 rounded-full bg-warning align-middle" />
              Alterações não salvas
            </span>
            <Button icon={Undo2} variant="ghost" onClick={() => setDraft(view.profile)}>
              Descartar
            </Button>
            <Button icon={Save} variant="primary" loading={saving} onClick={() => void save()}>
              Salvar
            </Button>
          </footer>
        )}
      </div>
    </div>
  );
}
