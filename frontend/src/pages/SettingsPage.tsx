import { Save, Undo } from "../components/ui/icons";
import { useCallback, useEffect, useImperativeHandle, useRef, useState, type Ref } from "react";
import { AppearanceSection } from "../components/settings/AppearanceSection";
import { AdvancedSection, PlaybackSection } from "../components/settings/OtherSections";
import { GeneralSection } from "../components/settings/GeneralSection";
import { HelpDocs, HelpSection } from "../components/settings/HelpSection";
import { PlayersSection } from "../components/settings/PlayersSection";
import { ShortcutsSection } from "../components/settings/ShortcutsSection";
import { StudySection } from "../components/settings/StudySection";
import { SubtitleLineSection, SubtitlesSection } from "../components/settings/SubtitlesSection";
import type { SectionProps } from "../components/settings/section";
import { Button } from "../components/ui/Button";
import { Dialog } from "../components/ui/Dialog";
import { Banner } from "../components/ui/Feedback";
import { t } from "../i18n";
import { backend, messageOf, type InterfaceSettings, type ProfileSettings, type SettingsView } from "../services/backend";
import { lineLanguage, relabeled, sameSettings, settingsSections, type SectionId, type SectionItem } from "../settings";
import type { Theme } from "../theme";
import { after } from "../timing";

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

/** SettingsHandle lets the screen around the settings ask before closing them. */
export interface SettingsHandle {
  /** dirty tells there are choices not saved. */
  dirty: () => boolean;
  /** confirmLeave asks what to do with the choices not saved: true to close (saved or discarded), false to stay. */
  confirmLeave: () => Promise<boolean>;
}

interface SettingsPageProps {
  theme: Theme;
  onTheme: (theme: Theme) => void;
  /** look is the app's own accent, font and text size; onLook applies and keeps another. */
  look: InterfaceSettings;
  onLook: (look: InterfaceSettings) => void;
  /** section is the page to open first. */
  section?: SectionId;
  /** narrow lays the page out for the panel beside the video: the pages in a list on top instead of a menu beside. */
  narrow?: boolean;
  /** shown tells the page kept in a hidden tab that it shows again: without a draft, it reads the settings anew. */
  shown?: boolean;
  /** onSaved is told once the profile changed: saved, reset or switched. */
  onSaved?: () => void;
  /** onWelcome opens the welcome again; without it (a video plays) the settings do not offer it. */
  onWelcome?: () => void;
  ref?: Ref<SettingsHandle>;
}

/** previewDelay lets a slider move before the video shows the choice. */
const previewDelay = 150;

/** flatten lists the pages and their own pages in menu order. */
function flatten(items: SectionItem[], depth = 0): { item: SectionItem; depth: number }[] {
  return items.flatMap((item) => [{ item, depth }, ...flatten(item.children ?? [], depth + 1)]);
}

/**
 * SettingsPage edits the active profile as a draft: the preview and the video
 * being watched show every change, and nothing is kept until Save is
 * pressed; leaving the draft puts the saved look back on the video. Choices
 * outside the profile (theme, profiles, players) apply at once.
 */
