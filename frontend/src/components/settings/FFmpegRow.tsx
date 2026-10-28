import { Download, FolderOpen, X } from "../ui/icons";
import { useState } from "react";
import { useInstall } from "../../hooks/useInstall";
import { t } from "../../i18n";
import { backend, type VoiceSettings } from "../../services/backend";
import { Button } from "../ui/Button";
import { HelpTip } from "../ui/HelpTip";
import type { SectionProps } from "./section";

interface FFmpegRowProps extends Pick<SectionProps, "run"> {
  /**
   * voice holds the FFmpeg chosen, the one found, whether it was too slow to answer (its first opening or a busy
   * computer, not a missing FFmpeg) and whether one can be installed: the voice's and the stills' alike.
   */
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
  // Installed, the settings are read again: they tell the FFmpeg found.
  const install = useInstall("ffmpeg", () => void run(async () => undefined, t("ui.settings.ffmpeg.installed")));
  // Choosing waits for the file picked to answer, up to a minute on its first opening: the button says it works.
  const [choosing, setChoosing] = useState(false);
  const choose = () => {
    setChoosing(true);
    void run(() => backend.chooseFFmpeg(), t("ui.settings.ffmpeg.chosen")).finally(() => setChoosing(false));
  };

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-1">
        <span className="text-sm font-medium text-fg">{t("ui.settings.ffmpeg.label")}</span>
        <HelpTip topic={t("ui.settings.ffmpeg.topic")}>{help}</HelpTip>
      </div>
      <p className="text-sm break-all text-fg-muted">
        {voice.ffmpegFound ? found(voice.ffmpegFound) : voice.ffmpegSlow ? t("ui.settings.ffmpeg.slow") : missing}
      </p>
      <div className="flex flex-wrap gap-2">
        <Button icon={FolderOpen} loading={choosing} disabled={disabled} onClick={choose}>
          {t("ui.settings.ffmpeg.choose")}
        </Button>
        {voice.ffmpegPath && (
          <Button icon={X} onClick={() => void run(() => backend.clearFFmpeg())}>
            {t("ui.settings.ffmpeg.clear")}
          </Button>
        )}
        {voice.canInstall && (
          <Button icon={Download} loading={install.installing !== ""} disabled={disabled} onClick={() => install.start("ffmpeg")}>
            {t("ui.settings.ffmpeg.install")}
          </Button>
        )}
      </div>
      {install.error && <p className="text-sm text-error">{install.error}</p>}
    </div>
  );
}
