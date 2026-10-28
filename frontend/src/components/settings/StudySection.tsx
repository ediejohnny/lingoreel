import { t } from "../../i18n";
import type { KaraokeSettings } from "../../services/backend";
import { Card } from "../ui/Feedback";
import { Toggle } from "../ui/Field";
import { KeyedTip } from "../ui/HelpTip";
import { ColorField, NumberField, Segmented } from "../ui/Inputs";
import { FillColorNote } from "./FillColorNote";
import { change, type SectionProps } from "./section";
import { ShortcutCapture } from "./ShortcutCapture";
import { useShortcutEditing } from "./ShortcutsSection";
import { SpeechPacksCard } from "./SpeechPacksCard";
import { VoiceCard } from "./VoiceCard";

/** StudySection holds the karaoke mode and the translation key. */
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
          help={<KeyedTip k="ui.settings.study.mode" />}
        />
      </Card>
      <Card title={t("ui.settings.study.karaoke.title")} aside={<KeyedTip k="ui.settings.study.karaoke" />}>
        <div className="flex flex-col gap-5">
          <Toggle label={t("ui.settings.study.karaoke.enable")} checked={karaoke.enabled} onChange={set("enabled")} />
          <div className="grid grid-cols-[minmax(0,1fr)] gap-4 sm:grid-cols-2">
            <NumberField
              label={t("ui.settings.study.minGap.label")}
              value={karaoke.minGap}
              min={limits.gapMin}
              max={limits.gapMax}
              step={1}
              unit="s"
              onChange={set("minGap")}
              help={<KeyedTip k="ui.settings.study.minGap" />}
            />
            <NumberField
              label={t("ui.settings.study.lead.label")}
              value={karaoke.lead}
              min={limits.leadMin}
              max={limits.leadMax}
              step={0.5}
              unit="s"
              onChange={set("lead")}
              help={<KeyedTip k="ui.settings.study.lead" />}
            />
            <Segmented
              label={t("ui.settings.study.indicator.label")}
              value={karaoke.indicator}
              options={[
                { value: "bar", label: t("ui.settings.study.indicator.bar") },
                { value: "countdown", label: t("ui.settings.study.indicator.countdown") },
              ]}
              onChange={set("indicator")}
              help={<KeyedTip k="ui.settings.study.indicator" />}
            />
            <ColorField label={t("ui.settings.study.previewColor")} value={karaoke.previewColor} onChange={set("previewColor")} />
            <Toggle
              label={t("ui.settings.study.fill.label")}
              checked={karaoke.fill}
              onChange={set("fill")}
              help={<KeyedTip k="ui.settings.study.fill" />}
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
              help={<KeyedTip k="ui.settings.study.repeat" />}
            />
          </div>
        </div>
      </Card>
      <VoiceCard view={view} run={props.run} />
      <SpeechPacksCard />
      <Card title={t("ui.settings.study.key.title")} aside={<KeyedTip k="ui.settings.study.key" />}>
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
