import { t } from "../../i18n";
import type { KaraokeSettings } from "../../services/backend";
import { Card } from "../ui/Feedback";
import { Toggle } from "../ui/Field";
import { HelpTip } from "../ui/HelpTip";
import { ColorField, NumberField, Segmented } from "../ui/Inputs";
import { FillColorNote } from "./FillColorNote";
import { change, type SectionProps } from "./section";
import { ShortcutCapture } from "./ShortcutCapture";
import { useShortcutEditing } from "./ShortcutsSection";

/** StudySection holds the Modo Karaokê and the translation key. */
export function StudySection(props: SectionProps) {
  const { view, draft, setDraft } = props;
  const karaoke = draft.karaoke;
  const limits = view.options.limits;
  const set =
    <K extends keyof KaraokeSettings>(key: K) =>
    (value: KaraokeSettings[K]) =>
      change(setDraft, "karaoke")({ ...karaoke, [key]: value });
  const { taken, set: setShortcut, defaultOf } = useShortcutEditing(props);
  const translationKey = draft.shortcuts.find((item) => item.command === "translation");
  const standardKey = defaultOf("translation");

  return (
    <>
      <Card title={t("ui.settings.study.mode.title")}>
        <Segmented
          label={t("ui.settings.study.mode.label")}
          value={draft.mode}
          options={view.options.modes.map((option) => ({ value: option.value, label: option.label }))}
          onChange={change(setDraft, "mode")}
          help={<HelpTip topic={t("ui.settings.study.mode.topic")}>{t("ui.settings.study.mode.help")}</HelpTip>}
        />
      </Card>
      <Card
        title={t("ui.settings.study.karaoke.title")}
        aside={<HelpTip topic={t("ui.settings.study.karaoke.topic")}>{t("ui.settings.study.karaoke.help")}</HelpTip>}
      >
        <div className="flex flex-col gap-5">
          <Toggle label={t("ui.settings.study.karaoke.enable")} checked={karaoke.enabled} onChange={set("enabled")} />
          <div className="grid gap-4 sm:grid-cols-2">
            <NumberField
              label={t("ui.settings.study.minGap.label")}
              value={karaoke.minGap}
              min={limits.gapMin}
              max={limits.gapMax}
              step={1}
              unit="s"
              onChange={set("minGap")}
              help={<HelpTip topic={t("ui.settings.study.minGap.topic")}>{t("ui.settings.study.minGap.help")}</HelpTip>}
            />
            <NumberField
              label={t("ui.settings.study.lead.label")}
              value={karaoke.lead}
              min={limits.leadMin}
              max={limits.leadMax}
              step={0.5}
              unit="s"
              onChange={set("lead")}
              help={<HelpTip topic={t("ui.settings.study.lead.topic")}>{t("ui.settings.study.lead.help")}</HelpTip>}
            />
            <Segmented
              label={t("ui.settings.study.indicator.label")}
              value={karaoke.indicator}
              options={[
                { value: "bar", label: t("ui.settings.study.indicator.bar") },
                { value: "countdown", label: t("ui.settings.study.indicator.countdown") },
              ]}
              onChange={set("indicator")}
              help={<HelpTip topic={t("ui.settings.study.indicator.topic")}>{t("ui.settings.study.indicator.help")}</HelpTip>}
            />
            <ColorField label={t("ui.settings.study.previewColor")} value={karaoke.previewColor} onChange={set("previewColor")} />
            <Toggle
              label={t("ui.settings.study.fill.label")}
              checked={karaoke.fill}
              onChange={set("fill")}
              help={<HelpTip topic={t("ui.settings.study.fill.topic")}>{t("ui.settings.study.fill.help")}</HelpTip>}
            />
            <ColorField
              label={t("ui.settings.study.fillColor")}
              value={karaoke.fillColor}
              disabled={!karaoke.fill}
              onChange={set("fillColor")}
            />
            <div className="sm:col-span-2">
              <FillColorNote text={draft.primary.color} fill={karaoke.fillColor} enabled={karaoke.fill} onAdopt={set("fillColor")} />
            </div>
            <NumberField
              label={t("ui.settings.study.repeat.label")}
              value={karaoke.repeat}
              min={0}
              max={limits.repeatMax}
              step={1}
              unit="×"
              onChange={set("repeat")}
              help={<HelpTip topic={t("ui.settings.study.repeat.topic")}>{t("ui.settings.study.repeat.help")}</HelpTip>}
            />
          </div>
        </div>
      </Card>
      <Card
        title={t("ui.settings.study.key.title")}
        aside={<HelpTip topic={t("ui.settings.study.key.topic")}>{t("ui.settings.study.key.help")}</HelpTip>}
      >
        <div className="flex flex-col gap-4">
          {translationKey && (
            <div className="flex items-center justify-between gap-4">
              <span className="text-sm text-fg">{t("ui.settings.study.key.label")}</span>
              <ShortcutCapture
                name={translationKey.name}
                label={translationKey.label}
                taken={taken("translation")}
                onChange={(chord, label) => setShortcut("translation", chord, label)}
                onClear={translationKey.chord !== "" ? () => setShortcut("translation", "", "") : undefined}
                onDefault={
                  standardKey && standardKey.chord !== translationKey.chord && !taken("translation")(standardKey.chord)
                    ? () => setShortcut("translation", standardKey.chord, standardKey.label)
                    : undefined
                }
              />
            </div>
          )}
          <Toggle
            label={t("ui.settings.study.key.hold")}
            checked={draft.reveal === "hold"}
            onChange={(hold) => change(setDraft, "reveal")(hold ? "hold" : "toggle")}
          />
        </div>
      </Card>
    </>
  );
}
