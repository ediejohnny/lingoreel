import { Eye, EyeOff, Pause, Play, Rewind, FastForward, Square } from "lucide-react";
import { useEffect } from "react";
import { SyncRow } from "../components/player/SyncRow";
import { Timeline } from "../components/player/Timeline";
import { Button, IconButton } from "../components/ui/Button";
import { Banner, Card } from "../components/ui/Feedback";
import { HelpTip } from "../components/ui/HelpTip";
import { Select } from "../components/ui/Select";
import { isTyping, speedLabel } from "../format";
import type { Line, Playback } from "../services/backend";

/** seekStep is the jump of the back and forward buttons, in seconds. */
const seekStep = 5;

interface PlayingPageProps {
  name: string;
  state: Playback;
  speeds: number[];
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
    stop: () => Promise<void>;
  };
}

/** PlayingPage is the playback panel while the video plays in the player. */
export function PlayingPage({ name, state, speeds, error, onDismissError, onCommand, commands }: PlayingPageProps) {
  const playing = state.status === "playing";
  const { capabilities } = state;

  // Space plays or pauses and the arrows jump, unless a field has the focus.
  useEffect(() => {
    const handle = (event: KeyboardEvent) => {
      if (isTyping(event.target) || event.altKey || event.ctrlKey || event.metaKey) {
        return;
      }
      if (event.key === " ") {
        event.preventDefault();
        onCommand(commands.togglePause);
      } else if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        onCommand(() => commands.seekBy(event.key === "ArrowLeft" ? -seekStep : seekStep));
      }
    };
    window.addEventListener("keydown", handle);
    return () => window.removeEventListener("keydown", handle);
  }, [commands, onCommand]);

  const speedOptions = speeds.map((speed) => ({ value: String(speed), label: speedLabel(speed) }));
  const currentSpeed = speeds.find((speed) => Math.abs(speed - state.speed) < 1e-6) ?? 1;

  return (
    <main className="flex flex-1 justify-center overflow-y-auto p-6">
      <div className="flex w-full max-w-3xl flex-col gap-5">
        {error && (
          <Banner tone="error" onClose={onDismissError}>
            {error}
          </Banner>
        )}
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
              label={`Voltar ${seekStep} segundos (←)`}
              disabled={!capabilities.seek}
              onClick={() => onCommand(() => commands.seekBy(-seekStep))}
            />
            <IconButton
              icon={playing ? Pause : Play}
              label={playing ? "Pausar (Espaço)" : "Reproduzir (Espaço)"}
              size="lg"
              active
              disabled={!capabilities.pause}
              onClick={() => onCommand(commands.togglePause)}
            />
            <IconButton
              icon={FastForward}
              label={`Avançar ${seekStep} segundos (→)`}
              disabled={!capabilities.seek}
              onClick={() => onCommand(() => commands.seekBy(seekStep))}
            />
          </div>
        </Card>

        <div className="grid gap-5 sm:grid-cols-2">
          <Card title="Reprodução">
            <div className="flex flex-col gap-4">
              <Select
                label="Velocidade"
                value={String(currentSpeed)}
                options={speedOptions}
                disabled={!capabilities.speed}
                onChange={(value) => onCommand(() => commands.setSpeed(Number(value)))}
              />
              <Button
                icon={state.translationVisible ? EyeOff : Eye}
                onClick={() => onCommand(() => commands.setTranslationVisible(!state.translationVisible))}
              >
                {state.translationVisible ? "Ocultar tradução" : "Mostrar tradução"}
              </Button>
            </div>
          </Card>

          <Card
            title="Sincronização"
            aside={
              <HelpTip topic="sincronização">
                Se uma legenda aparece antes ou depois da fala, ajuste aqui. Com o player em foco, Shift+, e Shift+. movem a legenda de
                cima, Ctrl+, e Ctrl+. a de baixo; segurar a tecla acelera.
              </HelpTip>
            }
          >
            <div className="flex flex-col gap-4">
              <SyncRow
                name={state.lines.primary}
                offset={state.primaryOffset}
                shortcut="Shift+, e Shift+."
                onEarlier={() => onCommand(() => commands.nudge("primary", false))}
                onLater={() => onCommand(() => commands.nudge("primary", true))}
                onReset={() => onCommand(() => commands.resetSync("primary"))}
              />
              <SyncRow
                name={state.lines.translation}
                offset={state.translationOffset}
                shortcut="Ctrl+, e Ctrl+."
                onEarlier={() => onCommand(() => commands.nudge("translation", false))}
                onLater={() => onCommand(() => commands.nudge("translation", true))}
                onReset={() => onCommand(() => commands.resetSync("translation"))}
              />
            </div>
          </Card>
        </div>

        <div className="flex justify-end pb-2">
          <Button icon={Square} onClick={() => onCommand(commands.stop)}>
            Encerrar
          </Button>
        </div>
      </div>
    </main>
  );
}
