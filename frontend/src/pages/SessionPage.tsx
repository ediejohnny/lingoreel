import { Eraser, Play, RotateCcw } from "lucide-react";
import { useEffect, useState } from "react";
import { SavedPassages } from "../components/player/SavedPassages";
import { none, noteOf, SubtitlePicker, trackOptions } from "../components/player/SubtitlePicker";
import { InstallCards } from "../components/session/InstallCards";
import { Button } from "../components/ui/Button";
import { Banner, Card } from "../components/ui/Feedback";
import { HelpTip } from "../components/ui/HelpTip";
import { Segmented } from "../components/ui/Inputs";
import { Rich } from "../components/ui/Rich";
import { Select, type Option } from "../components/ui/Select";
import { t } from "../i18n";
import { keyLabel } from "../keys";
import { backend, messageOf } from "../services/backend";
import type { Line, PlayerInstall, Players, SavedPassage, ShortcutSetting, Video, WatchRequest } from "../services/backend";

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
  /** onVideo takes the video described again, with a subtitle file added to it. */
  onVideo: (video: Video) => void;
  /** onReset forgets everything kept about the video and opens it as new. */
  onReset: () => Promise<void>;
  onDismissError: () => void;
  onInstall: (id: string) => void;
  onDownload: (id: string) => void;
  onDismissRecommendation: (id: string) => void;
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
  // continueFrom goes on where the video was left; on by default when offered.
  const [continueFrom, setContinueFrom] = useState(video.resume.offered);
  const [passages, setPassages] = useState<SavedPassage[]>(video.savedPassages);
  const [keys, setKeys] = useState<ShortcutSetting[]>([]);
  const [resetting, setResetting] = useState(false);
  const [listError, setListError] = useState("");
  const [adding, setAdding] = useState<Line | "">("");
  const [addError, setAddError] = useState("");

  useEffect(() => {
    let current = true;
    backend
      .shortcuts()
      .then((list) => current && setKeys(list))
      .catch(() => current && setKeys([]));
    return () => {
      current = false;
    };
  }, []);

  const startKey = keyLabel(keys, "passage-mark-start");
  const endKey = keyLabel(keys, "passage-mark-end");

  const supported = players.installed.filter((item) => item.supported);
  const playerOptions: Option[] = players.installed.map((item) => ({
    value: item.id,
    label: item.supported ? item.name : t("ui.session.player.soon", { name: item.name }),
    disabled: !item.supported,
  }));
  const canWatch = supported.some((item) => item.id === player) && (primary !== none || translation !== none);
  // A player that keeps its own audio track: the choice is hidden, never sent.
  const chosen = players.installed.find((item) => item.id === player);
  const fixedAudio = chosen?.fixedAudio ?? false;
  const chosenName = chosen?.name ?? t("ui.session.player.fallback");

  // A lost player's place, or where the video was left when asked; a saved
  // passage opens the video at its own start.
  const watch = (resume = false, passage = "") =>
    props.onWatch({
      audio: fixedAudio ? 0 : Number(audio),
      primary: Number(primary),
      translation: Number(translation),
      player,
      alwaysUse,
      mode,
      resume: resume || (video.resume.offered && continueFrom),
      passage,
    });

  // A file from any folder joins the video and goes on the line it was added for.
  const addFile = async (line: Line) => {
    setAdding(line);
    try {
      const added = await backend.addSubtitleFile(line);
      if (added.path === "") {
        return;
      }
      props.onVideo(added);
      if (line === "primary") {
        setPrimary(String(added.suggested.primary));
      } else {
        setTranslation(String(added.suggested.translation));
      }
      setAddError("");
    } catch (failure) {
      setAddError(messageOf(failure));
    } finally {
      setAdding("");
    }
  };

  const refresh = async () => setPassages(await backend.savedPassages());
  const listCommand = async (run: () => Promise<void>) => {
    try {
      await run();
      setListError("");
    } catch (failure) {
      setListError(messageOf(failure));
    }
    await refresh();
  };

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
                  {t("ui.session.watchAgain")}
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
          title={t("ui.session.subtitles.title")}
          aside={<HelpTip topic={t("ui.session.subtitles.topic")}>{t("ui.session.subtitles.help")}</HelpTip>}
        >
          {addError && (
            <div className="mb-4">
              <Banner tone="error" onClose={() => setAddError("")}>
                {addError}
              </Banner>
            </div>
          )}
          <div className="grid gap-5 sm:grid-cols-2">
            <SubtitlePicker
              label={t("ui.session.subtitles.primary")}
              value={primary}
              options={trackOptions(video.subtitles, t("ui.session.subtitles.none"))}
              onChange={setPrimary}
              onAdd={() => void addFile("primary")}
              adding={adding === "primary"}
              {...noteOf(video.subtitles, primary)}
            />
            <SubtitlePicker
              label={t("ui.session.subtitles.translation")}
              value={translation}
              options={trackOptions(video.subtitles, t("ui.session.subtitles.noTranslation"))}
              onChange={setTranslation}
              onAdd={() => void addFile("translation")}
              adding={adding === "translation"}
              {...noteOf(video.subtitles, translation)}
            />
          </div>
          {video.subtitles.length === 0 && <p className="mt-4 text-sm text-fg-muted">{t("ui.session.subtitles.missing")}</p>}
          {video.modes.length > 0 && (
            <div className="mt-5">
              <Segmented
                label={t("ui.session.mode.label")}
                value={mode}
                options={video.modes.map((option) => ({ value: option.value, label: option.label }))}
                onChange={setMode}
                help={<HelpTip topic={t("ui.session.mode.topic")}>{t("ui.session.mode.help")}</HelpTip>}
              />
            </div>
          )}
        </Card>

        <Card title={t("ui.session.audio.title")}>
          <Select
            label={t("ui.session.audio.label")}
            value={fixedAudio ? none : audio}
            options={[
              { value: none, label: t("ui.session.audio.player") },
              ...video.audio.map((track) => ({ value: String(track.number), label: track.label })),
            ]}
            onChange={setAudio}
            disabled={fixedAudio}
            note={fixedAudio ? t("ui.session.audio.fixed", { name: chosenName }) : undefined}
          />
        </Card>

        <Card
          title={t("ui.session.passages.title")}
          aside={
            <HelpTip topic={t("ui.session.passages.topic")}>
              {t("ui.session.passages.help", {
                start: startKey || t("ui.session.passages.startNone"),
                end: endKey || t("ui.session.passages.endNone"),
              })}
            </HelpTip>
          }
        >
          {listError && (
            <Banner tone="error" onClose={() => setListError("")}>
              {listError}
            </Banner>
          )}
          {passages.length === 0 ? (
            <p className="text-sm text-fg-muted">
              <Rich
                text={t("ui.session.passages.empty")}
                nodes={{
                  start: startKey ? <kbd>{startKey}</kbd> : t("ui.session.passages.startNone"),
                  end: endKey ? <kbd>{endKey}</kbd> : t("ui.session.passages.endNone"),
                }}
              />
            </p>
          ) : (
            <SavedPassages
              passages={passages}
              studyLabel={t("ui.session.passages.study")}
              studyDisabled={canWatch ? undefined : t("ui.session.passages.blocked")}
              onStudy={(name) => watch(false, name)}
              onRename={async (from, to) => {
                await backend.renamePassage(from, to);
                await refresh();
              }}
              onDelete={(name) => listCommand(() => backend.deletePassage(name))}
            />
          )}
        </Card>

        <Card title={t("ui.session.player.title")}>
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
              {players.preferredMissing && <Banner tone="info">{t("ui.session.player.missing")}</Banner>}
              <Select label={t("ui.session.player.label")} value={player} options={playerOptions} onChange={setPlayer} />
              <label className="flex w-fit items-center gap-2 text-sm text-fg">
                <input
                  type="checkbox"
                  checked={alwaysUse}
                  onChange={(event) => setAlwaysUse(event.target.checked)}
                  className="size-4 accent-[var(--sp-accent)]"
                />
                {t("ui.session.player.always")}
                <HelpTip topic={t("ui.session.player.alwaysTopic")}>{t("ui.session.player.alwaysHelp")}</HelpTip>
              </label>
            </div>
          )}
        </Card>

        <Card
          title={t("ui.session.forget.title")}
          aside={<HelpTip topic={t("ui.session.forget.topic")}>{t("ui.session.forget.help")}</HelpTip>}
        >
          {resetting ? (
            <div className="flex flex-wrap items-center gap-2 text-sm text-fg">
              <span>{t("ui.session.forget.confirm")}</span>
              <Button
                variant="secondary"
                icon={Eraser}
                onClick={() =>
                  void props.onReset().finally(() => {
                    setResetting(false);
                  })
                }
              >
                {t("ui.session.forget.yes")}
              </Button>
              <Button variant="ghost" onClick={() => setResetting(false)}>
                {t("ui.common.no")}
              </Button>
            </div>
          ) : (
            <Button variant="ghost" icon={Eraser} onClick={() => setResetting(true)}>
              {t("ui.session.forget.button")}
            </Button>
          )}
        </Card>

        {/* Always in view, however tall the window's content. */}
        <div className="sticky bottom-0 -mx-6 flex flex-wrap items-center justify-end gap-4 border-t border-line bg-bg/95 px-6 py-3 backdrop-blur">
          {video.resume.offered && (
            <label className="flex items-center gap-2 text-sm text-fg">
              <input
                type="checkbox"
                checked={continueFrom}
                onChange={(event) => setContinueFrom(event.target.checked)}
                className="size-4 accent-[var(--sp-accent)]"
              />
              {t("ui.session.continueFrom", { time: video.resume.text })}
            </label>
          )}
          <Button variant="primary" size="lg" icon={Play} disabled={!canWatch} loading={props.starting} onClick={() => watch()}>
            {props.starting ? t("ui.session.starting") : t("ui.session.watch")}
          </Button>
        </div>
      </div>
    </main>
  );
}
