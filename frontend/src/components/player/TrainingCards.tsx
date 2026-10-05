import { BookOpenCheck, Bookmark, Flag, LogOut, RotateCcw, Undo2 } from "lucide-react";
import { useState } from "react";
import { passageClock } from "../../format";
import { withKey } from "../../keys";
import { backend } from "../../services/backend";
import type { Playback } from "../../services/backend";
import { Button } from "../ui/Button";
import { Toggle, TextField } from "../ui/Field";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import { Select } from "../ui/Select";
import { SavedPassages } from "./SavedPassages";

interface TrainingCardsProps {
  state: Playback;
  /** label is the key of a command, such as "Ctrl+F9" for "passage-mark-start". */
  label: (command: string) => string;
  onCommand: (command: () => Promise<void>) => void;
}

const repeatOptions = Array.from({ length: 11 }, (_, times) => ({
  value: String(times),
  label: times === 0 ? "Desligado" : times === 1 ? "1 vez" : `${times} vezes`,
}));

const gapOptions = [0, 1, 2, 3, 5].map((seconds) => ({ value: String(seconds), label: seconds === 0 ? "Sem intervalo" : `${seconds} s` }));

const goalOptions = [0, 3, 5, 10, 20].map((laps) => ({ value: String(laps), label: laps === 0 ? "Sem meta" : `${laps} repetições` }));

/** TrainingCards are the Modo Karaokê and "Estudar trecho" controls. */
export function TrainingCards({ state, label, onCommand }: TrainingCardsProps) {
  return (
    <div className="@container">
      <div className="grid gap-5 [&>*]:min-w-0 @2xl:grid-cols-2">
        <KaraokeCard state={state} label={label} onCommand={onCommand} />
        <PassageCard state={state} label={label} onCommand={onCommand} />
      </div>
    </div>
  );
}

function KaraokeCard({ state, onCommand }: TrainingCardsProps) {
  const { karaoke } = state;
  return (
    <Card
      title="Modo Karaokê"
      aside={
        <HelpTip topic="Modo Karaokê">
          Mostra a próxima fala antes da hora, para você se preparar e falar junto com o ator. A prévia aparece 5 s antes, só nas pausas
          maiores que 10 s. Durante a fala, o texto é pintado como num karaokê; esse preenchimento é uma estimativa, porque a legenda não
          traz o tempo de cada palavra.
        </HelpTip>
      }
    >
      <div className="flex flex-col gap-4">
        <Toggle label="Ligar o Modo Karaokê" checked={karaoke.enabled} onChange={(on) => onCommand(() => backend.setKaraoke(on))} />
        <Select
          label="Tempo até a fala"
          value={karaoke.indicator}
          options={[
            { value: "bar", label: "Barra" },
            { value: "countdown", label: "Contagem regressiva" },
          ]}
          onChange={(value) => onCommand(() => backend.setKaraokeIndicator(value))}
          help={
            <HelpTip topic="tempo até a fala">
              A barra, na largura do texto, diminui até a fala começar. A contagem mostra os segundos que faltam, em tempo real (a 0,75× ela
              dura mais).
            </HelpTip>
          }
        />
        <Select
          label="Repetir cada fala"
          value={String(karaoke.repeat)}
          options={repeatOptions}
          onChange={(value) => onCommand(() => backend.setLineRepeats(Number(value)))}
          help={
            <HelpTip topic="repetir cada fala">
              Quando uma fala termina, o vídeo volta para a prévia dela e repete, quantas vezes você escolher. Buscar outro ponto do vídeo
              começa a contagem de novo.
            </HelpTip>
          }
        />
        <div className="flex items-center gap-1">
          <Button icon={RotateCcw} onClick={() => onCommand(backend.replayLine)} title="Treinar de novo (Backspace na janela do filme)">
            Treinar de novo
          </Button>
          <HelpTip topic="treinar de novo">
            Volta para o início da prévia da fala atual (ou da última), para treinar mais uma vez. Na janela do filme, a tecla é Backspace.
          </HelpTip>
        </div>
      </div>
    </Card>
  );
}

