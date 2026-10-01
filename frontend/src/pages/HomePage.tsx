import { Clapperboard, FolderOpen } from "lucide-react";
import { Button } from "../components/ui/Button";
import { Banner } from "../components/ui/Feedback";

interface HomePageProps {
  loading: boolean;
  error: string;
  onChoose: () => void;
  onDismissError: () => void;
}

/** HomePage is the empty state: what to do to start (DESIGN §30). */
export function HomePage({ loading, error, onChoose, onDismissError }: HomePageProps) {
  return (
    <main className="flex flex-1 items-center justify-center p-8">
      <div className="flex w-full max-w-xl flex-col gap-4">
        <div className="flex flex-col items-center gap-5 rounded-card border-2 border-dashed border-line-strong bg-surface px-8 py-14 text-center">
          <span className="flex size-14 items-center justify-center rounded-full bg-accent-soft text-accent">
            <Clapperboard aria-hidden className="size-7" />
          </span>
          {loading ? (
            <div role="status" className="flex flex-col gap-1">
              <p className="text-lg font-semibold text-fg">Lendo o vídeo e as legendas…</p>
              <p className="text-sm text-fg-muted">Em filmes longos isso leva alguns segundos.</p>
            </div>
          ) : (
            <>
              <div className="flex flex-col gap-1">
                <p className="text-lg font-semibold text-fg">Arraste um vídeo para cá</p>
                <p className="text-sm text-fg-muted">ou escolha o arquivo no computador.</p>
              </div>
              <Button variant="primary" size="lg" icon={FolderOpen} onClick={onChoose}>
                Abrir vídeo
              </Button>
              <p className="text-xs text-fg-muted">
                Funciona melhor com .mkv. Também abre .mp4, .avi, .mov e .wmv com legendas em arquivo.
              </p>
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
