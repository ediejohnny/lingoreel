import { Eye, EyeOff, Pause, Play, Rewind, FastForward, Square } from "lucide-react";
import { useEffect, useState } from "react";
import { EditorCard } from "../components/player/EditorCard";
import { SubtitlesCard } from "../components/player/SubtitlesCard";
import { SyncRow } from "../components/player/SyncRow";
import { Timeline } from "../components/player/Timeline";
import { TrainingCards } from "../components/player/TrainingCards";
import { Button, IconButton } from "../components/ui/Button";
import { Banner, Card } from "../components/ui/Feedback";
import { HelpTip } from "../components/ui/HelpTip";
import { Select } from "../components/ui/Select";
import { speedLabel } from "../format";
import { t } from "../i18n";
import { both, keyLabel, translationCommand, withKey } from "../keys";
import { backend, type Choice, type Line, type Playback, type ShortcutSetting } from "../services/backend";

/** seekStep is the jump of the back and forward buttons, in seconds. */
const seekStep = 5;

/** alwaysMode is the study mode where the translation never hides. */
const alwaysMode = "always";

interface PlayingPageProps {
  name: string;
  /** embedded is the panel beside the video inside the app's window: the bar over the video has the timeline and the play buttons. */
  embedded?: boolean;
  state: Playback;
  speeds: number[];
  modes: Choice[];
  error: string;
  onDismissError: () => void;
  onCommand: (command: () => Promise<void>) => void;
  /** keysVersion changes when the profile's keys may have changed, so their labels are read again. */
  keysVersion?: number;
  commands: {
    togglePause: () => Promise<void>;
    seekBy: (seconds: number) => Promise<void>;
    seekTo: (seconds: number) => Promise<void>;
    setSpeed: (speed: number) => Promise<void>;
    nudge: (line: Line, later: boolean) => Promise<void>;
    resetSync: (line: Line) => Promise<void>;
    setTranslationVisible: (visible: boolean) => Promise<void>;
    setStudyMode: (mode: string) => Promise<void>;
    runShortcut: (command: string, repeat: boolean) => Promise<void>;
    reveal: (pressed: boolean) => Promise<void>;
    stop: () => Promise<void>;
    setFullscreen: (on: boolean) => Promise<void>;
    switchSubtitle: (line: Line, track: number) => Promise<void>;
    switchAudio: (track: number) => Promise<void>;
    addSubtitleFile: (line: Line) => Promise<void>;
  };
}

function modeHelp(translationKey: string): string {
  return t("ui.playing.mode.help", { key: translationKey || t("ui.playing.mode.noKey") });
}

function syncHelp(primaryKeys: string, translationKeys: string): string {
  const primary = primaryKeys ? t("ui.playing.sync.helpPrimary", { keys: primaryKeys }) : t("ui.playing.sync.helpPrimaryNone");
  const translation = translationKeys
    ? t("ui.playing.sync.helpTranslation", { keys: translationKeys })
    : t("ui.playing.sync.helpTranslationNone");
  return t("ui.playing.sync.help", { primary, translation });
}