/** PassageOptionsState is how a passage is studied, as the card edits it. */
interface PassageOptionsState {
  snap: boolean;
  loop: boolean;
  gap: number;
  progressive: boolean;
  ladder: boolean;
  goal: number;
}

function PassageCard({ state, label, onCommand }: TrainingCardsProps) {
  const { passage } = state;
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [name, setName] = useState("");
  // The options start as the session has them: the ones the end key uses.
  const [options, setOptions] = useState<PassageOptionsState>({ snap: state.passageOptions.snap, ...state.passageOptions.plan });
  const startKey = label("passage-mark-start");
  const endKey = label("passage-mark-end");

  // A changed option is also the next marked passage's.
  const change = (patch: Partial<PassageOptionsState>) => {
    const next = { ...options, ...patch };
    setOptions(next);
    const { snap, ...plan } = next;
    onCommand(() => backend.setPassageOptions({ snap, plan }));
  };
  const mark = (set: (value: string) => void) => (
    <Button icon={Flag} onClick={() => set(passageClock(state.position))} title="Marcar a posição atual do vídeo">
      Marcar
    </Button>
  );
  const study = () => onCommand(() => backend.studyPassage({ start, end, ...options }));
  const save = () =>
    onCommand(async () => {
      await backend.savePassage(name);
      setName("");
    });
  const thisLine = () =>
    onCommand(async () => {
      const times = await backend.passageOfLine();
      setStart(times.start);
      setEnd(times.end);
    });

  return (
    <Card
      title="Estudar trecho"
      aside={
        <HelpTip topic="estudar trecho">
          Marque o início e o fim de um trecho do filme: ele é salvo e começa a repetir. No fim do trecho o vídeo pausa; Reproduzir ou
          Espaço volta ao início e toca de novo. Buscar para fora do trecho o deixa em pausa até você voltar.
        </HelpTip>
      }
    >
      <div className="flex flex-col gap-4">
        <p className="text-sm text-fg-muted">
          {startKey ? <kbd>{startKey}</kbd> : "O atalho de início"} marca o começo e {endKey ? <kbd>{endKey}</kbd> : "o de fim"} marca o
          fim: o trecho é salvo e começa a repetir. Os atalhos valem na janela do filme, aqui e com o player em primeiro plano.
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <Button icon={Flag} onClick={() => onCommand(() => backend.runShortcut("passage-mark-start", false))}>
            {withKey("Marcar início", startKey)}
          </Button>
          <Button
            icon={Bookmark}
            disabled={state.markedStart === ""}
            title={state.markedStart === "" ? "Marque o início primeiro" : undefined}
            onClick={() => onCommand(() => backend.runShortcut("passage-mark-end", false))}
          >
            {withKey("Marcar fim e salvar", endKey)}
          </Button>
          {state.markedStart && (
            <span role="status" className="text-sm text-fg">
              Início marcado: {state.markedStart}
            </span>
          )}
        </div>

        {passage.active && (
          <div className="rounded-control border border-line bg-surface-sunken p-3 text-sm text-fg" role="status">
            <p>
              Trecho {passage.start}–{passage.end} · repetição {passage.lap}
              {passage.goal > 0 ? ` de ${passage.goal}` : ""}
            </p>
            {passage.suspended && <p className="mt-1 text-warning">Trecho pausado: o vídeo saiu do intervalo.</p>}
            {passage.atEnd && <p className="mt-1 text-fg-muted">Fim do trecho: Espaço ou Reproduzir repete.</p>}
            <div className="mt-3 flex flex-wrap gap-2">
              {passage.suspended && (
                <Button icon={Undo2} onClick={() => onCommand(backend.resumePassage)}>
                  Voltar ao trecho
                </Button>
              )}
              <Button icon={LogOut} onClick={() => onCommand(backend.stopPassage)}>
                Sair do trecho
              </Button>
            </div>
            <div className="mt-3 border-t border-line pt-3">
              <TextField
                label="Salvar com outro nome"
                value={name}
                onChange={setName}
                placeholder={`Trecho ${passage.start}–${passage.end}`}
                inputMode="text"
                action={
                  <Button icon={Bookmark} onClick={save}>
                    Salvar este trecho
                  </Button>
                }
                help={
                  <HelpTip topic="salvar este trecho">
                    Guarda o trecho no arquivo ao lado do vídeo, com as opções em uso, para estudar de novo outro dia. Sem nome, ele leva os
                    tempos no nome; um nome que já existe é substituído. Um trecho marcado pelo atalho de fim já foi salvo.
                  </HelpTip>
                }
              />
            </div>
          </div>
        )}

        {state.savedPassages.length > 0 && (
          <div className="flex flex-col gap-2">
            <p className="text-sm font-medium text-fg">Trechos salvos</p>
            <SavedPassages
              passages={state.savedPassages}
              studyLabel="Estudar"
              onStudy={(saved) => onCommand(() => backend.studySavedPassage(saved))}
              onRename={(from, to) => backend.renamePassage(from, to)}
              onDelete={async (saved) => onCommand(() => backend.deletePassage(saved))}
            />
          </div>
        )}

        <details className="group rounded-control border border-line px-3 py-2">
          <summary className="cursor-pointer text-sm font-medium text-fg">Escolher pelos tempos e mais opções</summary>
          <div className="mt-3 flex flex-col gap-4">
            <div className="grid grid-cols-2 gap-3">
              <TextField label="Início" value={start} onChange={setStart} placeholder="1:10:23" action={mark(setStart)} />
              <TextField label="Fim" value={end} onChange={setEnd} placeholder="1:11:04" action={mark(setEnd)} />
            </div>
            <div className="flex items-center gap-1">
              <Button onClick={thisLine}>Esta fala</Button>
              <HelpTip topic="esta fala">Preenche o início e o fim com a fala de cima que está na tela (ou a última que apareceu).</HelpTip>
            </div>
            <Toggle
              label="Ajustar às falas"
              checked={options.snap}
              onChange={(snap) => change({ snap })}
              help={
                <HelpTip topic="ajustar às falas">
                  Estica o trecho para nenhuma frase ficar cortada no meio, com meio segundo de folga antes da primeira fala. Vale também
                  para os trechos marcados pelos atalhos.
                </HelpTip>
              }
            />
            <Select
              label="No fim do trecho"
              value={options.loop ? "loop" : "pause"}
              options={[
                { value: "pause", label: "Pausar no fim" },
                { value: "loop", label: "Repetir sem parar" },
              ]}
              onChange={(value) => change({ loop: value === "loop" })}
              help={
                <HelpTip topic="no fim do trecho">
                  Pausar no fim espera você apertar Reproduzir. Repetir sem parar volta sozinho, depois do intervalo escolhido.
                </HelpTip>
              }
            />
            {options.loop && (
              <Select
                label="Intervalo entre as repetições"
                value={String(options.gap)}
                options={gapOptions}
                onChange={(value) => change({ gap: Number(value) })}
              />
            )}
            <Toggle
              label="Velocidade progressiva"
              checked={options.progressive}
              onChange={(progressive) => change({ progressive })}
              help={
                <HelpTip topic="velocidade progressiva">
                  As primeiras repetições tocam mais devagar: 0,75×, depois 0,90× e então 1,00×. Primeiro você entende, depois treina no
                  ritmo real.
                </HelpTip>
              }
            />
            <Toggle
              label="Escada de legendas"
              checked={options.ladder}
              onChange={(ladder) => change({ ladder })}
              help={
                <HelpTip topic="escada de legendas">
                  A cada repetição, uma ajuda a menos: primeiro inglês e português, depois só inglês, depois sem legenda, para treinar o
                  ouvido.
                </HelpTip>
              }
            />
            <Select
              label="Meta"
              value={String(options.goal)}
              options={goalOptions}
              onChange={(value) => change({ goal: Number(value) })}
              help={<HelpTip topic="meta">Ao completar esse número de repetições, o trecho termina sozinho.</HelpTip>}
            />
            <Button variant="primary" icon={BookOpenCheck} onClick={study}>
              Estudar trecho
            </Button>
          </div>
        </details>
      </div>
    </Card>
  );
}
