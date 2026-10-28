import { useId, type ReactNode } from "react";
import { useEscape } from "../../hooks/useEscape";

interface DialogProps {
  /** title names the dialog: its heading, or the question itself. */
  title: string;
  /** alert is a question that must be answered before going on. */
  alert?: boolean;
  /**
   * within covers only the nearest positioned box instead of the whole window: beside the video, a box over the whole
   * window would sit under the video.
   */
  within?: boolean;
  /** onCancel closes the dialog without an answer: Esc and a click beside it do too. */
  onCancel: () => void;
  children: ReactNode;
}

/** Dialog is a box over the screen that holds the keys and the clicks until it closes. */
export function Dialog({ title, alert = false, within = false, onCancel, children }: DialogProps) {
  const id = useId();
  useEscape(onCancel, { modal: true });
  return (
    <div
      className={`inset-0 flex justify-center p-4 ${within ? "absolute z-20 items-start bg-bg/80" : "fixed z-50 items-center bg-black/50"}`}
      onClick={onCancel}
    >
      <div
        role={alert ? "alertdialog" : "dialog"}
        aria-modal="true"
        aria-labelledby={id}
        className={`flex w-full max-w-md flex-col gap-4 rounded-card border border-line bg-surface p-5 shadow-card ${within ? "mt-8" : ""}`}
        onClick={(event) => event.stopPropagation()}
      >
        <h2 id={id} className="text-base font-semibold text-fg">
          {title}
        </h2>
        {children}
      </div>
    </div>
  );
}
