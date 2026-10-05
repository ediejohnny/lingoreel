import { Save, Undo2 } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { AppearanceSection, AdvancedSection, PlaybackSection } from "../components/settings/OtherSections";
import { GeneralSection } from "../components/settings/GeneralSection";
import { PlayersSection } from "../components/settings/PlayersSection";
import { ShortcutsSection } from "../components/settings/ShortcutsSection";
import { StudySection } from "../components/settings/StudySection";
import { SubtitleLineSection, SubtitlesSection } from "../components/settings/SubtitlesSection";
import type { SectionProps } from "../components/settings/section";
import { Button } from "../components/ui/Button";
import { Banner } from "../components/ui/Feedback";
import { backend, messageOf, type ProfileSettings, type SettingsView } from "../services/backend";
import { lineLanguage, sameSettings, settingsSections, type SectionId, type SectionItem } from "../settings";
import type { Theme } from "../stores/theme";

interface MenuEntryProps {
  item: SectionItem;
  current: SectionId;
  onOpen: (section: SectionId) => void;
}

/** MenuEntry is one page of the settings menu, with its own pages below it. */
function MenuEntry({ item, current, onOpen }: MenuEntryProps) {
  const active = item.id === current;
  return (
    <li>
      <button
        type="button"
        aria-current={active ? "page" : undefined}
        onClick={() => onOpen(item.id)}
        className={`flex w-full flex-col rounded-control px-3 py-2 text-left text-sm transition-colors duration-[var(--duration-fast)] ${
          active ? "bg-accent-soft font-medium text-accent" : "text-fg-secondary hover:bg-sunken hover:text-fg"
        }`}
      >
        {item.label}
        {item.detail && <span className={`text-xs font-normal ${active ? "text-accent" : "text-fg-muted"}`}>{item.detail}</span>}
      </button>
      {item.children && (
        <ul className="mt-0.5 ml-3 flex flex-col gap-0.5 border-l border-line pl-2">
          {item.children.map((child) => (
            <MenuEntry key={child.id} item={child} current={current} onOpen={onOpen} />
          ))}
        </ul>
      )}
    </li>
  );
}

interface SettingsPageProps {
  theme: Theme;
  onTheme: (theme: Theme) => void;
  /** section is the page to open first. */
  section?: SectionId;
  /** narrow lays the page out for the panel beside the video: the pages in a list on top instead of a menu beside. */
  narrow?: boolean;
}

/** flatten lists the pages and their own pages in menu order. */
function flatten(items: SectionItem[], depth = 0): { item: SectionItem; depth: number }[] {
  return items.flatMap((item) => [{ item, depth }, ...flatten(item.children ?? [], depth + 1)]);
}

/**
 * SettingsPage edits the active profile as a draft: the preview follows
 * every change, and nothing reaches the profile or the video until "Salvar"
 * is pressed. Choices outside the profile (theme, profiles, players) apply
 * at once.
 */
export function SettingsPage({ theme, onTheme, section: first = "general", narrow = false }: SettingsPageProps) {
  const [view, setView] = useState<SettingsView | null>(null);
  const [draft, setDraft] = useState<ProfileSettings | null>(null);
  const [section, setSection] = useState<SectionId>(first);
  // A request for another page (from the video's menu) opens it.
  const [asked, setAsked] = useState<SectionId>(first);
  if (asked !== first) {
    setAsked(first);
    setSection(first);
  }
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

  // Another page starts from its top, not where the last one was scrolled to.
  const scroller = useRef<HTMLElement>(null);
  useEffect(() => {
    scroller.current?.scrollTo({ top: 0 });
  }, [section]);

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
  const props: SectionProps = { view, draft, setDraft, dirty, run, open: setSection };
  const languages = view.options.languages;
  const sections = settingsSections({
    primary: lineLanguage("primary", draft.languages.primary, languages),
    translation: lineLanguage("translation", draft.languages.translation, languages),
  });
  // The subtitle pages hold the preview beside their fields.
  const wide = section === "subtitles" || section === "subtitles-primary" || section === "subtitles-translation";

  return (
    <div className={`flex min-h-0 flex-1 ${narrow ? "flex-col" : ""}`}>
      {narrow ? (
        <div className="shrink-0 border-b border-line bg-surface p-3">
          <label className="flex flex-col gap-1 text-sm text-fg-secondary">
            Seção
            <select
              value={section}
              onChange={(event) => setSection(event.target.value as SectionId)}
              className="h-9 rounded-control border border-line-strong bg-surface px-2 text-fg"
            >
              {flatten(sections).map(({ item, depth }) => (
                <option key={item.id} value={item.id}>
                  {"  ".repeat(depth) + item.label}
                </option>
              ))}
            </select>
          </label>
        </div>
      ) : (
        <nav aria-label="Seções das configurações" className="w-52 shrink-0 overflow-y-auto border-r border-line bg-surface p-3">
          <ul className="flex flex-col gap-0.5">
            {sections.map((item) => (
              <MenuEntry key={item.id} item={item} current={section} onOpen={setSection} />
            ))}
          </ul>
        </nav>
      )}
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        {/* The padding is inside the scrolling area, so whatever sticks to its top sticks to the very top. */}
        <main ref={scroller} className="flex-1 overflow-y-auto">
          <div className={`mx-auto flex flex-col gap-5 ${narrow ? "p-4" : "p-6"} ${wide ? "max-w-6xl" : "max-w-4xl"}`}>
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
            {section === "subtitles-primary" && <SubtitleLineSection {...props} line="primary" />}
            {section === "subtitles-translation" && <SubtitleLineSection {...props} line="translation" />}
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
