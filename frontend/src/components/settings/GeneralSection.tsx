import { Pencil, Plus, Trash, Wand } from "../ui/icons";
import { useState } from "react";
import { setLanguage, t } from "../../i18n";
import { backend } from "../../services/backend";
import { Button } from "../ui/Button";
import { Card } from "../ui/Feedback";
import { TextField, Toggle } from "../ui/Field";
import { HelpTip, KeyedTip } from "../ui/HelpTip";
import { Select } from "../ui/Select";
import { ComputerCard } from "./ComputerCard";
import type { SectionProps } from "./section";

/** ProfileEdit is what is being done to the profiles: nothing, naming a new or the active one, or deleting it. */
type ProfileEdit = { kind: "none" } | { kind: "create" | "rename"; name: string } | { kind: "delete" };

/**
 * GeneralSection holds the profiles, the app language (with more than one), the welcome, the computer, the window and the
 * sync notice.
 */
export function GeneralSection({ view, dirty, run, welcome }: SectionProps) {
  const [editing, setEditing] = useState<ProfileEdit>({ kind: "none" });
  const active = view.general.activeProfile;
  const finish = async (action: () => Promise<unknown>, done: string) => {
    await run(action, done);
    setEditing({ kind: "none" });
  };

  return (
    <>
      <Card title={t("ui.settings.profile.title")} aside={<KeyedTip k="ui.settings.profile" />}>
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
              <Button icon={Trash} disabled={dirty || view.profiles.length < 2} onClick={() => setEditing({ kind: "delete" })}>
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
                  icon={Trash}
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
            help={<KeyedTip k="ui.settings.language" />}
          />
        </Card>
      )}
      <Card
        title={t("ui.settings.welcome.title")}
        aside={<HelpTip topic={t("ui.settings.welcome.title")}>{t("ui.settings.welcome.help")}</HelpTip>}
      >
        <div className="flex flex-col gap-2">
          <div>
            <Button icon={Wand} disabled={!welcome || dirty} onClick={welcome}>
              {t("ui.settings.welcome.open")}
            </Button>
          </div>
          {!welcome ? (
            <p className="text-xs text-fg-muted">{t("ui.settings.welcome.watching")}</p>
          ) : (
            dirty && <p className="text-xs text-fg-muted">{t("ui.settings.profile.saveFirst")}</p>
          )}
        </div>
      </Card>
      <ComputerCard view={view} dirty={dirty} run={run} />
      <Card title={t("ui.settings.window.title")}>
        <Toggle
          label={t("ui.settings.window.onTop")}
          checked={view.general.alwaysOnTop}
          onChange={(on) => void run(() => backend.setAlwaysOnTop(on))}
          help={<KeyedTip k="ui.settings.window" />}
        />
      </Card>
      <Card title={t("ui.settings.notices.title")}>
        <Toggle
          label={t("ui.settings.notices.sync")}
          checked={view.general.syncNotice}
          onChange={(show) => void run(() => backend.setSyncNotice(show))}
          help={<KeyedTip k="ui.settings.notices" />}
        />
      </Card>
    </>
  );
}
