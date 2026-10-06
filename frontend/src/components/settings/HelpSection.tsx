import { ScrollText } from "lucide-react";
import { useEffect, useState } from "react";
import { t } from "../../i18n";
import { backend, messageOf } from "../../services/backend";
import { Button } from "../ui/Button";
import { Card } from "../ui/Feedback";

/** HelpSection tells what the app is and whose code it carries; the guide to using the app will join it here. */
export function HelpSection() {
  return <AboutCard />;
}

/**
 * AboutCard tells the app's version and license and shows, when asked, the
 * licenses of the code from others it carries, as their authors ask. They are
 * long, so they are fetched only to be read.
 */
function AboutCard() {
  const [version, setVersion] = useState("");
  const [notices, setNotices] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState("");

  useEffect(() => {
    backend
      .about()
      .then((about) => setVersion(about.version))
      .catch((failure) => setStatus(messageOf(failure)));
  }, []);

  const toggle = () => {
    if (open || notices !== null) {
      setOpen(!open);
      return;
    }
    setLoading(true);
    backend
      .thirdPartyNotices()
      .then((text) => {
        setNotices(text);
        setOpen(true);
        setStatus("");
      })
      .catch((failure) => setStatus(messageOf(failure)))
      .finally(() => setLoading(false));
  };

  return (
    <Card title={t("ui.settings.about.title")}>
      <p className="text-base font-semibold text-fg">{t("ui.app.name")}</p>
      {version && <p className="text-sm text-fg-secondary">{t("ui.settings.about.version", { version })}</p>}
      <p className="mb-3 text-sm text-fg-secondary">{t("ui.settings.about.license")}</p>
      <p className="mb-3 text-sm text-fg-secondary">{t("ui.settings.about.notices")}</p>
      <div className="flex flex-wrap items-center gap-2">
        <Button icon={ScrollText} loading={loading} aria-expanded={open} onClick={toggle}>
          {open ? t("ui.settings.about.hide") : t("ui.settings.about.show")}
        </Button>
        {status && (
          <span role="status" className="text-sm text-fg-secondary">
            {status}
          </span>
        )}
      </div>
      {open && notices !== null && (
        <pre
          tabIndex={0}
          aria-label={t("ui.settings.about.text")}
          className="mt-3 max-h-96 overflow-y-auto rounded-control border border-line bg-surface-sunken p-3 font-mono text-xs leading-relaxed break-words whitespace-pre-wrap text-fg select-text"
        >
          {notices}
        </pre>
      )}
    </Card>
  );
}
