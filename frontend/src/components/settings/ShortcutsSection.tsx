import { useCallback } from "react";
import { t } from "../../i18n";
import type { ShortcutSetting } from "../../services/backend";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import type { SectionProps } from "./section";
import { ShortcutCapture } from "./ShortcutCapture";

/** useShortcutEditing gives what a ShortcutCapture of the draft needs. */
export function useShortcutEditing({ view, draft, setDraft }: Pick<SectionProps, "view" | "draft" | "setDraft">) {
  const defaults = view.options.defaults.shortcuts;
  // Commands without a shortcut share no chord.
  const taken = useCallback(
    (command: string) => (chord: string) =>
      chord === "" ? undefined : draft.shortcuts.find((item) => item.chord === chord && item.command !== command)?.name,
    [draft.shortcuts],
  );
  const set = useCallback(
    (command: string, chord: string, label: string) =>
      setDraft((current) =>
        current
          ? { ...current, shortcuts: current.shortcuts.map((item) => (item.command === command ? { ...item, chord, label } : item)) }
          : current,
      ),
    [setDraft],
  );
  const defaultOf = (command: string) => defaults.find((item) => item.command === command);
  return { taken, set, defaultOf };
}

/** ShortcutsSection lists the commands by group, each with its keys. */
export function ShortcutsSection(props: SectionProps) {
  const { draft } = props;
  const { taken, set, defaultOf } = useShortcutEditing(props);
  const groups = new Map<string, ShortcutSetting[]>();
  for (const item of draft.shortcuts) {
    groups.set(item.category, [...(groups.get(item.category) ?? []), item]);
  }

  return (
    <>
      <p className="text-sm leading-relaxed text-fg-secondary">{t("ui.settings.shortcuts.intro")}</p>
      {[...groups.entries()].map(([category, items]) => (
        <Card
          key={category}
          title={category}
          aside={
            // The sync group, known by its commands: its name comes translated.
            items.some((item) => item.command.startsWith("sync-")) ? (
              <HelpTip topic={t("ui.settings.shortcuts.syncTopic")}>{t("ui.settings.shortcuts.syncHelp")}</HelpTip>
            ) : undefined
          }
        >
          <ul className="flex flex-col divide-y divide-line">
            {items.map((item) => {
              const standard = defaultOf(item.command);
              return (
                <li key={item.command} className="flex items-center justify-between gap-4 py-2.5">
                  <span className="text-sm text-fg">{item.name}</span>
                  <ShortcutCapture
                    name={item.name}
                    label={item.label}
                    taken={taken(item.command)}
                    onChange={(chord, label) => set(item.command, chord, label)}
                    onClear={item.chord !== "" ? () => set(item.command, "", "") : undefined}
                    onDefault={
                      standard && standard.chord !== item.chord && !taken(item.command)(standard.chord)
                        ? () => set(item.command, standard.chord, standard.label)
                        : undefined
                    }
                  />
                </li>
              );
            })}
          </ul>
        </Card>
      ))}
    </>
  );
}
