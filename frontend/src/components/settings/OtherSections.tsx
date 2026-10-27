import { FolderOpen, RotateCcw } from "lucide-react";
import { useState } from "react";
import { speedLabel } from "../../format";
import { t } from "../../i18n";
import { both, keyLabel } from "../../keys";
import { backend } from "../../services/backend";
import { Button } from "../ui/Button";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import { NumberField } from "../ui/Inputs";
import { Select } from "../ui/Select";
import { AssociationCard } from "./AssociationCard";
import { DiagnosticsCard } from "./DiagnosticsCard";
import { change, type SectionProps } from "./section";

/** PlaybackSection holds how a video starts, its speed and its audio, and the precision step. */
export function PlaybackSection({ view, draft, setDraft }: SectionProps) {
  const options = view.options;
  const precisionKeys = both(keyLabel(draft.shortcuts, "precision-back"), keyLabel(draft.shortcuts, "precision-forward"));
  const volumeKeys = both(keyLabel(draft.shortcuts, "volume-up"), keyLabel(draft.shortcuts, "volume-down"));
  return (
    <>
      <Card title={t("ui.settings.playback.open.title")}>
        <div className="grid grid-cols-[minmax(0,1fr)] gap-4 sm:grid-cols-2">
          <Select
            label={t("ui.settings.playback.speed.label")}
            value={String(draft.speed)}
            options={options.speeds.map((speed) => ({ value: String(speed), label: speedLabel(speed) }))}
            onChange={(speed) => change(setDraft, "speed")(Number(speed))}
            help={<HelpTip topic={t("ui.settings.playback.speed.topic")}>{t("ui.settings.playback.speed.help")}</HelpTip>}
          />
          <Select
            label={t("ui.settings.playback.audio.label")}
            value={draft.languages.audio}
            options={options.languages.map((option) => ({ value: option.value, label: option.label }))}
            onChange={(audio) => change(setDraft, "languages")({ ...draft.languages, audio })}
            help={<HelpTip topic={t("ui.settings.playback.audio.topic")}>{t("ui.settings.playback.audio.help")}</HelpTip>}
          />
          <NumberField
            label={t("ui.settings.playback.volumeStep")}
            value={draft.volumeStep}
            min={options.limits.volumeStepMin}
            max={options.limits.volumeStepMax}
            step={1}
            unit="%"
            onChange={(step) => change(setDraft, "volumeStep")(Math.round(step))}
            help={
              <HelpTip topic={t("ui.settings.playback.volumeStep")}>
                {volumeKeys ? t("ui.settings.playback.volumeStepHelpKeys", { keys: volumeKeys }) : t("ui.settings.playback.volumeStepHelp")}
              </HelpTip>
            }
          />
        </div>
      </Card>
      <Card title={t("ui.settings.playback.seek.title")}>
        <NumberField
          label={t("ui.settings.playback.precision.label")}
          value={draft.precisionStep}
          min={options.limits.precisionStepMin}
          max={options.limits.precisionStepMax}
          step={10}
          unit="ms"
          onChange={(step) => change(setDraft, "precisionStep")(Math.round(step))}
          help={
            <HelpTip topic={t("ui.settings.playback.precision.topic")}>
              {precisionKeys
                ? t("ui.settings.playback.precision.helpKeys", { keys: precisionKeys })
                : t("ui.settings.playback.precision.help")}
            </HelpTip>
          }
        />
      </Card>
    </>
  );
}

/** AdvancedSection holds the settings folder, the Windows association, the diagnostics and the reset of the profile. */
export function AdvancedSection({ view, dirty, run }: SectionProps) {
  const [confirming, setConfirming] = useState(false);
  return (
    <>
      <Card title={t("ui.settings.folder.title")}>
        <p className="mb-3 text-sm break-all text-fg-secondary">{view.folder}</p>
        <p className="mb-3 text-xs text-fg-muted">{t("ui.settings.folder.note")}</p>
        <Button icon={FolderOpen} onClick={() => void run(() => backend.openSettingsFolder())}>
          {t("ui.settings.folder.open")}
        </Button>
      </Card>
      <AssociationCard />
      <DiagnosticsCard />
      <Card title={t("ui.settings.reset.title")}>
        <p className="mb-3 text-sm text-fg-secondary">{t("ui.settings.reset.note", { name: view.general.activeProfile })}</p>
        {confirming ? (
          <div className="flex flex-wrap items-center gap-2 rounded-control border border-error/40 bg-error-soft p-3">
            <span className="flex-1 text-sm text-fg">{t("ui.settings.reset.confirm")}</span>
            <Button
              icon={RotateCcw}
              onClick={() => {
                setConfirming(false);
                void run(() => backend.resetProfile(), t("ui.settings.reset.done"));
              }}
            >
              {t("ui.settings.reset.yes")}
            </Button>
            <Button variant="ghost" onClick={() => setConfirming(false)}>
              {t("ui.common.cancel")}
            </Button>
          </div>
        ) : (
          <Button icon={RotateCcw} disabled={dirty} onClick={() => setConfirming(true)}>
            {t("ui.settings.reset.button")}
          </Button>
        )}
      </Card>
    </>
  );
}