export function SettingsPage({
  theme,
  onTheme,
  look,
  onLook,
  section: first = "general",
  narrow = false,
  shown = true,
  onSaved,
  onWelcome,
  ref,
}: SettingsPageProps) {
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

  // latest is the draft and the settings of the last drawing, for what runs later.
  const latest = useRef({ draft, view });
  useEffect(() => {
    latest.current = { draft, view };
  });

  // Reading the settings again keeps a draft of the same saved profile (the
  // language, the window or the notices changed meanwhile), in the language
  // of the moment; another saved profile, or none read yet, starts a new draft.
  const reload = useCallback(async () => {
    const next = await backend.settings();
    const { draft, view } = latest.current;
    const same =
      draft !== null &&
      view !== null &&
      view.general.activeProfile === next.general.activeProfile &&
      sameSettings(relabeled(view.profile, next.profile), next.profile);
    setView(next);
    setDraft(same ? relabeled(draft, next.profile) : next.profile);
  }, []);

  // Another page starts from its top, not where the last one was scrolled to.
  const scroller = useRef<HTMLElement>(null);
  useEffect(() => {
    scroller.current?.scrollTo({ top: 0 });
  }, [section]);

  const run = useCallback(
    async (action: () => Promise<unknown>, done?: string | (() => string)) => {
      setError("");
      setNotice("");
      try {
        await action();
        await reload();
        onSaved?.();
        if (done) {
          // Worded after the action, which may have changed the language.
          setNotice(typeof done === "function" ? done() : done);
        }
        return true;
      } catch (failure) {
        setError(messageOf(failure));
        return false;
      }
    },
    [reload, onSaved],
  );

  // While the choices differ from the saved ones the video shows them: it is
  // the preview. The calls go one after another, so the last one wins.
  const previewing = useRef(false);
  const queue = useRef<Promise<unknown>>(Promise.resolve());
  const send = useCallback((call: () => Promise<unknown>) => {
    queue.current = queue.current.then(call).catch(() => undefined);
  }, []);
  useEffect(() => {
    if (!view || !draft) {
      return;
    }
    if (sameSettings(draft, view.profile)) {
      if (previewing.current) {
        previewing.current = false;
        send(backend.endPreview);
      }
      return;
    }
    return after(previewDelay, () => {
      previewing.current = true;
      // A choice still being typed is refused until it is whole: the video keeps the last good one.
      send(() => backend.previewLook(draft));
    });
  }, [draft, view, send]);
  // Leaving the page with the draft (the video ended, the app closed) puts the saved look back.
  useEffect(
    () => () => {
      if (previewing.current) {
        backend.endPreview().catch(() => undefined);
      }
    },
    [],
  );

  // The page reads the settings when it opens, and again when shown with
  // nothing unsaved: they may have changed meanwhile.
  useEffect(() => {
    const { draft, view } = latest.current;
    if (draft === null || view === null || (shown && sameSettings(draft, view.profile))) {
      reload().catch((failure) => setError(messageOf(failure)));
    }
  }, [shown, reload]);
  const [asking, setAsking] = useState(false);
  const answer = useRef<((leave: boolean) => void) | null>(null);
  useImperativeHandle(
    ref,
    () => ({
      dirty: () => {
        const { draft, view } = latest.current;
        return draft !== null && view !== null && !sameSettings(draft, view.profile);
      },
      confirmLeave: () =>
        new Promise<boolean>((resolve) => {
          answer.current?.(false);
          answer.current = resolve;
          setAsking(true);
        }),
    }),
    [],
  );
  const settle = (leave: boolean) => {
    setAsking(false);
    answer.current?.(leave);
    answer.current = null;
  };

  if (!view || !draft) {
    return (
      <main className="flex flex-1 items-center justify-center p-6">
        {error ? <Banner tone="error">{error}</Banner> : <p className="text-sm text-fg-muted">{t("ui.settings.loading")}</p>}
      </main>
    );
  }

  const dirty = !sameSettings(draft, view.profile);
  /** save keeps the draft; it tells whether it was kept. */
  const save = async (): Promise<boolean> => {
    setSaving(true);
    setError("");
    setNotice("");
    try {
      await backend.saveProfile(draft);
      previewing.current = false; // saving gave the video the saved look
      await reload();
      onSaved?.();
      setNotice(t("ui.settings.saved"));
      return true;
    } catch (failure) {
      setError(messageOf(failure));
      return false;
    } finally {
      setSaving(false);
    }
  };
  const discard = () => setDraft(view.profile);
  const props: SectionProps = { view, draft, setDraft, dirty, run, open: setSection, welcome: onWelcome };
  const languages = view.options.languages;
  const sections = settingsSections({
    primary: lineLanguage("primary", draft.languages.primary, languages),
    translation: lineLanguage("translation", draft.languages.translation, languages),
  });
  // The subtitle pages hold the preview beside their fields.
  const wide = section === "subtitles" || section === "subtitles-primary" || section === "subtitles-translation";
  // Beside the video the bar sits on top, where it is always seen; on the page of its own, at the foot.
  const saveBar = dirty && (
    <div
      role="status"
      className={`flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2 bg-surface ${narrow ? "border-b border-line px-3 py-2" : "border-t border-line px-6 py-3"}`}
    >
      <span className="min-w-0 flex-1 text-sm text-fg">
        <span aria-hidden className="mr-2 inline-block size-2 rounded-control bg-warning align-middle" />
        {t("ui.settings.unsaved")}
      </span>
      <Button icon={Undo} variant="ghost" onClick={discard}>
        {t("ui.settings.discard")}
      </Button>
      <Button icon={Save} variant="primary" loading={saving} onClick={() => void save()}>
        {t("ui.common.save")}
      </Button>
    </div>
  );
  // A settings file the app could not read stays on top of every page: it
  // tells what becomes of the changes.
  const banners = (
    <>
      {view.notes.map((note) => (
        <Banner key={note} tone="warning">
          {note}
        </Banner>
      ))}
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
    </>
  );

  return (
    <div className={`relative flex min-h-0 flex-1 ${narrow ? "flex-col" : ""}`}>
      {narrow ? (
        <div className="shrink-0 border-b border-line bg-surface p-3">
          <label className="flex flex-col gap-1 text-sm text-fg-secondary">
            {t("ui.settings.section")}
            <select
              value={section}
              onChange={(event) => setSection(event.target.value as SectionId)}
              className="h-9 min-w-0 rounded-control border border-line-strong bg-surface px-2 text-fg"
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
        <nav aria-label={t("ui.settings.nav")} className="w-52 shrink-0 overflow-y-auto border-r border-line bg-surface p-3">
          <ul className="flex flex-col gap-0.5">
            {sections.map((item) => (
              <MenuEntry key={item.id} item={item} current={section} onOpen={setSection} />
            ))}
          </ul>
        </nav>
      )}
      {narrow && saveBar}
      {narrow && (error || notice || view.notes.length > 0) && <div className="flex shrink-0 flex-col gap-2 p-3 pb-0">{banners}</div>}
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        {/* The padding is inside the scrolling area, so whatever sticks to its top sticks to the very top. */}
        <main ref={scroller} className="flex-1 overflow-x-hidden overflow-y-auto">
          <div className={`mx-auto flex min-w-0 flex-col gap-5 ${narrow ? "p-4" : "p-6"} ${wide ? "max-w-6xl" : "max-w-4xl"}`}>
            {!narrow && banners}
            {section === "general" && <GeneralSection {...props} />}
            {section === "playback" && <PlaybackSection {...props} />}
            {section === "subtitles" && <SubtitlesSection {...props} />}
            {section === "subtitles-primary" && <SubtitleLineSection {...props} line="primary" />}
            {section === "subtitles-translation" && <SubtitleLineSection {...props} line="translation" />}
            {section === "study" && <StudySection {...props} />}
            {section === "shortcuts" && <ShortcutsSection {...props} />}
            {section === "players" && <PlayersSection {...props} />}
            {section === "appearance" && (
              <AppearanceSection view={view} draft={draft} run={props.run} theme={theme} onTheme={onTheme} look={look} onLook={onLook} />
            )}
            {section === "advanced" && <AdvancedSection {...props} />}
            {(section === "help" || section === "help-docs") && <HelpDocs keys={draft.shortcuts} narrow={narrow} onOpen={setSection} />}
            {section === "help-about" && <HelpSection />}
          </div>
        </main>
        {!narrow && saveBar}
      </div>
      {asking && (
        <Dialog title={t("ui.settings.leave.question")} alert within onCancel={() => settle(false)}>
          <div className="flex flex-wrap justify-end gap-2">
            <Button variant="ghost" onClick={() => settle(false)}>
              {t("ui.settings.leave.keepEditing")}
            </Button>
            <Button
              icon={Undo}
              onClick={() => {
                discard();
                settle(true);
              }}
            >
              {t("ui.settings.discard")}
            </Button>
            <Button icon={Save} variant="primary" autoFocus loading={saving} onClick={() => void save().then(settle)}>
              {t("ui.common.save")}
            </Button>
          </div>
        </Dialog>
      )}
    </div>
  );
}
