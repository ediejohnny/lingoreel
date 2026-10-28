import { ArrowLeft, PanelRight, Settings } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AppHeader } from "./components/AppHeader";
import { ComputerChangeBanner } from "./components/ComputerChangeBanner";
import { SidePanel, type PanelTab } from "./components/player/SidePanel";
import { VideoStage } from "./components/player/VideoStage";
import { IconButton } from "./components/ui/Button";
import { HomePage } from "./pages/HomePage";
import { PlayingPage } from "./pages/PlayingPage";
import { SessionPage } from "./pages/SessionPage";
import { SettingsPage, type SettingsHandle } from "./pages/SettingsPage";
import { WelcomePage } from "./pages/WelcomePage";
import { useAppliedTheme } from "./hooks/useAppliedTheme";
import { useDebouncedCall } from "./hooks/useDebouncedCall";
import { useInstall } from "./hooks/useInstall";
import { useVideoKeys } from "./hooks/useVideoKeys";
import {
  backend,
  messageOf,
  type InterfaceSettings,
  type Line,
  type PanelState,
  type Playback,
  type Players,
  type PlayingCommands,
  type Video,
  type WatchRequest,
} from "./services/backend";
import { sectionOfPage, type SectionId } from "./settings";
import { asTheme, type Theme } from "./theme";
import { durationLabel } from "./format";
import { t, setLanguage, useLanguage } from "./i18n";

type Screen = "home" | "session" | "playing" | "settings" | "welcome";

/** waitBeforeKeeping lets a slider move before the app's look is saved. */
const waitBeforeKeeping = 300;

