import { ArrowLeft, Settings } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { AppHeader } from "./components/AppHeader";
import { IconButton } from "./components/ui/Button";
import { HomePage } from "./pages/HomePage";
import { PlayingPage } from "./pages/PlayingPage";
import { SessionPage } from "./pages/SessionPage";
import { SettingsPage } from "./pages/SettingsPage";
import { backend, messageOf, type Playback, type Players, type Video, type WatchRequest } from "./services/backend";
import { asTheme, useAppliedTheme, type Theme } from "./stores/theme";
import { durationLabel } from "./format";

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
      setScreen((current) => (current === "playing" ? "session" : current));
      if (ended.error) {
        setError(ended.error);
      }
      setResumable(ended.resume);
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

  // The settings go back to where they were opened; a session that ended
  // meanwhile goes back to its screen.
  const closeSettings = () => setScreen(returnTo === "playing" && !state ? "session" : returnTo);
  const back =
    screen === "settings" ? (
      <IconButton icon={ArrowLeft} label="Voltar" onClick={closeSettings} />
    ) : screen === "session" ? (
      <IconButton icon={ArrowLeft} label="Abrir outro vídeo" onClick={() => setScreen("home")} />
    ) : undefined;
  const subtitle =
    screen === "settings" ? "Configurações" : video && screen !== "home" ? `${video.name} · ${durationLabel(video.duration)}` : undefined;
  const openSettings =
    screen === "settings" ? undefined : (
      <IconButton
        icon={Settings}
        label="Configurações"
        onClick={() => {
          setReturnTo(screen);
          setScreen("settings");
        }}
      />
    );

  return (
    <div className="flex h-full flex-col bg-bg">
      <AppHeader theme={theme} onTheme={chooseTheme} leading={back} subtitle={subtitle} actions={openSettings} />
      {screen === "settings" ? (
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
