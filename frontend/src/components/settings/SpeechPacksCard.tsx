import { Download, Trash, X } from "../ui/icons";
import { useState } from "react";
import { levelName } from "../../computer";
import { sizeLabel } from "../../format";
import { useFollowed } from "../../hooks/useFollowed";
import { t } from "../../i18n";
import { backend, messageOf, type SpeechPackView } from "../../services/backend";
import { Button } from "../ui/Button";
import { Banner, Card } from "../ui/Feedback";
import { KeyedTip } from "../ui/HelpTip";
import { ProgressBar } from "../ui/ProgressBar";

/**
 * SpeechPacksCard lets each language's speech pack be fetched into the app's folder, or removed: one at a time, with
 * how far it went, the packs that suit the computer's level marked. Fetching is always the person's choice.
 */
export function SpeechPacksCard() {
  const view = useFollowed(backend.speechPacks, backend.onSpeechPacks);
  const [error, setError] = useState("");

  if (!view?.available) {
    return null;
  }
  const busy = view.packs.some((pack) => pack.fetching);
  const act = (action: () => Promise<void>) => {
    setError("");
    action().catch((failure) => setError(messageOf(failure)));
  };

  return (
    <Card title={t("ui.settings.packs.title")} aside={<KeyedTip k="ui.settings.packs" />}>
      <div className="flex flex-col gap-4">
        <p className="text-sm text-fg-secondary">{t("ui.settings.packs.intro")}</p>
        {view.level === "basic" && <Banner tone="warning">{t("ui.settings.packs.basic")}</Banner>}
        {error && <Banner tone="error">{error}</Banner>}
        <ul className="flex flex-col divide-y divide-line rounded-control border border-line">
          {view.packs.map((pack) => (
            <PackRow
              key={pack.language}
              pack={pack}
              level={view.level}
              busy={busy}
              onFetch={() => act(() => backend.fetchSpeechPack(pack.language))}
              onCancel={() => act(() => backend.cancelSpeechPack())}
              onRemove={() => act(() => backend.removeSpeechPack(pack.language))}
            />
          ))}
        </ul>
      </div>
    </Card>
  );
}

interface PackRowProps {
  pack: SpeechPackView;
  level: string;
  busy: boolean;
  onFetch: () => void;
  onCancel: () => void;
  onRemove: () => void;
}

/** PackRow is one language: its name and size, whether it suits the computer, and what can be done with it now. */
function PackRow({ pack, level, busy, onFetch, onCancel, onRemove }: PackRowProps) {
  return (
    <li className="flex flex-col gap-2 p-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-col">
          <span className="text-sm font-medium text-fg">{pack.name}</span>
          <span className="text-xs text-fg-muted">
            {pack.ready ? t("ui.settings.packs.ready") : sizeLabel(pack.size)}
            {pack.suits && !pack.ready && ` · ${t("ui.settings.packs.suits", { level: levelName(level) })}`}
          </span>
        </div>
        {pack.fetching ? (
          <Button variant="ghost" icon={X} onClick={onCancel}>
            {t("ui.common.cancel")}
          </Button>
        ) : pack.ready ? (
          <Button variant="ghost" icon={Trash} disabled={busy} onClick={onRemove}>
            {t("ui.settings.packs.remove")}
          </Button>
        ) : (
          <Button icon={Download} disabled={busy} onClick={onFetch}>
            {t("ui.settings.packs.fetch")}
          </Button>
        )}
      </div>
      {pack.fetching && (
        <ProgressBar label={t("ui.settings.packs.fetching")} percent={pack.total > 0 ? (pack.done / pack.total) * 100 : 0} />
      )}
      {pack.error && !pack.fetching && <p className="text-xs text-error">{pack.error}</p>}
    </li>
  );
}
