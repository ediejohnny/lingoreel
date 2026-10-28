import { useEffect, useState } from "react";
import { t } from "../../i18n";
import { backend, messageOf } from "../../services/backend";
import { Toggle } from "../ui/Field";
import { Card } from "../ui/Feedback";
import { KeyedTip } from "../ui/HelpTip";

/** AssociationCard lets Windows offer the app for videos, when the user wants it. */
export function AssociationCard() {
  const [on, setOn] = useState<boolean | null>(null);
  const [status, setStatus] = useState("");

  useEffect(() => {
    backend
      .fileAssociation()
      .then(setOn)
      .catch((failure) => setStatus(messageOf(failure)));
  }, []);

  const change = async (next: boolean) => {
    setStatus("");
    try {
      await backend.setFileAssociation(next);
      setOn(next);
      setStatus(next ? t("ui.settings.association.on") : t("ui.settings.association.off"));
    } catch (failure) {
      setStatus(messageOf(failure));
    }
  };

  return (
    <Card title={t("ui.settings.association.title")}>
      <Toggle
        label={t("ui.settings.association.label")}
        checked={on ?? false}
        disabled={on === null}
        onChange={(next) => void change(next)}
        help={<KeyedTip k="ui.settings.association" />}
      />
      {status && (
        <p role="status" className="mt-2 text-sm text-fg-secondary">
          {status}
        </p>
      )}
    </Card>
  );
}
