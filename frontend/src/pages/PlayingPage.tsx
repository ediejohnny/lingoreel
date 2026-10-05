import { Eye, EyeOff, Pause, Play, Rewind, FastForward, Square } from "lucide-react";
import { useEffect, useState } from "react";
import { EditorCard } from "../components/player/EditorCard";
import { SyncRow } from "../components/player/SyncRow";
import { Timeline } from "../components/player/Timeline";
import { TrainingCards } from "../components/player/TrainingCards";
import { Button, IconButton } from "../components/ui/Button";
import { Banner, Card } from "../components/ui/Feedback";
import { HelpTip } from "../components/ui/HelpTip";
import { Select } from "../components/ui/Select";
import { isTyping, speedLabel } from "../format";
import { fullscreenOf, keyLabel, panelCommand, translationCommand, withKey } from "../keys";
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
  };
}

/** both joins two keys for a hint: "Shift+, e Shift+.", or the one there is. */
function both(first: string, second: string): string {
  return [first, second].filter((label) => label !== "").join(" e ");
}

function modeHelp(translationKey: string): string {
  return (
    `Dupla mostra as duas legendas, e a tecla da tradução (${translationKey || "sem atalho"}) esconde e mostra a de baixo. ` +
    "Tradução sob demanda começa só com a legenda de cima: a tecla mostra a tradução quando você precisar. " +
    "Tradução sempre visível nunca esconde a de baixo. Vale só para este vídeo; o padrão fica nas Configurações."
  );
}

function syncHelp(primaryKeys: string, translationKeys: string): string {
  const primary = primaryKeys ? `${primaryKeys} movem a legenda de cima` : "a legenda de cima está sem atalho";
  const translation = translationKeys ? `${translationKeys} a de baixo` : "a de baixo está sem atalho";
  return (
    "Se uma legenda aparece antes ou depois da fala, ajuste aqui ou pelos atalhos, na janela do filme ou neste painel: " +
    `${primary}, ${translation}. Segurar a tecla acelera.`
  );
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
  commands,
}: PlayingPageProps) {
  const playing = state.status === "playing";
  const { capabilities } = state;
  const [keys, setKeys] = useState<ShortcutSetting[]>([]);
  const label = (command: string) => keyLabel(keys, command);

  useEffect(() => {
    let current = true;
    backend
      .shortcuts()
      .then((list) => current && setKeys(list))
      .catch(() => current && setKeys([]));
    return () => {
      current = false;
    };
  }, []);

  // The profile's shortcuts work here too, as in the player window, unless
  // a field has the focus. The translation key also hears its release.
  useEffect(() => {
    let revealCode = "";
    const release = () => {
      if (revealCode !== "") {
        revealCode = "";
        onCommand(() => commands.reveal(false));
      }
    };
    const down = (event: KeyboardEvent) => {
      if (isTyping(event.target)) {
        return;
      }
      const command = panelCommand(event, keys, embedded);
      if (command === "") {
        return;
      }
      event.preventDefault();
      const fullscreen = fullscreenOf(command);
      if (fullscreen !== undefined) {
        onCommand(() => commands.setFullscreen(fullscreen));
        return;
      }
      if (command === translationCommand) {
        if (!event.repeat) {
          revealCode = event.code;
          onCommand(() => commands.reveal(true));
        }
        return;
      }
      onCommand(() => commands.runShortcut(command, event.repeat));
    };
    const up = (event: KeyboardEvent) => {
      if (revealCode !== "" && event.code === revealCode) {
        // Also keeps the right Alt from opening the window menu.
        event.preventDefault();
        release();
      }
    };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    window.addEventListener("blur", release);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
      window.removeEventListener("blur", release);
      release();
    };
  }, [keys, commands, onCommand, embedded]);

  const speedOptions = speeds.map((speed) => ({ value: String(speed), label: speedLabel(speed) }));
  const currentSpeed = speeds.find((speed) => Math.abs(speed - state.speed) < 1e-6) ?? 1;
  const translationLocked = state.mode === alwaysMode;
  const primaryKeys = both(label("sync-primary-earlier"), label("sync-primary-later"));
  const translationKeys = both(label("sync-translation-earlier"), label("sync-translation-later"));

  return (
    <main className={`flex flex-1 justify-center overflow-y-auto ${embedded ? "p-4" : "p-6"}`}>
      <div className="flex w-full max-w-3xl flex-col gap-5">
        {error && (
          <Banner tone="error" onClose={onDismissError}>
            {error}
          </Banner>
        )}
        {state.health && <Banner tone="warning">{state.health}</Banner>}
        {state.projectNote && <Banner tone="warning">{state.projectNote}</Banner>}
        {!embedded && (
          <Card>
            <p className="mb-1 truncate text-sm text-fg-muted">Tocando no player</p>
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
                label={withKey(`Voltar ${seekStep} segundos`, label("seek-back"))}
                disabled={!capabilities.seek}
                onClick={() => onCommand(() => commands.seekBy(-seekStep))}
              />
              <IconButton
                icon={playing ? Pause : Play}
                label={withKey(playing ? "Pausar" : "Reproduzir", label("play-pause"))}
                size="lg"
                active
                disabled={!capabilities.pause}
                onClick={() => onCommand(commands.togglePause)}
              />
              <IconButton
                icon={FastForward}
                label={withKey(`Avançar ${seekStep} segundos`, label("seek-forward"))}
                disabled={!capabilities.seek}
                onClick={() => onCommand(() => commands.seekBy(seekStep))}
              />
            </div>
          </Card>
        )}

        <div className={`grid gap-5 ${embedded ? "" : "sm:grid-cols-2"}`}>
          <Card title="Reprodução">
            <div className="flex flex-col gap-4">
              <Select
                label="Velocidade"
                value={String(currentSpeed)}
                options={speedOptions}
                disabled={!capabilities.speed}
                onChange={(value) => onCommand(() => commands.setSpeed(Number(value)))}
              />
              {modes.length > 0 && (
                <Select
                  label="Modo de estudo"
                  value={state.mode}
                  options={modes.map((mode) => ({ value: mode.value, label: mode.label }))}
                  onChange={(mode) => onCommand(() => commands.setStudyMode(mode))}
                  help={<HelpTip topic="modo de estudo">{modeHelp(label(translationCommand))}</HelpTip>}
                />
              )}
              <Button
                icon={state.translationVisible ? EyeOff : Eye}
                disabled={translationLocked}
                title={translationLocked ? "A tradução fica sempre visível neste modo de estudo" : undefined}
                onClick={() => onCommand(() => commands.setTranslationVisible(!state.translationVisible))}
              >
                {state.translationVisible ? "Ocultar tradução" : "Mostrar tradução"}
              </Button>
            </div>
          </Card>

          <Card title="Sincronização" aside={<HelpTip topic="sincronização">{syncHelp(primaryKeys, translationKeys)}</HelpTip>}>
            <div className="flex flex-col gap-4">
              <SyncRow
                name={state.lines.primary}
                offset={state.primaryOffset}
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

        <div id="panel-editor">
          <EditorCard state={state} label={label} onCommand={onCommand} />
        </div>

        <TrainingCards state={state} label={label} onCommand={onCommand} />

        <div className="flex justify-end pb-2">
          <Button icon={Square} onClick={() => onCommand(commands.stop)}>
            Encerrar
          </Button>
        </div>
      </div>
    </main>
  );
}
