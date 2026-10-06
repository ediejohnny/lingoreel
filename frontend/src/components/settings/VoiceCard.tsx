import { Download, FolderOpen, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { t } from "../../i18n";
import { backend, type VoiceSettings } from "../../services/backend";
import { Button } from "../ui/Button";
import { Card } from "../ui/Feedback";
import { Toggle } from "../ui/Field";
import { HelpTip } from "../ui/HelpTip";
import { NumberField, Segmented } from "../ui/Inputs";
import type { SectionProps } from "./section";

/** delaySaveAfter is how long the delay waits for the slider to stop before it is saved. */
const delaySaveAfter = 400;

/**
 * VoiceCard is how the karaoke mode follows the actors' voice on this computer, for every profile: each choice is
 * saved at once, the FFmpeg is found, chosen or installed here.
 */
export function VoiceCard({ view, run }: Pick<SectionProps, "view" | "run">) {
  const voice = view.voice;
  const [installing, setInstalling] = useState(false);
  const [installError, setInstallError] = useState("");
  const [delay, setDelay] = useState<number | null>(null);
  const timer = useRef<number | undefined>(undefined);

  useEffect(
    () =>
      backend.onInstall((progress) => {
        if (progress.id !== "ffmpeg") {
          return;
        }
        setInstalling(false);
        setInstallError(progress.error);
        if (!progress.error) {
          void run(async () => undefined, t("ui.settings.voice.ffmpeg.installed"));
        }
      }),
    [run],
  );
  useEffect(() => () => window.clearTimeout(timer.current), []);

  const save = (next: Partial<VoiceSettings>) => void run(() => backend.setVoice({ ...voice, ...next }));
  const changeDelay = (ms: number) => {
    setDelay(ms);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      setDelay(null);
      save({ delayMs: ms });
    }, delaySaveAfter);
  };
  const install = () => {
    setInstalling(true);
    setInstallError("");
    backend.installFFmpeg().catch((failure: unknown) => {
      setInstalling(false);
      setInstallError(String(failure));
    });
  };

  return (
    <Card
      title={t("ui.settings.voice.title")}
      aside={<HelpTip topic={t("ui.settings.voice.topic")}>{t("ui.settings.voice.help")}</HelpTip>}
    >
      <div className="flex flex-col gap-5">
        <Toggle label={t("ui.settings.voice.follow.label")} checked={voice.follow} onChange={(follow) => save({ follow })} />
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
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-1">
            <span className="text-sm font-medium text-fg">{t("ui.settings.voice.ffmpeg.label")}</span>
            <HelpTip topic={t("ui.settings.voice.ffmpeg.topic")}>{t("ui.settings.voice.ffmpeg.help")}</HelpTip>
          </div>
          <p className="text-sm break-all text-fg-muted">
            {voice.ffmpegFound ? t("ui.settings.voice.ffmpeg.found", { path: voice.ffmpegFound }) : t("ui.settings.voice.ffmpeg.missing")}
          </p>
          <div className="flex flex-wrap gap-2">
            <Button
              icon={FolderOpen}
              disabled={!voice.follow}
              onClick={() => void run(() => backend.chooseFFmpeg(), t("ui.settings.voice.ffmpeg.chosen"))}
            >
              {t("ui.settings.voice.ffmpeg.choose")}
            </Button>
            {voice.ffmpegPath && (
              <Button icon={X} onClick={() => void run(() => backend.clearFFmpeg())}>
                {t("ui.settings.voice.ffmpeg.clear")}
              </Button>
            )}
            {voice.canInstall && (
              <Button icon={Download} loading={installing} disabled={!voice.follow} onClick={install}>
                {t("ui.settings.voice.ffmpeg.install")}
              </Button>
            )}
          </div>
          {installError && <p className="text-sm text-error">{installError}</p>}
        </div>
        <NumberField
          label={t("ui.settings.voice.delay.label")}
          value={delay ?? voice.delayMs}
          min={0}
          max={1000}
          step={10}
          unit="ms"
          disabled={!voice.follow}
          onChange={changeDelay}
          help={<HelpTip topic={t("ui.settings.voice.delay.topic")}>{t("ui.settings.voice.delay.help")}</HelpTip>}
        />
        <Toggle
          label={t("ui.settings.voice.busy.label")}
          checked={voice.evenWhenBusy}
          disabled={!voice.follow}
          onChange={(evenWhenBusy) => save({ evenWhenBusy })}
          help={<HelpTip topic={t("ui.settings.voice.busy.topic")}>{t("ui.settings.voice.busy.help")}</HelpTip>}
        />
      </div>
    </Card>
  );
}
