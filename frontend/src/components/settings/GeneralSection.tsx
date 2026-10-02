import { Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { backend } from "../../services/backend";
import { Button } from "../ui/Button";
import { Card } from "../ui/Feedback";
import { TextField, Toggle } from "../ui/Field";
import { HelpTip } from "../ui/HelpTip";
import { Select } from "../ui/Select";
import type { SectionProps } from "./section";

type Editing = { kind: "none" } | { kind: "create" | "rename"; name: string } | { kind: "delete" };

/** GeneralSection holds the profiles and the sync notice. */
export function GeneralSection({ view, dirty, run }: SectionProps) {
  const [editing, setEditing] = useState<Editing>({ kind: "none" });
  const active = view.general.activeProfile;
  const finish = async (action: () => Promise<unknown>, done: string) => {
    await run(action, done);
    setEditing({ kind: "none" });
  };

  return (
    <>
      <Card
        title="Perfil"
        aside={
          <HelpTip topic="Perfil">
            Um perfil guarda as suas escolhas para um idioma: legendas, cores, atalhos, Modo Karaokê e player. Crie um para cada idioma que
            você estuda, como “Inglês” e “Francês”.
          </HelpTip>
        }
      >
        <div className="flex flex-col gap-4">
          <Select
            label="Perfil em uso"
            value={active}
            disabled={dirty || editing.kind !== "none"}
            options={view.profiles.map((name) => ({ value: name, label: name }))}
            onChange={(name) => void run(() => backend.switchProfile(name), `Perfil “${name}” em uso.`)}
            note={dirty ? "Salve ou descarte as alterações antes de trocar de perfil." : undefined}
          />
          {editing.kind === "none" && (
            <div className="flex flex-wrap gap-2">
              <Button icon={Plus} disabled={dirty} onClick={() => setEditing({ kind: "create", name: "" })}>
                Novo perfil
              </Button>
              <Button icon={Pencil} disabled={dirty} onClick={() => setEditing({ kind: "rename", name: active })}>
                Renomear
              </Button>
              <Button icon={Trash2} disabled={dirty || view.profiles.length < 2} onClick={() => setEditing({ kind: "delete" })}>
                Excluir
              </Button>
            </div>
          )}
          {(editing.kind === "create" || editing.kind === "rename") && (
            <form
              className="flex flex-col gap-3"
              onSubmit={(event) => {
                event.preventDefault();
                const name = editing.name;
                if (editing.kind === "create") {
                  void finish(() => backend.createProfile(name), `Perfil “${name}” criado como cópia de “${active}”.`);
                } else {
                  void finish(() => backend.renameProfile(active, name), `Perfil renomeado para “${name}”.`);
                }
              }}
            >
              <TextField
                label={editing.kind === "create" ? `Nome do novo perfil (cópia de “${active}”)` : "Novo nome"}
                value={editing.name}
                placeholder="Francês"
                onChange={(name) => setEditing({ ...editing, name })}
              />
              <div className="flex gap-2">
                <Button type="submit" variant="primary" disabled={editing.name.trim() === ""}>
                  {editing.kind === "create" ? "Criar" : "Renomear"}
                </Button>
                <Button variant="ghost" onClick={() => setEditing({ kind: "none" })}>
                  Cancelar
                </Button>
              </div>
            </form>
          )}
          {editing.kind === "delete" && (
            <div className="flex flex-col gap-3 rounded-control border border-error/40 bg-error-soft p-3">
              <p className="text-sm text-fg">Excluir o perfil “{active}”? As escolhas dele não poderão ser recuperadas.</p>
              <div className="flex gap-2">
                <Button icon={Trash2} onClick={() => void finish(() => backend.deleteProfile(active), `Perfil “${active}” excluído.`)}>
                  Excluir
                </Button>
                <Button variant="ghost" onClick={() => setEditing({ kind: "none" })}>
                  Cancelar
                </Button>
              </div>
            </div>
          )}
        </div>
      </Card>
      <Card title="Avisos no vídeo">
        <Toggle
          label="Mostrar no vídeo qual legenda a sincronização mudou"
          checked={view.general.syncNotice}
          onChange={(show) => void run(() => backend.setSyncNotice(show))}
          help={
            <HelpTip topic="Aviso de sincronização">
              Ao adiantar ou atrasar uma legenda pelos atalhos, um aviso pequeno aparece no canto do vídeo por um instante, como “Legenda de
              cima (inglês): −0,150 s”. Vale para todos os perfis.
            </HelpTip>
          }
        />
      </Card>
    </>
  );
}
