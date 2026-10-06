import { BookOpenCheck, Bookmark, Flag, LogOut, RotateCcw, Undo2 } from "lucide-react";
import { useState } from "react";
import { passageClock } from "../../format";
import { t } from "../../i18n";
import { withKey } from "../../keys";
import { backend } from "../../services/backend";
import type { Playback } from "../../services/backend";
import { Button } from "../ui/Button";
import { Toggle, TextField } from "../ui/Field";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import { Rich } from "../ui/Rich";
import { Select } from "../ui/Select";
import { SavedPassages } from "./SavedPassages";

interface TrainingCardsProps {
  state: Playback;
  /** label is the key of a command, such as "Ctrl+F9" for "passage-mark-start". */
  label: (command: string) => string;
  onCommand: (command: () => Promise<void>) => void;
}

const repeatOptions = () =>
  Array.from({ length: 11 }, (_, times) => ({
    value: String(times),
    label: times === 0 ? t("ui.karaoke.repeat.off") : t("ui.karaoke.repeat.times", { count: times }),
  }));

const gapOptions = () =>
  [0, 1, 2, 3, 5].map((seconds) => ({
    value: String(seconds),
    label: seconds === 0 ? t("ui.passage.gap.none") : t("ui.passage.gap.seconds", { seconds }),
  }));

const goalOptions = () =>
  [0, 3, 5, 10, 20].map((laps) => ({
    value: String(laps),
    label: laps === 0 ? t("ui.passage.goal.none") : t("ui.passage.goal.laps", { laps }),
  }));

/** TrainingCards are the Modo Karaokê and "Estudar trecho" controls. */
export function TrainingCards({ state, label, onCommand }: TrainingCardsProps) {
  return (
    <div className="@container">
      <div className="grid gap-5 [&>*]:min-w-0 @2xl:grid-cols-2">
        <KaraokeCard state={state} label={label} onCommand={onCommand} />
        <PassageCard state={state} label={label} onCommand={onCommand} />
      </div>
    </div>
  );
}

/** voiceStates word how the line's voice is followed now. */
const voiceStates: Record<string, () => string> = {
  ahead: () => t("ui.karaoke.voice.ahead"),
  live: () => t("ui.karaoke.voice.live"),
  estimate: () => t("ui.karaoke.voice.estimate"),
};

function KaraokeCard({ state, onCommand }: TrainingCardsProps) {
  const { karaoke } = state;
  const voice = voiceStates[karaoke.voice];
  return (
    <Card title={t("ui.karaoke.title")} aside={<HelpTip topic={t("ui.karaoke.topic")}>{t("ui.karaoke.help")}</HelpTip>}>
      <div className="flex flex-col gap-4">
        <Toggle label={t("ui.karaoke.enable")} checked={karaoke.enabled} onChange={(on) => onCommand(() => backend.setKaraoke(on))} />
        {karaoke.enabled && voice && <p className="text-xs text-fg-muted">{voice()}</p>}
        <Select
          label={t("ui.karaoke.indicator.label")}
          value={karaoke.indicator}
          options={[
            { value: "bar", label: t("ui.karaoke.indicator.bar") },
            { value: "countdown", label: t("ui.karaoke.indicator.countdown") },
          ]}
          onChange={(value) => onCommand(() => backend.setKaraokeIndicator(value))}
          help={<HelpTip topic={t("ui.karaoke.indicator.topic")}>{t("ui.karaoke.indicator.help")}</HelpTip>}
        />
        <Select
          label={t("ui.karaoke.repeat.label")}
          value={String(karaoke.repeat)}
          options={repeatOptions()}
          onChange={(value) => onCommand(() => backend.setLineRepeats(Number(value)))}
          help={<HelpTip topic={t("ui.karaoke.repeat.topic")}>{t("ui.karaoke.repeat.help")}</HelpTip>}
        />
        <div className="flex items-center gap-1">
          <Button icon={RotateCcw} onClick={() => onCommand(backend.replayLine)} title={t("ui.karaoke.replay.tip")}>
            {t("ui.karaoke.replay.label")}
          </Button>
          <HelpTip topic={t("ui.karaoke.replay.topic")}>{t("ui.karaoke.replay.help")}</HelpTip>
        </div>
      </div>
    </Card>
  );
}

