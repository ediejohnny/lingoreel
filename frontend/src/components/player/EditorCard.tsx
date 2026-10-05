import { CircleDot, FileDown, Flag, PencilLine, Redo2, RotateCcw, Save, Undo2, X } from "lucide-react";
import { useId, useState } from "react";
import { editClock } from "../../format";
import { t } from "../../i18n";
import { withKey } from "../../keys";
import { backend } from "../../services/backend";
import type { Editing, Line, Playback } from "../../services/backend";
import { Button } from "../ui/Button";
import { TextField } from "../ui/Field";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";

interface EditorCardProps {
  state: Playback;
  /** label is the key of a command, such as "Ctrl+Z" for "undo". */
  label: (command: string) => string;
  onCommand: (command: () => Promise<void>) => void;
}

/**
 * EditorCard corrects the subtitles while the video is paused: the line
 * picked by a click on the video, or by the buttons here, opens the editor.
 */
export function EditorCard({ state, label, onCommand }: EditorCardProps) {
  const { editing } = state;
  const playing = state.status === "playing";
  const pick = (line: Line) => onCommand(() => backend.selectLine(line));
  const exportLine = (line: Line) => onCommand(() => backend.exportSubtitle(line));
  // pausedOnly is why the editor's buttons are off while the video plays.
  const pausedOnly = t("ui.editor.pausedOnly");

  return (
    <Card title={t("ui.editor.title")} aside={<HelpTip topic={t("ui.editor.topic")}>{t("ui.editor.help")}</HelpTip>}>
      <div className="@container flex flex-col gap-4">
        {editing.staleEdits > 0 && (
          <p role="status" className="text-sm text-warning">
            {t("ui.editor.stale", { count: editing.staleEdits })}
          </p>
        )}
        {editing.active ? (
          <LineEditor
            key={[editing.line, editing.original, editing.text, editing.start, editing.end].join("\u0000")}
            editing={editing}
            position={state.position}
            playing={playing}
            onCommand={onCommand}
          />
        ) : (
          <div className="grid gap-2 @md:grid-cols-2">
            <Button icon={PencilLine} disabled={playing} title={playing ? pausedOnly : undefined} onClick={() => pick("primary")}>
              {t("ui.editor.fix.top")}
            </Button>
            <Button icon={PencilLine} disabled={playing} title={playing ? pausedOnly : undefined} onClick={() => pick("translation")}>
              {t("ui.editor.fix.bottom")}
            </Button>
          </div>
        )}
        <div className="grid grid-cols-2 gap-2 border-t border-line pt-4">
          <Button
            icon={Undo2}
            disabled={playing || !editing.canUndo}
            title={playing ? pausedOnly : withKey(t("ui.editor.undo.tip"), label("undo"))}
            onClick={() => onCommand(backend.undoEdit)}
          >
            {t("ui.editor.undo.label")}
          </Button>
          <Button
            icon={Redo2}
            disabled={playing || !editing.canRedo}
            title={playing ? pausedOnly : withKey(t("ui.editor.redo.tip"), label("redo"))}
            onClick={() => onCommand(backend.redoEdit)}
          >
            {t("ui.editor.redo.label")}
          </Button>
        </div>
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <Button icon={FileDown} variant="ghost" onClick={() => exportLine("primary")}>
            {t("ui.editor.export.top")}
          </Button>
          <Button icon={FileDown} variant="ghost" onClick={() => exportLine("translation")}>
            {t("ui.editor.export.bottom")}
          </Button>
          <HelpTip topic={t("ui.editor.export.topic")}>{t("ui.editor.export.help")}</HelpTip>
        </div>
      </div>
    </Card>
  );
}

interface LineEditorProps {
  editing: Editing;
  position: number;
  playing: boolean;
  onCommand: (command: () => Promise<void>) => void;
}

/** LineEditor holds the draft of one line; a new selection starts a new one. */
function LineEditor({ editing, position, playing, onCommand }: LineEditorProps) {
  const textId = useId();
  const [text, setText] = useState(editing.text);
  const [start, setStart] = useState(editing.start);
  const [end, setEnd] = useState(editing.end);
  const unsaved = text !== editing.text || start !== editing.start || end !== editing.end;

  const now = (set: (value: string) => void) => (
    <Button icon={Flag} onClick={() => set(editClock(position))} title={t("ui.editor.now.tip")}>
      {t("ui.editor.now.label")}
    </Button>
  );

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-baseline justify-between gap-3">
        <p className="text-sm font-medium text-fg">{editing.lineName}</p>
        {unsaved && (
          <p role="status" className="flex items-center gap-1.5 text-sm text-warning">
            <CircleDot aria-hidden className="size-3.5" />
            {t("ui.editor.unsaved")}
          </p>
        )}
      </div>
      <div className="rounded-control border border-line bg-surface-sunken p-3 text-sm">
        <p className="mb-1 text-xs font-medium tracking-wide text-fg-muted uppercase">
          {t("ui.editor.original", { start: editing.originalStart, end: editing.originalEnd })}
        </p>
        <p className="whitespace-pre-wrap text-fg-secondary">{editing.original}</p>
      </div>
      <div className="flex flex-col gap-1.5">
        <label htmlFor={textId} className="text-sm font-medium text-fg">
          {t("ui.editor.yourVersion")}
        </label>
        <textarea
          id={textId}
          value={text}
          rows={3}
          onChange={(event) => setText(event.target.value)}
          className="w-full resize-y rounded-control border border-line-strong bg-surface px-3 py-2 text-sm text-fg"
        />
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <TextField label={t("ui.editor.start")} value={start} onChange={setStart} placeholder="1:02:03,456" action={now(setStart)} />
        <TextField label={t("ui.editor.end")} value={end} onChange={setEnd} placeholder="1:02:05,000" action={now(setEnd)} />
      </div>
      <div className="flex flex-wrap gap-2">
        <Button
          variant="primary"
          icon={Save}
          disabled={playing || !unsaved}
          title={playing ? t("ui.editor.pausedToSave") : undefined}
          onClick={() => onCommand(() => backend.editLine({ text, start, end }))}
        >
          {t("ui.common.save")}
        </Button>
        <Button icon={X} onClick={() => onCommand(backend.clearSelection)}>
          {t("ui.common.cancel")}
        </Button>
        <Button
          icon={RotateCcw}
          disabled={playing || !editing.edited}
          title={playing ? t("ui.editor.pausedOnly") : t("ui.editor.restore.tip")}
          onClick={() => onCommand(backend.restoreLine)}
        >
          {t("ui.editor.restore.label")}
        </Button>
      </div>
    </div>
  );
}
