import { CircleDot, FileDown, Flag, PencilLine, Redo2, RotateCcw, Save, Undo2, X } from "lucide-react";
import { useId, useState } from "react";
import { editClock } from "../../format";
import { withKey } from "../../keys";
import { backend } from "../../services/backend";
import type { Editing, Line, Playback } from "../../services/backend";
import { Button } from "../ui/Button";
import { TextField } from "../ui/Field";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";

/** pausedOnly is why the editor's buttons are off while the video plays. */
const pausedOnly = "Pause o vídeo para corrigir uma legenda";

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

  return (
    <Card
      title="Corrigir legendas"
      aside={
        <HelpTip topic="corrigir legendas">
          Com o vídeo pausado, clique numa legenda na janela do filme, ou use os botões abaixo, para corrigir o texto ou os tempos da fala.
          O original nunca se perde, e as correções ficam gravadas num arquivo ao lado do vídeo, que nunca é alterado.
        </HelpTip>
      }
    >
      <div className="flex flex-col gap-4">
        {editing.staleEdits > 0 && (
          <p role="status" className="text-sm text-warning">
            {editing.staleEdits === 1
              ? "Uma correção gravada não vale mais: a fala original mudou no arquivo da legenda."
              : `${editing.staleEdits} correções gravadas não valem mais: as falas originais mudaram no arquivo da legenda.`}
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
          <div className="flex flex-wrap gap-2">
            <Button icon={PencilLine} disabled={playing} title={playing ? pausedOnly : undefined} onClick={() => pick("primary")}>
              Corrigir a legenda de cima
            </Button>
            <Button icon={PencilLine} disabled={playing} title={playing ? pausedOnly : undefined} onClick={() => pick("translation")}>
              Corrigir a de baixo
            </Button>
          </div>
        )}
        <div className="flex flex-wrap items-center gap-2 border-t border-line pt-4">
          <Button
            icon={Undo2}
            disabled={playing || !editing.canUndo}
            title={playing ? pausedOnly : withKey("Desfazer a última correção", label("undo"))}
            onClick={() => onCommand(backend.undoEdit)}
          >
            Desfazer
          </Button>
          <Button
            icon={Redo2}
            disabled={playing || !editing.canRedo}
            title={playing ? pausedOnly : withKey("Refazer a correção desfeita", label("redo"))}
            onClick={() => onCommand(backend.redoEdit)}
          >
            Refazer
          </Button>
          <span className="flex-1" />
          <Button icon={FileDown} variant="ghost" onClick={() => exportLine("primary")}>
            Exportar a legenda de cima
          </Button>
          <Button icon={FileDown} variant="ghost" onClick={() => exportLine("translation")}>
            Exportar a de baixo
          </Button>
          <HelpTip topic="exportar legenda">
            Grava a legenda como você a vê, com as correções e a sincronização, num arquivo .srt ou .ass que você escolhe. Serve para usar
            em outro player.
          </HelpTip>
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
    <Button icon={Flag} onClick={() => set(editClock(position))} title="Usar a posição atual do vídeo">
      Posição atual
    </Button>
  );

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-baseline justify-between gap-3">
        <p className="text-sm font-medium text-fg">{editing.lineName}</p>
        {unsaved && (
          <p role="status" className="flex items-center gap-1.5 text-sm text-warning">
            <CircleDot aria-hidden className="size-3.5" />
            Não salvo
          </p>
        )}
      </div>
      <div className="rounded-control border border-line bg-surface-sunken p-3 text-sm">
        <p className="mb-1 text-xs font-medium tracking-wide text-fg-muted uppercase">
          Original · {editing.originalStart} – {editing.originalEnd}
        </p>
        <p className="whitespace-pre-wrap text-fg-secondary">{editing.original}</p>
      </div>
      <div className="flex flex-col gap-1.5">
        <label htmlFor={textId} className="text-sm font-medium text-fg">
          Sua versão
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
        <TextField label="Início" value={start} onChange={setStart} placeholder="1:02:03,456" action={now(setStart)} />
        <TextField label="Fim" value={end} onChange={setEnd} placeholder="1:02:05,000" action={now(setEnd)} />
      </div>
      <div className="flex flex-wrap gap-2">
        <Button
          variant="primary"
          icon={Save}
          disabled={playing || !unsaved}
          title={playing ? "Pause o vídeo para salvar" : undefined}
          onClick={() => onCommand(() => backend.editLine({ text, start, end }))}
        >
          Salvar
        </Button>
        <Button icon={X} onClick={() => onCommand(backend.clearSelection)}>
          Cancelar
        </Button>
        <Button
          icon={RotateCcw}
          disabled={playing || !editing.edited}
          title={playing ? pausedOnly : "Volta a fala ao texto e aos tempos do arquivo da legenda"}
          onClick={() => onCommand(backend.restoreLine)}
        >
          Restaurar o original
        </Button>
      </div>
    </div>
  );
}
