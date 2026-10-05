import { Clapperboard, FolderOpen } from "lucide-react";
import { Button } from "../components/ui/Button";
import { Banner } from "../components/ui/Feedback";
import { t } from "../i18n";

interface HomePageProps {
  loading: boolean;
  error: string;
  onChoose: () => void;
  onDismissError: () => void;
}

/** HomePage is the empty state: what to do to start. */
export function HomePage({ loading, error, onChoose, onDismissError }: HomePageProps) {
  return (
    <main className="flex flex-1 items-center justify-center p-8">
      <div className="flex w-full max-w-xl flex-col gap-4">
        <div className="flex flex-col items-center gap-5 rounded-card border-2 border-dashed border-line-strong bg-surface px-8 py-14 text-center">
          <span className="flex size-14 items-center justify-center rounded-control bg-accent-soft text-accent">
            <Clapperboard aria-hidden className="size-7" />
          </span>
          {loading ? (
            <div role="status" className="flex flex-col gap-1">
              <p className="text-lg font-semibold text-fg">{t("ui.home.loading")}</p>
              <p className="text-sm text-fg-muted">{t("ui.home.loadingNote")}</p>
            </div>
          ) : (
            <>
              <div className="flex flex-col gap-1">
                <p className="text-lg font-semibold text-fg">{t("ui.home.drop")}</p>
                <p className="text-sm text-fg-muted">{t("ui.home.orChoose")}</p>
              </div>
              <Button variant="primary" size="lg" icon={FolderOpen} onClick={onChoose}>
                {t("ui.home.open")}
              </Button>
              <p className="text-xs text-fg-muted">{t("ui.home.formats")}</p>
            </>
          )}
        </div>
        {error && (
          <Banner tone="error" onClose={onDismissError}>
            {error}
          </Banner>
        )}
      </div>
    </main>
  );
}
