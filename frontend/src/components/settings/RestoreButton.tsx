import { t } from "../../i18n";

/**
 * RestoreButton puts one group of the settings back to the default. Like any
 * other change, it waits for Save, and Discard undoes it.
 */
export function RestoreButton({ onRestore }: { onRestore: () => void }) {
  return (
    <button
      type="button"
      onClick={onRestore}
      className="h-7 rounded-control px-2 text-xs text-fg-secondary hover:bg-sunken hover:text-fg focus-visible:outline-2 focus-visible:outline-[var(--sp-focus)]"
    >
      {t("ui.settings.restoreGroup")}
    </button>
  );
}
