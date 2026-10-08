import { Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { setLanguage, t } from "../../i18n";
import { backend } from "../../services/backend";
import { Button } from "../ui/Button";
import { Card } from "../ui/Feedback";
import { TextField, Toggle } from "../ui/Field";
import { HelpTip } from "../ui/HelpTip";
import { Select } from "../ui/Select";
import type { SectionProps } from "./section";

type Editing = { kind: "none" } | { kind: "create" | "rename"; name: string } | { kind: "delete" };

/** GeneralSection holds the profiles, the interface's language (with more than one) and the sync notice. */
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
        title={t("ui.settings.profile.title")}
        aside={<HelpTip topic={t("ui.settings.profile.topic")}>{t("ui.settings.profile.help")}</HelpTip>}
      >
        <div className="flex flex-col gap-4">
          <Select
            label={t("ui.settings.profile.active")}
            value={active}
            disabled={dirty || editing.kind !== "none"}
            options={view.profiles.map((name) => ({ value: name, label: name }))}
            onChange={(name) => void run(() => backend.switchProfile(name), t("ui.settings.profile.switched", { name }))}
            note={dirty ? t("ui.settings.profile.saveFirst") : undefined}
          />
          {editing.kind === "none" && (
            <div className="flex flex-wrap gap-2">
              <Button icon={Plus} disabled={dirty} onClick={() => setEditing({ kind: "create", name: "" })}>
                {t("ui.settings.profile.new")}
              </Button>
              <Button icon={Pencil} disabled={dirty} onClick={() => setEditing({ kind: "rename", name: active })}>
                {t("ui.settings.profile.rename")}
              </Button>
              <Button icon={Trash2} disabled={dirty || view.profiles.length < 2} onClick={() => setEditing({ kind: "delete" })}>
                {t("ui.settings.profile.delete")}
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
                  void finish(() => backend.createProfile(name), t("ui.settings.profile.created", { name, from: active }));
                } else {
                  void finish(() => backend.renameProfile(active, name), t("ui.settings.profile.renamed", { name }));
                }
              }}
            >
              <TextField
                label={editing.kind === "create" ? t("ui.settings.profile.newName", { from: active }) : t("ui.settings.profile.renameTo")}
                value={editing.name}
                placeholder={t("ui.settings.profile.placeholder")}
                onChange={(name) => setEditing({ ...editing, name })}
              />
              <div className="flex gap-2">
                <Button type="submit" variant="primary" disabled={editing.name.trim() === ""}>
                  {editing.kind === "create" ? t("ui.settings.profile.create") : t("ui.settings.profile.rename")}
                </Button>
                <Button variant="ghost" onClick={() => setEditing({ kind: "none" })}>
                  {t("ui.common.cancel")}
                </Button>
              </div>
            </form>
          )}
          {editing.kind === "delete" && (
            <div className="flex flex-col gap-3 rounded-control border border-error/40 bg-error-soft p-3">
              <p className="text-sm text-fg">{t("ui.settings.profile.confirmDelete", { name: active })}</p>
              <div className="flex gap-2">
                <Button
                  icon={Trash2}
                  onClick={() => void finish(() => backend.deleteProfile(active), t("ui.settings.profile.deleted", { name: active }))}
                >
                  {t("ui.settings.profile.delete")}
                </Button>
                <Button variant="ghost" onClick={() => setEditing({ kind: "none" })}>
                  {t("ui.common.cancel")}
                </Button>
              </div>
            </div>
          )}
        </div>
      </Card>
      {view.general.languages.length > 1 && (
        <Card title={t("ui.settings.language.title")}>
          <Select
            label={t("ui.settings.language.label")}
            value={view.general.language}
            options={[
              { value: "", label: t("ui.settings.language.windows") },
              ...view.general.languages.map((language) => ({ value: language.value, label: language.label })),
            ]}
            onChange={(language) =>
              void run(
                async () => setLanguage(await backend.setLanguage(language)),
                () => t("ui.settings.language.saved"),
              )
            }
            help={<HelpTip topic={t("ui.settings.language.topic")}>{t("ui.settings.language.help")}</HelpTip>}
          />
        </Card>
      )}
      <Card title={t("ui.settings.window.title")}>
        <Toggle
          label={t("ui.settings.window.onTop")}
          checked={view.general.alwaysOnTop}
          onChange={(on) => void run(() => backend.setAlwaysOnTop(on))}
          help={<HelpTip topic={t("ui.settings.window.topic")}>{t("ui.settings.window.help")}</HelpTip>}
        />
      </Card>
      <Card title={t("ui.settings.notices.title")}>
        <Toggle
          label={t("ui.settings.notices.sync")}
          checked={view.general.syncNotice}
          onChange={(show) => void run(() => backend.setSyncNotice(show))}
          help={<HelpTip topic={t("ui.settings.notices.topic")}>{t("ui.settings.notices.help")}</HelpTip>}
        />
      </Card>
    </>
  );
}
