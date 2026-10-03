import { BookOpenCheck, Bookmark, Flag, LogOut, RotateCcw, Trash2, Undo2 } from "lucide-react";
import { useState } from "react";
import { passageClock } from "../../format";
import { backend } from "../../services/backend";
import type { Playback, SavedPassage } from "../../services/backend";
import { Button, IconButton } from "../ui/Button";
import { Toggle, TextField } from "../ui/Field";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";
import { Select } from "../ui/Select";

interface TrainingCardsProps {
  state: Playback;
  onCommand: (command: () => Promise<void>) => void;
}

const repeatOptions = Array.from({ length: 11 }, (_, times) => ({
  value: String(times),
  label: times === 0 ? "Desligado" : times === 1 ? "1 vez" : `${times} vezes`,
}));

const gapOptions = [0, 1, 2, 3, 5].map((seconds) => ({ value: String(seconds), label: seconds === 0 ? "Sem intervalo" : `${seconds} s` }));

const goalOptions = [0, 3, 5, 10, 20].map((laps) => ({ value: String(laps), label: laps === 0 ? "Sem meta" : `${laps} repetições` }));

/** TrainingCards are the Modo Karaokê and "Estudar trecho" controls. */
export function TrainingCards({ state, onCommand }: TrainingCardsProps) {
  return (
    <div className="grid gap-5 sm:grid-cols-2">
      <KaraokeCard state={state} onCommand={onCommand} />
      <PassageCard state={state} onCommand={onCommand} />
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

function PassageCard({ state, onCommand }: TrainingCardsProps) {
  const { passage } = state;
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [snap, setSnap] = useState(true);
  const [loop, setLoop] = useState(false);
  const [gap, setGap] = useState(0);
  const [progressive, setProgressive] = useState(false);
  const [ladder, setLadder] = useState(false);
  const [goal, setGoal] = useState(0);

  const mark = (set: (value: string) => void) => (
    <Button icon={Flag} onClick={() => set(passageClock(state.position))} title="Marcar a posição atual do vídeo">
      Marcar
    </Button>
  );
  const [name, setName] = useState("");
  const study = () => onCommand(() => backend.studyPassage({ start, end, snap, loop, gap, progressive, ladder, goal }));
  // A saved passage was already fitted to the lines when it was studied.
  const studySaved = (saved: SavedPassage) =>
    onCommand(async () => {
      setStart(saved.start);
      setEnd(saved.end);
      await backend.studyPassage({ start: saved.start, end: saved.end, snap: false, loop, gap, progressive, ladder, goal });
    });
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
          Escolha um trecho do filme. No fim dele o vídeo pausa; Reproduzir ou Espaço volta ao início e toca de novo. Buscar para fora do
          trecho o deixa em pausa até você voltar.
        </HelpTip>
      }
    >
      <div className="flex flex-col gap-4">
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
                label="Nome do trecho"
                value={name}
                onChange={setName}
                placeholder="Discussão na loja"
                inputMode="text"
                action={
                  <Button icon={Bookmark} disabled={name.trim() === ""} onClick={save}>
                    Salvar este trecho
                  </Button>
                }
                help={
                  <HelpTip topic="salvar este trecho">
                    Guarda o trecho com um nome, no arquivo de correções ao lado do vídeo, para estudar de novo outro dia. Um nome que já
                    existe é substituído.
                  </HelpTip>
                }
              />
            </div>
          </div>
        )}
        {state.savedPassages.length > 0 && (
          <div className="flex flex-col gap-2">
            <p className="text-sm font-medium text-fg">Trechos salvos</p>
            <ul className="flex flex-col gap-1">
              {state.savedPassages.map((saved) => (
                <li key={saved.name} className="flex items-center gap-2 rounded-control border border-line px-3 py-1.5 text-sm">
                  <span className="min-w-0 flex-1 truncate text-fg" title={saved.name}>
                    {saved.name}
                  </span>
                  <span className="text-fg-muted tabular-nums">
                    {saved.start}–{saved.end}
                  </span>
                  <Button variant="ghost" icon={BookOpenCheck} onClick={() => studySaved(saved)}>
                    Estudar
                  </Button>
                  <IconButton
                    icon={Trash2}
                    label={`Excluir o trecho "${saved.name}"`}
                    onClick={() => onCommand(() => backend.deletePassage(saved.name))}
                  />
                </li>
              ))}
            </ul>
          </div>
        )}
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
          checked={snap}
          onChange={setSnap}
          help={
            <HelpTip topic="ajustar às falas">
              Estica o trecho para nenhuma frase ficar cortada no meio, com meio segundo de folga antes da primeira fala.
            </HelpTip>
          }
        />
        <Select
          label="No fim do trecho"
          value={loop ? "loop" : "pause"}
          options={[
            { value: "pause", label: "Pausar no fim" },
            { value: "loop", label: "Repetir sem parar" },
          ]}
          onChange={(value) => setLoop(value === "loop")}
          help={
            <HelpTip topic="no fim do trecho">
              Pausar no fim espera você apertar Reproduzir. Repetir sem parar volta sozinho, depois do intervalo escolhido.
            </HelpTip>
          }
        />
        {loop && (
          <Select
            label="Intervalo entre as repetições"
            value={String(gap)}
            options={gapOptions}
            onChange={(value) => setGap(Number(value))}
          />
        )}
        <Toggle
          label="Velocidade progressiva"
          checked={progressive}
          onChange={setProgressive}
          help={
            <HelpTip topic="velocidade progressiva">
              As primeiras repetições tocam mais devagar: 0,75×, depois 0,90× e então 1,00×. Primeiro você entende, depois treina no ritmo
              real.
            </HelpTip>
          }
        />
        <Toggle
          label="Escada de legendas"
          checked={ladder}
          onChange={setLadder}
          help={
            <HelpTip topic="escada de legendas">
              A cada repetição, uma ajuda a menos: primeiro inglês e português, depois só inglês, depois sem legenda, para treinar o ouvido.
            </HelpTip>
          }
        />
        <Select
          label="Meta"
          value={String(goal)}
          options={goalOptions}
          onChange={(value) => setGoal(Number(value))}
          help={<HelpTip topic="meta">Ao completar esse número de repetições, o trecho termina sozinho.</HelpTip>}
        />
        <Button variant="primary" icon={BookOpenCheck} onClick={study}>
          Estudar trecho
        </Button>
      </div>
    </Card>
  );
}
