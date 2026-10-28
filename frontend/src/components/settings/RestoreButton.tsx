import type { ReactNode } from "react";
import { t } from "../../i18n";

interface RestoreButtonProps {
  onRestore: () => void;
  /** help is the card's (?), shown beside the button in the card's corner. */
  help?: ReactNode;
}

/**
 * RestoreButton puts one group of the settings back to the default. Like any
 * other change, it waits for Save, and Discard undoes it.
 */
export function RestoreButton({ onRestore, help }: RestoreButtonProps) {
  const button = (
    <button
      type="button"
      onClick={onRestore}
      className="h-7 rounded-control px-2 text-xs text-fg-secondary hover:bg-sunken hover:text-fg focus-visible:outline-2 focus-visible:outline-[var(--sp-focus)]"
    >
      {t("ui.settings.restoreGroup")}
    </button>
  );
  if (!help) {
    return button;
  }
  return (
    <div className="flex items-center gap-1">
      {button}
      {help}
    </div>
  );
}
