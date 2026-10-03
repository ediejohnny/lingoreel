import { Play, RotateCcw } from "lucide-react";
import { useState } from "react";
import { InstallCards } from "../components/session/InstallCards";
import { Button } from "../components/ui/Button";
import { Banner, Card } from "../components/ui/Feedback";
import { HelpTip } from "../components/ui/HelpTip";
import { Segmented } from "../components/ui/Inputs";
import { Select, type Option } from "../components/ui/Select";
import type { PlayerInstall, Players, Track, Video, WatchRequest } from "../services/backend";

const none = "0";

interface SessionPageProps {
  video: Video;
  players: Players;
  starting: boolean;
  error: string;
  /** resumable offers to watch again from where the lost player was. */
  resumable: boolean;
  installing: string;
  installError: string;
  onWatch: (request: WatchRequest) => void;
  onDismissError: () => void;
  onInstall: (id: string) => void;
  onDownload: (id: string) => void;
  onDismissRecommendation: (id: string) => void;
}

function trackOptions(tracks: Track[], emptyLabel: string): Option[] {
  return [
    { value: none, label: emptyLabel },
    ...tracks.map((track) => ({ value: String(track.number), label: track.label, disabled: !track.readable })),
  ];
}

function noteOf(tracks: Track[], value: string): { note?: string; warning: boolean } {
  const track = tracks.find((candidate) => String(candidate.number) === value);
  return { note: track?.note || undefined, warning: track?.warning ?? false };
}

/** SessionPage chooses what to watch with: subtitles, translation, audio and player. */
export function SessionPage(props: SessionPageProps) {
  const { video, players } = props;
  const [primary, setPrimary] = useState(String(video.suggested.primary));
  const [translation, setTranslation] = useState(String(video.suggested.translation));
  const [audio, setAudio] = useState(String(video.suggested.audio));
  const [player, setPlayer] = useState(players.choice);
  const [alwaysUse, setAlwaysUse] = useState(false);
  const [mode, setMode] = useState(video.mode);

  const supported = players.installed.filter((item) => item.supported);
  const playerOptions: Option[] = players.installed.map((item) => ({
    value: item.id,
    label: item.supported ? item.name : `${item.name} (suporte em breve)`,
    disabled: !item.supported,
  }));
  const canWatch = supported.some((item) => item.id === player) && (primary !== none || translation !== none);
  // A player that keeps its own audio track: the choice is hidden, never sent.
  const chosen = players.installed.find((item) => item.id === player);
  const fixedAudio = chosen?.fixedAudio ?? false;
  const chosenName = chosen?.name ?? "player";

  const watch = (resume = false) =>
    props.onWatch({
      audio: fixedAudio ? 0 : Number(audio),
      primary: Number(primary),
      translation: Number(translation),
      player,
      alwaysUse,
      mode,
      resume,
    });

  return (
    <main className="flex flex-1 justify-center overflow-y-auto px-6 pt-6">
      <div className="flex w-full max-w-3xl flex-col gap-5">
        {props.error && (
          <Banner
            tone="error"
            onClose={props.onDismissError}
            action={
              props.resumable && canWatch ? (
                <Button icon={RotateCcw} loading={props.starting} onClick={() => watch(true)}>
                  Assistir de novo
                </Button>
              ) : undefined
            }
          >
            {props.error}
          </Banner>
        )}
        {video.projectNote && <Banner tone="warning">{video.projectNote}</Banner>}
        {video.languageNote && <Banner tone="info">{video.languageNote}</Banner>}
        <Card
          title="Legendas"
          aside={
            <HelpTip topic="legendas">
              A legenda de cima é a do idioma que você estuda; a de baixo é a tradução. Legendas baixadas na pasta do vídeo ou em subpastas
              aparecem aqui, e o tempo delas é conferido com o vídeo.
            </HelpTip>
          }
        >
          <div className="grid gap-5 sm:grid-cols-2">
            <Select
              label="Legenda (idioma estudado)"
              value={primary}
              options={trackOptions(video.subtitles, "Sem legenda")}
              onChange={setPrimary}
              {...noteOf(video.subtitles, primary)}
            />
            <Select
              label="Tradução"
              value={translation}
              options={trackOptions(video.subtitles, "Sem tradução")}
              onChange={setTranslation}
              {...noteOf(video.subtitles, translation)}
            />
          </div>
          {video.subtitles.length === 0 && (
            <p className="mt-4 text-sm text-fg-muted">
              Este vídeo não tem legendas. Baixe uma legenda (.srt) e coloque na mesma pasta do vídeo.
            </p>
          )}
          {video.modes.length > 0 && (
            <div className="mt-5">
              <Segmented
                label="Modo"
                value={mode}
                options={video.modes.map((option) => ({ value: option.value, label: option.label }))}
                onChange={setMode}
                help={
                  <HelpTip topic="modo de estudo">
                    Dupla mostra as duas legendas o tempo todo. Tradução sob demanda, o modo recomendado para estudar, mostra só a legenda
                    de cima: a tecla da tradução mostra a de baixo quando precisar. Tradução sempre visível nunca esconde a de baixo. O
                    padrão de cada perfil fica nas Configurações, com a tecla.
                  </HelpTip>
                }
              />
            </div>
          )}
        </Card>

        <Card title="Áudio">
          <Select
            label="Faixa de áudio"
            value={fixedAudio ? none : audio}
            options={[
              { value: none, label: "Escolha do player" },
              ...video.audio.map((track) => ({ value: String(track.number), label: track.label })),
            ]}
            onChange={setAudio}
            disabled={fixedAudio}
            note={fixedAudio ? `O ${chosenName} não aceita trocar a faixa de áudio pelo app; escolha no menu dele.` : undefined}
          />
        </Card>

        <Card title="Player">
          {players.choice === "" ? (
            <InstallCards
              recommendations={players.recommendations as PlayerInstall[]}
              installing={props.installing}
              error={props.installError}
              onInstall={props.onInstall}
              onDownload={props.onDownload}
              onDismiss={props.onDismissRecommendation}
            />
          ) : (
            <div className="flex flex-col gap-3">
              {players.preferredMissing && (
                <Banner tone="info">O seu player preferido não está mais instalado; foi escolhido outro.</Banner>
              )}
              <Select label="Abrir com" value={player} options={playerOptions} onChange={setPlayer} />
              <label className="flex w-fit items-center gap-2 text-sm text-fg">
                <input
                  type="checkbox"
                  checked={alwaysUse}
                  onChange={(event) => setAlwaysUse(event.target.checked)}
                  className="size-4 accent-[var(--sp-accent)]"
                />
                Usar sempre este player
                <HelpTip topic="usar sempre este player">
                  O LingoReel passa a abrir os vídeos sempre com este player, enquanto ele estiver instalado.
                </HelpTip>
              </label>
            </div>
          )}
        </Card>

        {/* Always in view, however tall the window's content. */}
        <div className="sticky bottom-0 -mx-6 flex justify-end border-t border-line bg-bg/95 px-6 py-3 backdrop-blur">
          <Button variant="primary" size="lg" icon={Play} disabled={!canWatch} loading={props.starting} onClick={() => watch()}>
            {props.starting ? "Iniciando o player…" : "Assistir"}
          </Button>
        </div>
      </div>
    </main>
  );
}
