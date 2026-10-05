import { ArrowLeft, PanelRight, Settings } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { AppHeader } from "./components/AppHeader";
import { SidePanel, type PanelTab } from "./components/player/SidePanel";
import { VideoStage } from "./components/player/VideoStage";
import { IconButton } from "./components/ui/Button";
import { HomePage } from "./pages/HomePage";
import { PlayingPage } from "./pages/PlayingPage";
import { SessionPage } from "./pages/SessionPage";
import { SettingsPage } from "./pages/SettingsPage";
import { backend, messageOf, type PanelState, type Playback, type Players, type Video, type WatchRequest } from "./services/backend";
import { sectionOfPage, type SectionId } from "./settings";
import { asTheme, useAppliedTheme, type Theme } from "./stores/theme";
import { durationLabel } from "./format";
import { t } from "./i18n";

type Screen = "home" | "session" | "playing" | "settings";

/** App holds which screen is open; each screen keeps its own controls. */
export function App() {
  const [theme, setTheme] = useState<Theme>("system");
  useAppliedTheme(theme);
  const [speeds, setSpeeds] = useState<number[]>([1]);
  const [screen, setScreen] = useState<Screen>("home");
  // returnTo is the screen the settings go back to.
  const [returnTo, setReturnTo] = useState<Screen>("home");
  const [video, setVideo] = useState<Video | null>(null);
  const [players, setPlayers] = useState<Players | null>(null);
  const [state, setState] = useState<Playback | null>(null);
  const [loading, setLoading] = useState(false);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState("");
  // resumable offers "Assistir de novo" after the player was lost.
  const [resumable, setResumable] = useState(false);
  const [installing, setInstalling] = useState("");
  const [installError, setInstallError] = useState("");
  // resets counts the times the video was forgotten: the session screen
  // starts over with what the video has become.
  const [resets, setResets] = useState(0);
  // The panel beside a video inside the window, the tab it shows, and the
  // video's fullscreen, which leaves only the video.
  const [panel, setPanel] = useState<PanelState>({ open: true, width: 0 });
  const [panelTab, setPanelTab] = useState<PanelTab>("playback");
  const [settingsPage, setSettingsPage] = useState<SectionId>("general");
  const [fullscreen, setFullscreen] = useState(false);

  const open = useCallback(async (path: string) => {
    if (!path) {
      return;
    }
    setLoading(true);
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

  useEffect(() => {
    backend
      .preferences()
      .then((preferences) => {
        setTheme(asTheme(preferences.theme));
        setSpeeds(preferences.speeds);
        setPanel(preferences.panel);
      })
      .catch(() => undefined);
    backend
      .initialVideo()
      .then(open)
      .catch(() => undefined);
    const offDropped = backend.onDropped((path) => void open(path));
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
    const offPanel = backend.onPanel((request) => {
      setPanel((current) => (current.open ? current : { ...current, open: true }));
      if (request.section === "settings") {
        setSettingsPage(sectionOfPage(request.page));
        setPanelTab("settings");
        return;
      }
      setPanelTab("playback");
      requestAnimationFrame(() => document.getElementById("panel-editor")?.scrollIntoView({ block: "start" }));
    });
    const offInstall = backend.onInstall(async (progress) => {
      setInstalling("");
      setInstallError(progress.error);
      if (!progress.error) {
        setPlayers(await backend.players());
      }
    });
    return () => {
      offDropped();
      offPlayback();
      offEnded();
      offFullscreen();
      offPanel();
      offInstall();
    };
  }, [open]);

  const chooseTheme = (next: Theme) => {
    setTheme(next);
    // The theme still changes when saving fails; it is only not remembered.
    backend.setTheme(next).catch(() => undefined);
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

  const commands = useMemo(
    () => ({
      togglePause: backend.togglePause,
      seekBy: backend.seekBy,
      seekTo: backend.seekTo,
      setSpeed: backend.setSpeed,
      nudge: backend.nudge,
      resetSync: backend.resetSync,
      setTranslationVisible: backend.setTranslationVisible,
      setStudyMode: backend.setStudyMode,
      runShortcut: backend.runShortcut,
      reveal: backend.reveal,
      stop: backend.stop,
      setFullscreen: backend.setFullscreen,
    }),
    [],
  );

  const install = (id: string) => {
    setInstalling(id);
    setInstallError("");
    backend.installPlayer(id).catch((failure) => {
      setInstalling("");
      setInstallError(messageOf(failure));
    });
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
  const keepPanel = (next: PanelState) => {
    setPanel(next);
    // The panel still changes when saving fails; it is only not remembered.
    backend.setPanel(next).catch(() => undefined);
  };

  // The settings go back to where they were opened; a session that ended
  // meanwhile goes back to its screen.
  const closeSettings = () => setScreen(returnTo === "playing" && !state ? "session" : returnTo);
  const back = embedded ? (
    <IconButton icon={ArrowLeft} label={t("ui.app.stopVideo")} onClick={() => command(backend.stop)} />
  ) : screen === "settings" ? (
    <IconButton icon={ArrowLeft} label={t("ui.app.back")} onClick={closeSettings} />
  ) : screen === "session" ? (
    <IconButton icon={ArrowLeft} label={t("ui.app.openAnother")} onClick={() => setScreen("home")} />
  ) : undefined;
  const subtitle =
    screen === "settings"
      ? t("ui.app.settings")
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
        onClick={() => keepPanel({ ...panel, open: !panel.open })}
      />
    </>
  ) : screen === "settings" ? undefined : (
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
      {embedded && video && state ? (
        <div className="flex min-h-0 flex-1">
          <VideoStage />
          {panel.open && !fullscreen && (
            <SidePanel
              width={panel.width}
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
                  commands={commands}
                />
              }
              settings={<SettingsPage theme={theme} onTheme={chooseTheme} section={settingsPage} narrow />}
            />
          )}
        </div>
      ) : screen === "settings" ? (
        <SettingsPage theme={theme} onTheme={chooseTheme} />
      ) : screen === "playing" && video && state ? (
        <PlayingPage
          name={video.name}
          state={state}
          speeds={speeds}
          modes={video.modes}
          error={error}
          onDismissError={() => setError("")}
          onCommand={command}
          commands={commands}
        />
      ) : screen === "session" && video && players ? (
        <SessionPage
          key={`${video.path}#${resets}`}
          video={video}
          players={players}
          starting={starting}
          error={error}
          resumable={resumable}
          installing={installing}
          installError={installError}
          onWatch={watch}
          onReset={reset}
          onDismissError={() => {
            setError("");
            setResumable(false);
          }}
          onInstall={install}
          onDownload={(id) => command(() => backend.openDownloadPage(id))}
          onDismissRecommendation={dismissRecommendation}
        />
      ) : (
        <HomePage loading={loading} error={error} onChoose={chooseVideo} onDismissError={() => setError("")} />
      )}
    </div>
  );
}
