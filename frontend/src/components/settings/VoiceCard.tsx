import { useState } from "react";
import { useDebouncedCall } from "../../hooks/useDebouncedCall";
import { t } from "../../i18n";
import { backend, type VoiceSettings } from "../../services/backend";
import { Banner, Card } from "../ui/Feedback";
import { Toggle } from "../ui/Field";
import { KeyedTip } from "../ui/HelpTip";
import { NumberField, Segmented } from "../ui/Inputs";
import { FFmpegRow } from "./FFmpegRow";
import type { SectionProps } from "./section";

/** delaySaveAfter is how long the delay waits for the slider to stop before it is saved. */
const delaySaveAfter = 400;

/**
 * VoiceCard is how the app hears the actors' voice on this computer, for every profile: whether the voice AI works here
 * (and on the battery), whether the karaoke mode follows the voice; each choice is saved at once, the FFmpeg is found,
 * chosen or installed here.
 */
export function VoiceCard({ view, run }: Pick<SectionProps, "view" | "run">) {
  const voice = view.voice;
  const [delay, setDelay] = useState<number | null>(null);

  const following = voice.model && voice.follow;
  const save = (next: Partial<VoiceSettings>) => void run(() => backend.setVoice({ ...voice, ...next }));
  const saveDelay = useDebouncedCall((ms: number) => {
    setDelay(null);
    save({ delayMs: ms });
  }, delaySaveAfter);
  const changeDelay = (ms: number) => {
    setDelay(ms);
    saveDelay(ms);
  };

  return (
    <Card title={t("ui.settings.voice.title")} aside={<KeyedTip k="ui.settings.voice" />}>
      <div className="flex flex-col gap-5">
        {voice.weak && <Banner tone="warning">{t("ui.settings.voice.weak")}</Banner>}
        <Toggle
          label={t("ui.settings.voice.model.label")}
          checked={voice.model}
          onChange={(model) => save({ model })}
          help={<KeyedTip k="ui.settings.voice.model" />}
        />
        {voice.battery && (
          <Toggle
            label={t("ui.settings.voice.battery.label")}
            checked={voice.modelOnBattery}
            disabled={!voice.model}
            onChange={(modelOnBattery) => save({ modelOnBattery })}
            help={<KeyedTip k="ui.settings.voice.battery" />}
          />
        )}
        <Toggle
          label={t("ui.settings.voice.follow.label")}
          checked={voice.follow}
          disabled={!voice.model}
          onChange={(follow) => save({ follow })}
        />
        <Toggle
          label={t("ui.settings.voice.sync.label")}
          checked={voice.sync}
          disabled={!voice.model}
          onChange={(sync) => save({ sync })}
          help={<KeyedTip k="ui.settings.voice.sync" />}
        />
        <Segmented
          label={t("ui.settings.voice.sensitivity.label")}
          value={voice.sensitivity}
          options={[
            { value: "low", label: t("ui.settings.voice.sensitivity.low") },
            { value: "normal", label: t("ui.settings.voice.sensitivity.normal") },
            { value: "high", label: t("ui.settings.voice.sensitivity.high") },
          ]}
          onChange={(sensitivity) => save({ sensitivity })}
          help={<KeyedTip k="ui.settings.voice.sensitivity" />}
        />
        <FFmpegRow
          voice={voice}
          run={run}
          disabled={!following}
          found={(path) => t("ui.settings.voice.ffmpeg.found", { path })}
          missing={t("ui.settings.voice.ffmpeg.missing")}
          help={t("ui.settings.voice.ffmpeg.help")}
        />
        <NumberField
          label={t("ui.settings.voice.delay.label")}
          value={delay ?? voice.delayMs}
          min={0}
          max={1000}
          step={10}
          unit="ms"
          disabled={!following}
          onChange={changeDelay}
          help={<KeyedTip k="ui.settings.voice.delay" />}
        />
        <Toggle
          label={t("ui.settings.voice.busy.label")}
          checked={voice.evenWhenBusy}
          disabled={!following}
          onChange={(evenWhenBusy) => save({ evenWhenBusy })}
          help={<KeyedTip k="ui.settings.voice.busy" />}
        />
      </div>
    </Card>
  );
}