/** App holds which screen is open; each screen keeps its own controls. */
export function App() {
  const [theme, setTheme] = useState<Theme>("system");
  // look is the app's own accent, font and text size over the theme.
  const [look, setLook] = useState<InterfaceSettings>({ accent: "", fontFamily: "", textSize: 1 });
  useAppliedTheme(theme, look);
  const [speeds, setSpeeds] = useState<number[]>([1]);
  const [screen, setScreen] = useState<Screen>("home");
  // returnTo is the screen the settings go back to.
  const [returnTo, setReturnTo] = useState<Screen>("home");
  const [video, setVideo] = useState<Video | null>(null);
  const [players, setPlayers] = useState<Players | null>(null);
  const [state, setState] = useState<Playback | null>(null);
  const [loading, setLoading] = useState(false);
  // opened is how far opening the video went, from 0 to 100, for the bar while it loads.
  const [opened, setOpened] = useState(0);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState("");
  // resumable lets the session start again where it was after the player was lost.
  const [resumable, setResumable] = useState(false);
  // resets counts the times the video was forgotten: the session screen
  // starts over with what the video has become.
  const [resets, setResets] = useState(0);
  // The panel beside a video inside the window, the tab it shows, and the
  // video's fullscreen, which leaves only the video.
  const [panel, setPanel] = useState<PanelState>({ open: true, width: 0 });
  const [panelTab, setPanelTab] = useState<PanelTab>("playback");
  const [settingsSection, setSettingsSection] = useState<SectionId>("general");
  const [fullscreen, setFullscreen] = useState(false);

  // Another language draws every screen again in it; what the app worded
  // (the video, its players and the session) the app sends again.
  useLanguage();

  // welcoming tells the welcome shows; waiting is the video that waits for it,
  // afterWelcome the screen it goes back to.
  const welcoming = useRef(false);
  const waiting = useRef("");
  const afterWelcome = useRef<Screen>("home");

  const open = useCallback(async (path: string) => {
    if (!path) {
      return;
    }
    setLoading(true);
    setOpened(0);
    setError("");
    try {
      const opened = await backend.openVideo(path);
      setVideo(opened);
      setPlayers(opened.players);
      setScreen("session");
    } catch (failure) {
      setError(messageOf(failure));
      // A video refused while another plays leaves the playback panel open.
      setScreen((current) => (current === "playing" ? current : "home"));
    } finally {
      setLoading(false);
    }
  }, []);

  // What the app keeps of the person: the language, the look, the speeds and the panel.
  useEffect(() => {
    backend
      .preferences()
      .then((preferences) => {
        setLanguage(preferences.language);
        setTheme(asTheme(preferences.theme));
        setLook(preferences.interface);
        setSpeeds(preferences.speeds);
        setPanel(preferences.panel);
      })
      .catch(() => undefined);
  }, []);

  // On the first run the welcome comes first; the video the app was opened
  // with waits for it, as one dropped or sent meanwhile does.
  useEffect(() => {
    Promise.all([backend.welcome().catch(() => null), backend.initialVideo().catch(() => "")])
      .then(([welcome, path]) => {
        if (welcome?.pending) {
          welcoming.current = true;
          waiting.current = path;
          setScreen("welcome");
          return;
        }
        return open(path);
      })
      .catch(() => undefined);
    return backend.onDropped((path) => {
      if (welcoming.current) {
        waiting.current = path;
        return;
      }
      void open(path);
    });
  }, [open]);

  // The video: how far opening it went, and the same video described again
  // (a subtitle file added from the menu over it, or another language).
  useEffect(() => {
    const offOpening = backend.onOpening((opening) => setOpened(opening.percent));
    const offVideo = backend.onVideo((next) => {
      setVideo(next);
      setPlayers(next.players);
    });
    return () => {
      offOpening();
      offVideo();
    };
  }, []);

  // The playback: its state, its end and the fullscreen.
  useEffect(() => {
    const offPlayback = backend.onPlayback((next) => setState(next.active ? next : null));
    const offEnded = backend.onEnded((ended) => {
      setState(null);
      setFullscreen(false);
      setScreen((current) => (current === "playing" ? "session" : current));
      if (ended.error) {
        setError(ended.error);
      }
      setResumable(ended.resume);
    });
    const offFullscreen = backend.onFullscreen(setFullscreen);
    return () => {
      offPlayback();
      offEnded();
      offFullscreen();
    };
  }, []);

  // The video's menu opens the panel beside it on the editor or on a page of the settings.
  useEffect(
    () =>
      backend.onPanel((request) => {
        setPanel((current) => (current.open ? current : { ...current, open: true }));
        if (request.section === "settings") {
          setSettingsSection(sectionOfPage(request.page));
          setPanelTab("settings");
          return;
        }
        setPanelTab("playback");
        requestAnimationFrame(() => document.getElementById("panel-editor")?.scrollIntoView({ block: "start" }));
      }),
    [],
  );

  // A player installed, from here or from the welcome, is one the session screen offers.
  const install = useInstall("players", () => {
    backend
      .players()
      .then(setPlayers)
      .catch(() => undefined);
  });

  const chooseTheme = (next: Theme) => {
    setTheme(next);
    // The theme still changes when saving fails; it is only not remembered,
    // and the screen says why.
    backend.setTheme(next).catch((failure: unknown) => setError(messageOf(failure)));
  };

  // A slider sends many sizes: the last one is kept. The look still changes
  // when saving fails; it is only not remembered, and the screen says why.
  const keepLook = useDebouncedCall((next: InterfaceSettings) => {
    backend.setInterface(next).catch((failure: unknown) => setError(messageOf(failure)));
  }, waitBeforeKeeping);
  const chooseLook = (next: InterfaceSettings) => {
    setLook(next);
    keepLook(next);
  };

  const chooseVideo = useCallback(async () => {
    try {
      await open(await backend.chooseVideo());
    } catch (failure) {
      setError(messageOf(failure));
    }
  }, [open]);

  const watch = async (request: WatchRequest) => {
    setStarting(true);
    setError("");
    setResumable(false);
    try {
      await backend.watch(request);
      setState(await backend.playbackState());
      setScreen("playing");
    } catch (failure) {
      setError(messageOf(failure));
    } finally {
      setStarting(false);
    }
  };

  const reset = async () => {
    try {
      const fresh = await backend.resetProject();
      setVideo(fresh);
      setPlayers(fresh.players);
      setResets((count) => count + 1);
      setError("");
    } catch (failure) {
      setError(messageOf(failure));
    }
  };

  const command = useCallback((run: () => Promise<void>) => {
    run().catch((failure) => setError(messageOf(failure)));
  }, []);

  const commands = useMemo<PlayingCommands>(
    () => ({
      ...backend,
      // The video, described again with the file, is what the session screen lists after the session.
      addSubtitleFile: async (line: Line) => {
        const added = await backend.addSubtitleFile(line);
        if (added.path !== "") {
          setVideo(added);
        }
      },
    }),
    [],
  );

  // The welcome opened again from the settings goes back to them.
  const openWelcome = () => {
    afterWelcome.current = "settings";
    welcoming.current = true;
    setScreen("welcome");
  };
  const welcomeDone = () => {
    welcoming.current = false;
    setScreen(afterWelcome.current);
    afterWelcome.current = "home";
    const path = waiting.current;
    waiting.current = "";
    if (path) {
      void open(path);
    }
  };

  const dismissRecommendation = async (id: string) => {
    try {
      await backend.dismissRecommendation(id);
      setPlayers(await backend.players());
    } catch (failure) {
      setError(messageOf(failure));
    }
  };

  // A video drawn inside the window plays under the header, with the panel
  // beside it; fullscreen leaves only the video.
  const embedded = screen === "playing" && video !== null && state !== null && state.embedded;
  // The keys work while a video plays wherever the screen has them, with or
  // without the panel; saved settings may change them.
  const [keysVersion, setKeysVersion] = useState(0);
  const keys = useVideoKeys({
    active: screen === "playing" && state !== null,
    embedded,
    version: keysVersion,
    commands,
    onCommand: command,
  });
  const keepPanel = (next: PanelState) => {
    setPanel(next);
    // The panel still changes when saving fails; it is only not remembered.
    // Dragged often, it stays quiet: a full disk is told by what the user
    // saves on purpose.
    backend.setPanel(next).catch(() => undefined);
  };

  // Closing the settings with choices not saved asks first, on the settings
  // themselves (in the panel, its own tab shows).
  const settingsPage = useRef<SettingsHandle>(null);
  const leaveSettings = async () => {
    const page = settingsPage.current;
    if (!page?.dirty()) {
      return true;
    }
    setPanelTab("settings");
    return page.confirmLeave();
  };

  // The settings go back to where they were opened; a session that ended
  // meanwhile goes back to its screen.
  const closeSettings = async () => {
    if (await leaveSettings()) {
      setScreen(returnTo === "playing" && !state ? "session" : returnTo);
    }
  };
  const stopVideo = async () => {
    if (await leaveSettings()) {
      command(backend.stop);
    }
  };
  const back = embedded ? (
    <IconButton icon={ArrowLeft} label={t("ui.app.stopVideo")} onClick={() => void stopVideo()} />
  ) : screen === "settings" ? (
    <IconButton icon={ArrowLeft} label={t("ui.app.back")} onClick={() => void closeSettings()} />
  ) : screen === "session" ? (
    <IconButton icon={ArrowLeft} label={t("ui.app.openAnother")} onClick={() => setScreen("home")} />
  ) : undefined;
  const subtitle =
    screen === "settings"
      ? t("ui.app.settings")
      : screen === "welcome"
        ? t("ui.welcome.subtitle")
        : video && screen !== "home"
          ? `${video.name} · ${durationLabel(video.duration)}`
          : undefined;
  const openSettings = embedded ? (
    <>
      <IconButton
        icon={Settings}
        label={t("ui.app.settings")}
        onClick={() => {
          setPanelTab("settings");
          keepPanel({ ...panel, open: true });
        }}
      />
      <IconButton
        icon={PanelRight}
        label={panel.open ? t("ui.app.panel.close") : t("ui.app.panel.open")}
        active={panel.open}
        onClick={async () => {
          if (panel.open && !(await leaveSettings())) {
            return;
          }
          keepPanel({ ...panel, open: !panel.open });
        }}
      />
    </>
  ) : screen === "settings" || screen === "welcome" ? undefined : (
    <IconButton
      icon={Settings}
      label={t("ui.app.settings")}
      onClick={() => {
        setReturnTo(screen);
        setScreen("settings");
      }}
    />
  );

  return (
    <div className="flex h-full flex-col bg-bg">
      {!(embedded && fullscreen) && (
        <AppHeader theme={theme} onTheme={chooseTheme} leading={back} subtitle={subtitle} actions={openSettings} />
      )}
      {/* A changed computer is asked about outside the welcome and off a video in the window; it waits for them. */}
      {screen !== "welcome" && !embedded && <ComputerChangeBanner />}
      {screen === "welcome" ? (
        <WelcomePage onDone={welcomeDone} />
      ) : embedded && video && state ? (
        <div className="flex min-h-0 flex-1">
          <VideoStage />
          {panel.open && (
            <SidePanel
              width={panel.width}
              hidden={fullscreen}
              tab={panelTab}
              onTab={setPanelTab}
              onWidth={(width) => keepPanel({ open: true, width })}
              playback={
                <PlayingPage
                  name={video.name}
                  embedded
                  state={state}
                  speeds={speeds}
                  modes={video.modes}
                  error={error}
                  onDismissError={() => setError("")}
                  onCommand={command}
                  keys={keys}
                  commands={commands}
                />
              }
              settings={
                <SettingsPage
                  ref={settingsPage}
                  theme={theme}
                  onTheme={chooseTheme}
                  look={look}
                  onLook={chooseLook}
                  section={settingsSection}
                  narrow
                  shown={panelTab === "settings" && !fullscreen}
                  onSaved={() => setKeysVersion((version) => version + 1)}
                />
              }
            />
          )}
        </div>
      ) : screen === "settings" ? (
        <SettingsPage
          ref={settingsPage}
          theme={theme}
          onTheme={chooseTheme}
          look={look}
          onLook={chooseLook}
          onSaved={() => setKeysVersion((version) => version + 1)}
          onWelcome={state ? undefined : openWelcome}
        />
      ) : screen === "playing" && video && state ? (
        <PlayingPage
          name={video.name}
          state={state}
          speeds={speeds}
          modes={video.modes}
          error={error}
          onDismissError={() => setError("")}
          onCommand={command}
          keys={keys}
          commands={commands}
        />
      ) : screen === "session" && !loading && video && players ? (
        <SessionPage
          key={`${video.path}#${resets}`}
          video={video}
          players={players}
          starting={starting}
          error={error}
          resumable={resumable}
          installing={install.installing}
          installError={install.error}
          onWatch={watch}
          onVideo={setVideo}
          onReset={reset}
          onDismissError={() => {
            setError("");
            setResumable(false);
          }}
          onInstall={install.start}
          onDownload={(id) => command(() => backend.openDownloadPage(id))}
          onDismissRecommendation={dismissRecommendation}
        />
      ) : (
        <HomePage loading={loading} opened={opened} error={error} onChoose={chooseVideo} onDismissError={() => setError("")} />
      )}
    </div>
  );
}
