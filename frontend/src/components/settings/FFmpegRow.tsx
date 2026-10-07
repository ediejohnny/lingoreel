import { Download, FolderOpen, X } from "lucide-react";
import { useEffect, useState } from "react";
import { t } from "../../i18n";
import { backend, type VoiceSettings } from "../../services/backend";
import { Button } from "../ui/Button";
import { HelpTip } from "../ui/HelpTip";
import type { SectionProps } from "./section";

interface FFmpegRowProps extends Pick<SectionProps, "run"> {
  /** voice holds the FFmpeg chosen, the one found and whether one can be installed: the voice's and the stills' alike. */
  voice: VoiceSettings;
  /** disabled leaves choosing and installing off while nothing uses FFmpeg. */
  disabled: boolean;
  /** found words the FFmpeg in use, missing its absence, help the row's tip, as the card uses it. */
  found: (path: string) => string;
  missing: string;
  help: string;
}

/**
 * FFmpegRow is the user's own FFmpeg, which the app ships none of: the one in use, choosing another, going back to the
 * one found, and installing one with winget.
 */
export function FFmpegRow({ voice, run, disabled, found, missing, help }: FFmpegRowProps) {
  const [installing, setInstalling] = useState(false);
  const [installError, setInstallError] = useState("");

  useEffect(
    () =>
      backend.onInstall((progress) => {
        if (progress.id !== "ffmpeg") {
          return;
        }
        setInstalling(false);
        setInstallError(progress.error);
        if (!progress.error) {
          void run(async () => undefined, t("ui.settings.ffmpeg.installed"));
        }
      }),
    [run],
  );

  const install = () => {
    setInstalling(true);
    setInstallError("");
    backend.installFFmpeg().catch((failure: unknown) => {
      setInstalling(false);
      setInstallError(String(failure));
    });
  };

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-1">
        <span className="text-sm font-medium text-fg">{t("ui.settings.ffmpeg.label")}</span>
        <HelpTip topic={t("ui.settings.ffmpeg.topic")}>{help}</HelpTip>
      </div>
      <p className="text-sm break-all text-fg-muted">{voice.ffmpegFound ? found(voice.ffmpegFound) : missing}</p>
      <div className="flex flex-wrap gap-2">
        <Button
          icon={FolderOpen}
          disabled={disabled}
          onClick={() => void run(() => backend.chooseFFmpeg(), t("ui.settings.ffmpeg.chosen"))}
        >
          {t("ui.settings.ffmpeg.choose")}
        </Button>
        {voice.ffmpegPath && (
          <Button icon={X} onClick={() => void run(() => backend.clearFFmpeg())}>
            {t("ui.settings.ffmpeg.clear")}
          </Button>
        )}
        {voice.canInstall && (
          <Button icon={Download} loading={installing} disabled={disabled} onClick={install}>
            {t("ui.settings.ffmpeg.install")}
          </Button>
        )}
      </div>
      {installError && <p className="text-sm text-error">{installError}</p>}
    </div>
  );
}
