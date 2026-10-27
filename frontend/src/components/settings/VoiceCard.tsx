import { useEffect, useRef, useState } from "react";
import { t } from "../../i18n";
import { backend, type VoiceSettings } from "../../services/backend";
import { Banner, Card } from "../ui/Feedback";
import { Toggle } from "../ui/Field";
import { HelpTip } from "../ui/HelpTip";
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
  const timer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(timer.current), []);

  const following = voice.model && voice.follow;
  const save = (next: Partial<VoiceSettings>) => void run(() => backend.setVoice({ ...voice, ...next }));
  const changeDelay = (ms: number) => {
    setDelay(ms);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      setDelay(null);
      save({ delayMs: ms });
    }, delaySaveAfter);
  };

  return (
    <Card
      title={t("ui.settings.voice.title")}
      aside={<HelpTip topic={t("ui.settings.voice.topic")}>{t("ui.settings.voice.help")}</HelpTip>}
    >
      <div className="flex flex-col gap-5">
        {voice.weak && <Banner tone="warning">{t("ui.settings.voice.weak")}</Banner>}
        <Toggle
          label={t("ui.settings.voice.model.label")}
          checked={voice.model}
          onChange={(model) => save({ model })}
          help={<HelpTip topic={t("ui.settings.voice.model.topic")}>{t("ui.settings.voice.model.help")}</HelpTip>}
        />
        {voice.battery && (
          <Toggle
            label={t("ui.settings.voice.battery.label")}
            checked={voice.modelOnBattery}
            disabled={!voice.model}
            onChange={(modelOnBattery) => save({ modelOnBattery })}
            help={<HelpTip topic={t("ui.settings.voice.battery.topic")}>{t("ui.settings.voice.battery.help")}</HelpTip>}
          />
        )}
        <Toggle
          label={t("ui.settings.voice.follow.label")}
          checked={voice.follow}
          disabled={!voice.model}
          onChange={(follow) => save({ follow })}
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
          help={<HelpTip topic={t("ui.settings.voice.sensitivity.topic")}>{t("ui.settings.voice.sensitivity.help")}</HelpTip>}
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
          help={<HelpTip topic={t("ui.settings.voice.delay.topic")}>{t("ui.settings.voice.delay.help")}</HelpTip>}
        />
        <Toggle
          label={t("ui.settings.voice.busy.label")}
          checked={voice.evenWhenBusy}
          disabled={!following}
          onChange={(evenWhenBusy) => save({ evenWhenBusy })}
          help={<HelpTip topic={t("ui.settings.voice.busy.topic")}>{t("ui.settings.voice.busy.help")}</HelpTip>}
        />
      </div>
    </Card>
  );
}