/** PlayingPage is the playback panel while the video plays in the player. */
export function PlayingPage({
  name,
  embedded = false,
  state,
  speeds,
  modes,
  error,
  onDismissError,
  onCommand,
  keysVersion = 0,
  commands,
}: PlayingPageProps) {
  const playing = state.status === "playing";
  const { capabilities } = state;
  const [keys, setKeys] = useState<ShortcutSetting[]>([]);
  const label = (command: string) => keyLabel(keys, command);

  // The keys' labels for the buttons and the help; the keys themselves work
  // from the app (useVideoKeys), with or without this panel.
  useEffect(() => {
    let current = true;
    backend
      .shortcuts()
      .then((list) => current && setKeys(list))
      .catch(() => current && setKeys([]));
    return () => {
      current = false;
    };
  }, [keysVersion]);

  const speedOptions = speeds.map((speed) => ({ value: String(speed), label: speedLabel(speed) }));
  const currentSpeed = speeds.find((speed) => Math.abs(speed - state.speed) < 1e-6) ?? 1;
  const translationLocked = state.mode === alwaysMode;
  const primaryKeys = both(label("sync-primary-earlier"), label("sync-primary-later"));
  const translationKeys = both(label("sync-translation-earlier"), label("sync-translation-later"));

  return (
    <main className={`flex min-w-0 flex-1 justify-center overflow-y-auto ${embedded ? "overflow-x-hidden p-4" : "p-6"}`}>
      <div className="flex w-full max-w-3xl min-w-0 flex-col gap-5">
        {error && (
          <Banner tone="error" onClose={onDismissError}>
            {error}
          </Banner>
        )}
        {state.health && <Banner tone="warning">{state.health}</Banner>}
        {state.projectNote && <Banner tone="warning">{state.projectNote}</Banner>}
        {!embedded && (
          <Card>
            <p className="mb-1 truncate text-sm text-fg-muted">{t("ui.playing.title")}</p>
            <h1 className="mb-5 truncate text-lg font-semibold text-fg" title={name}>
              {name}
            </h1>
            <Timeline
              position={state.position}
              duration={state.duration}
              positionText={state.positionText}
              durationText={state.durationText}
              disabled={!capabilities.seek}
              onSeek={(seconds) => onCommand(() => commands.seekTo(seconds))}
            />
            <div className="mt-5 flex flex-wrap items-center justify-center gap-2">
              <IconButton
                icon={Rewind}
                label={withKey(t("ui.playing.seekBack", { seconds: seekStep }), label("seek-back"))}
                disabled={!capabilities.seek}
                onClick={() => onCommand(() => commands.seekBy(-seekStep))}
              />
              <IconButton
                icon={playing ? Pause : Play}
                label={withKey(playing ? t("ui.playing.pause") : t("ui.playing.play"), label("play-pause"))}
                size="lg"
                active
                disabled={!capabilities.pause}
                onClick={() => onCommand(commands.togglePause)}
              />
              <IconButton
                icon={FastForward}
                label={withKey(t("ui.playing.seekForward", { seconds: seekStep }), label("seek-forward"))}
                disabled={!capabilities.seek}
                onClick={() => onCommand(() => commands.seekBy(seekStep))}
              />
            </div>
          </Card>
        )}

        <div className={`grid gap-5 [&>*]:min-w-0 ${embedded ? "" : "sm:grid-cols-2"}`}>
          <Card title={t("ui.playing.playback.title")}>
            <div className="flex flex-col gap-4">
              <Select
                label={t("ui.playing.speed.label")}
                value={String(currentSpeed)}
                options={speedOptions}
                disabled={!capabilities.speed}
                onChange={(value) => onCommand(() => commands.setSpeed(Number(value)))}
              />
              {modes.length > 0 && (
                <Select
                  label={t("ui.playing.mode.label")}
                  value={state.mode}
                  options={modes.map((mode) => ({ value: mode.value, label: mode.label }))}
                  onChange={(mode) => onCommand(() => commands.setStudyMode(mode))}
                  help={<HelpTip topic={t("ui.playing.mode.topic")}>{modeHelp(label(translationCommand))}</HelpTip>}
                />
              )}
              <Button
                icon={state.translationVisible ? EyeOff : Eye}
                disabled={translationLocked}
                title={translationLocked ? t("ui.playing.translation.locked") : undefined}
                onClick={() => onCommand(() => commands.setTranslationVisible(!state.translationVisible))}
              >
                {state.translationVisible ? t("ui.playing.translation.hide") : t("ui.playing.translation.show")}
              </Button>
            </div>
          </Card>

          <Card
            title={t("ui.playing.sync.title")}
            aside={<HelpTip topic={t("ui.playing.sync.topic")}>{syncHelp(primaryKeys, translationKeys)}</HelpTip>}
          >
            <div className="flex flex-col gap-4">
              <SyncRow
                name={state.lines.primary}
                offset={state.primaryOffset}
                voice={state.primaryVoice}
                shortcut={primaryKeys}
                onEarlier={() => onCommand(() => commands.nudge("primary", false))}
                onLater={() => onCommand(() => commands.nudge("primary", true))}
                onReset={() => onCommand(() => commands.resetSync("primary"))}
              />
              <SyncRow
                name={state.lines.translation}
                offset={state.translationOffset}
                shortcut={translationKeys}
                onEarlier={() => onCommand(() => commands.nudge("translation", false))}
                onLater={() => onCommand(() => commands.nudge("translation", true))}
                onReset={() => onCommand(() => commands.resetSync("translation"))}
              />
            </div>
          </Card>
        </div>

        <SubtitlesCard state={state} onCommand={onCommand} commands={commands} />

        <div id="panel-editor">
          <EditorCard state={state} label={label} onCommand={onCommand} />
        </div>

        <TrainingCards state={state} label={label} onCommand={onCommand} />

        <div className="flex justify-end pb-2">
          <Button icon={Square} onClick={() => onCommand(commands.stop)}>
            {t("ui.playing.stop")}
          </Button>
        </div>
      </div>
    </main>
  );
}