/** PassageOptionsState is how a passage is studied, as the card edits it. */
interface PassageOptionsState {
  snap: boolean;
  loop: boolean;
  gap: number;
  progressive: boolean;
  ladder: boolean;
  goal: number;
}

function PassageCard({ state, label, onCommand }: TrainingCardsProps) {
  const { passage } = state;
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [name, setName] = useState("");
  // The options start as the session has them: the ones the end key uses.
  const [options, setOptions] = useState<PassageOptionsState>({ snap: state.passageOptions.snap, ...state.passageOptions.plan });
  const startKey = label("passage-mark-start");
  const endKey = label("passage-mark-end");

  // A changed option is also the next marked passage's.
  const change = (patch: Partial<PassageOptionsState>) => {
    const next = { ...options, ...patch };
    setOptions(next);
    const { snap, ...plan } = next;
    onCommand(() => backend.setPassageOptions({ snap, plan }));
  };
  const mark = (set: (value: string) => void) => (
    <Button icon={Flag} onClick={() => set(passageClock(state.position))} title={t("ui.passage.mark.tip")}>
      {t("ui.passage.mark.label")}
    </Button>
  );
  const study = () => onCommand(() => backend.studyPassage({ start, end, ...options }));
  const save = () =>
    onCommand(async () => {
      await backend.savePassage(name);
      setName("");
    });
  const thisLine = () =>
    onCommand(async () => {
      const times = await backend.passageOfLine();
      setStart(times.start);
      setEnd(times.end);
    });

  return (
    <Card title={t("ui.passage.title")} aside={<HelpTip topic={t("ui.passage.topic")}>{t("ui.passage.help")}</HelpTip>}>
      <div className="flex flex-col gap-4">
        <p className="text-sm text-fg-muted">
          <Rich
            text={t("ui.passage.hint.text")}
            nodes={{
              start: startKey ? <kbd>{startKey}</kbd> : t("ui.passage.hint.startNone"),
              end: endKey ? <kbd>{endKey}</kbd> : t("ui.passage.hint.endNone"),
            }}
          />
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <Button icon={Flag} onClick={() => onCommand(() => backend.runShortcut("passage-mark-start", false))}>
            {withKey(t("ui.passage.markStart"), startKey)}
          </Button>
          <Button
            icon={Bookmark}
            disabled={state.markedStart === ""}
            title={state.markedStart === "" ? t("ui.passage.markStartFirst") : undefined}
            onClick={() => onCommand(() => backend.runShortcut("passage-mark-end", false))}
          >
            {withKey(t("ui.passage.markEnd"), endKey)}
          </Button>
          {state.markedStart && (
            <span role="status" className="text-sm text-fg">
              {t("ui.passage.marked", { time: state.markedStart })}
            </span>
          )}
        </div>

        {passage.active && (
          <div className="rounded-control border border-line bg-surface-sunken p-3 text-sm text-fg" role="status">
            <p>
              {passage.goal > 0
                ? t("ui.passage.progressGoal", { start: passage.start, end: passage.end, lap: passage.lap, goal: passage.goal })
                : t("ui.passage.progress", { start: passage.start, end: passage.end, lap: passage.lap })}
            </p>
            {passage.suspended && <p className="mt-1 text-warning">{t("ui.passage.suspended")}</p>}
            {passage.atEnd && <p className="mt-1 text-fg-muted">{t("ui.passage.atEnd")}</p>}
            <div className="mt-3 flex flex-wrap gap-2">
              {passage.suspended && (
                <Button icon={Undo2} onClick={() => onCommand(backend.resumePassage)}>
                  {t("ui.passage.resume")}
                </Button>
              )}
              <Button icon={LogOut} onClick={() => onCommand(backend.stopPassage)}>
                {t("ui.passage.leave")}
              </Button>
            </div>
            <div className="mt-3 border-t border-line pt-3">
              <TextField
                label={t("ui.passage.saveAs.label")}
                value={name}
                onChange={setName}
                placeholder={t("ui.passage.saveAs.placeholder", { start: passage.start, end: passage.end })}
                inputMode="text"
                action={
                  <Button icon={Bookmark} onClick={save}>
                    {t("ui.passage.saveAs.button")}
                  </Button>
                }
                help={<HelpTip topic={t("ui.passage.saveAs.topic")}>{t("ui.passage.saveAs.help")}</HelpTip>}
              />
            </div>
          </div>
        )}

        {state.savedPassages.length > 0 && (
          <div className="flex flex-col gap-2">
            <p className="text-sm font-medium text-fg">{t("ui.passage.saved")}</p>
            <SavedPassages
              passages={state.savedPassages}
              studyLabel={t("ui.passage.study")}
              onStudy={(saved) => onCommand(() => backend.studySavedPassage(saved))}
              onRename={(from, to) => backend.renamePassage(from, to)}
              onDelete={async (saved) => onCommand(() => backend.deletePassage(saved))}
            />
          </div>
        )}

        <details className="group rounded-control border border-line px-3 py-2">
          <summary className="cursor-pointer text-sm font-medium text-fg">{t("ui.passage.more")}</summary>
          <div className="mt-3 flex flex-col gap-4">
            <div className="grid grid-cols-2 gap-3">
              <TextField label={t("ui.passage.start")} value={start} onChange={setStart} placeholder="1:10:23" action={mark(setStart)} />
              <TextField label={t("ui.passage.end")} value={end} onChange={setEnd} placeholder="1:11:04" action={mark(setEnd)} />
            </div>
            <div className="flex items-center gap-1">
              <Button onClick={thisLine}>{t("ui.passage.thisLine.label")}</Button>
              <HelpTip topic={t("ui.passage.thisLine.topic")}>{t("ui.passage.thisLine.help")}</HelpTip>
            </div>
            <Toggle
              label={t("ui.passage.snap.label")}
              checked={options.snap}
              onChange={(snap) => change({ snap })}
              help={<HelpTip topic={t("ui.passage.snap.topic")}>{t("ui.passage.snap.help")}</HelpTip>}
            />
            <Select
              label={t("ui.passage.onEnd.label")}
              value={options.loop ? "loop" : "pause"}
              options={[
                { value: "pause", label: t("ui.passage.onEnd.pause") },
                { value: "loop", label: t("ui.passage.onEnd.loop") },
              ]}
              onChange={(value) => change({ loop: value === "loop" })}
              help={<HelpTip topic={t("ui.passage.onEnd.topic")}>{t("ui.passage.onEnd.help")}</HelpTip>}
            />
            {options.loop && (
              <Select
                label={t("ui.passage.gap.label")}
                value={String(options.gap)}
                options={gapOptions()}
                onChange={(value) => change({ gap: Number(value) })}
              />
            )}
            <Toggle
              label={t("ui.passage.progressive.label")}
              checked={options.progressive}
              onChange={(progressive) => change({ progressive })}
              help={<HelpTip topic={t("ui.passage.progressive.topic")}>{t("ui.passage.progressive.help")}</HelpTip>}
            />
            <Toggle
              label={t("ui.passage.ladder.label")}
              checked={options.ladder}
              onChange={(ladder) => change({ ladder })}
              help={<HelpTip topic={t("ui.passage.ladder.topic")}>{t("ui.passage.ladder.help")}</HelpTip>}
            />
            <Select
              label={t("ui.passage.goal.label")}
              value={String(options.goal)}
              options={goalOptions()}
              onChange={(value) => change({ goal: Number(value) })}
              help={<HelpTip topic={t("ui.passage.goal.topic")}>{t("ui.passage.goal.help")}</HelpTip>}
            />
            <Button variant="primary" icon={BookOpenCheck} onClick={study}>
              {t("ui.passage.studyNow")}
            </Button>
          </div>
        </details>
      </div>
    </Card>
  );
}
