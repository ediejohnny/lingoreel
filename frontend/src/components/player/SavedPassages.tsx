import { BookOpenCheck, Check, Pencil, Trash, X } from "../ui/icons";
import { useState } from "react";
import { t } from "../../i18n";
import { messageOf, type SavedPassage } from "../../services/backend";
import { Button, IconButton } from "../ui/Button";

interface SavedPassagesProps {
  passages: SavedPassage[];
  /** studyLabel is the study button's text: "Study passage" or "Study this passage". */
  studyLabel: string;
  /** studyDisabled turns every study button off, with the reason in its tip. */
  studyDisabled?: string;
  onStudy: (name: string) => void;
  onRename: (from: string, to: string) => Promise<void>;
  /** onDelete deletes a passage; the confirmation stays until a promise it returns settles. */
  onDelete: (name: string) => void | Promise<void>;
}

/**
 * SavedPassages lists a video's saved passages: each one studied, renamed in
 * place or deleted after a confirmation. A passage past the end of this
 * video (a file of another cut) shows why it cannot be studied.
 */
export function SavedPassages({ passages, studyLabel, studyDisabled, onStudy, onRename, onDelete }: SavedPassagesProps) {
  return (
    <ul className="flex flex-col gap-1.5">
      {passages.map((passage) => (
        <PassageRow
          key={passage.name}
          passage={passage}
          studyLabel={studyLabel}
          studyDisabled={studyDisabled}
          onStudy={onStudy}
          onRename={onRename}
          onDelete={onDelete}
        />
      ))}
    </ul>
  );
}

type RowProps = Omit<SavedPassagesProps, "passages"> & { passage: SavedPassage };

function PassageRow({ passage, studyLabel, studyDisabled, onStudy, onRename, onDelete }: RowProps) {
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(passage.name);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState("");

  const rename = async () => {
    if (name.trim() === passage.name) {
      setEditing(false);
      return;
    }
    try {
      await onRename(passage.name, name);
      setEditing(false);
      setError("");
    } catch (failure) {
      setError(messageOf(failure));
    }
  };
  const cancel = () => {
    setName(passage.name);
    setEditing(false);
    setError("");
  };
  const blocked = studyDisabled || (passage.outside ? t("ui.passages.outside.tip") : "");

  return (
    <li className="flex flex-col gap-1 rounded-control border border-line px-3 py-2 text-sm">
      <div className="flex flex-wrap items-center gap-2">
        {editing ? (
          <input
            aria-label={t("ui.passages.rename.field")}
            className="h-8 min-w-0 flex-1 rounded-control border border-line-strong bg-surface px-2 text-sm text-fg"
            value={name}
            maxLength={80}
            autoFocus
            onChange={(event) => setName(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                void rename();
              } else if (event.key === "Escape") {
                cancel();
              }
            }}
          />
        ) : (
          <span className="min-w-0 flex-1 truncate font-medium text-fg" title={passage.name}>
            {passage.name}
          </span>
        )}
        <span className="text-fg-muted tabular-nums">
          {passage.start}–{passage.end}
        </span>
        {editing ? (
          <>
            <IconButton icon={Check} label={t("ui.passages.rename.save")} onClick={() => void rename()} />
            <IconButton icon={X} label={t("ui.common.cancel")} onClick={cancel} />
          </>
        ) : confirming ? (
          <>
            <span className="text-fg">{t("ui.passages.delete.confirm")}</span>
            <Button variant="secondary" onClick={() => void Promise.resolve(onDelete(passage.name)).finally(() => setConfirming(false))}>
              {t("ui.passages.delete.yes")}
            </Button>
            <Button variant="ghost" onClick={() => setConfirming(false)}>
              {t("ui.common.no")}
            </Button>
          </>
        ) : (
          <>
            <Button
              variant="ghost"
              icon={BookOpenCheck}
              disabled={blocked !== ""}
              title={blocked || undefined}
              onClick={() => onStudy(passage.name)}
            >
              {studyLabel}
            </Button>
            <IconButton icon={Pencil} label={t("ui.passages.rename.label", { name: passage.name })} onClick={() => setEditing(true)} />
            <IconButton icon={Trash} label={t("ui.passages.delete.label", { name: passage.name })} onClick={() => setConfirming(true)} />
          </>
        )}
      </div>
      <p className={`text-xs ${passage.outside ? "text-warning" : "text-fg-muted"}`}>
        {passage.outside ? t("ui.passages.outside.note") : passage.summary}
      </p>
      {error && (
        <p role="alert" className="text-xs text-warning">
          {error}
        </p>
      )}
    </li>
  );
}
