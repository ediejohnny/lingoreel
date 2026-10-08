import { ArrowRight, ScrollText } from "lucide-react";
import { useEffect, useState } from "react";
import { searchTopics, topics, type Keys } from "../../help/topics";
import { t } from "../../i18n";
import { backend, messageOf } from "../../services/backend";
import { noticesOf, type NoticeSection, type SectionId } from "../../settings";
import { Button } from "../ui/Button";
import { Card } from "../ui/Feedback";
import { TextField } from "../ui/Field";
import { Select } from "../ui/Select";

/** HelpSection tells what the app is and whose code it carries (Ajuda › Sobre). */
export function HelpSection() {
  return <AboutCard />;
}

interface HelpDocsProps {
  /** keys are the profile's shortcuts, which the texts name. */
  keys: Keys;
  /** narrow lays the topics out as a list to pick from, for the panel beside the video. */
  narrow: boolean;
  onOpen: (section: SectionId) => void;
}

/** HelpDocs is the documentation (Ajuda › Documentação): every topic, a search, and a way to the settings each one is about. */
export function HelpDocs({ keys, narrow, onOpen }: HelpDocsProps) {
  const [query, setQuery] = useState("");
  const [chosen, setChosen] = useState("start");
  const found = searchTopics(topics(keys), query);
  const topic = found.find((item) => item.id === chosen) ?? found[0];

  return (
    <Card title={t("ui.help.title")}>
      <div className="flex flex-col gap-4">
        <p className="text-sm text-fg-secondary">{t("ui.help.intro")}</p>
        <TextField label={t("ui.help.search")} value={query} onChange={setQuery} placeholder={t("ui.help.searchPlaceholder")} />
        {topic === undefined ? (
          <p role="status" className="text-sm text-fg-muted">
            {t("ui.help.noResults", { query: query.trim() })}
          </p>
        ) : (
          <div className={narrow ? "flex flex-col gap-4" : "flex gap-6"}>
            {narrow ? (
              <Select
                label={t("ui.help.topics")}
                value={topic.id}
                options={found.map((item) => ({ value: item.id, label: item.title }))}
                onChange={setChosen}
              />
            ) : (
              <nav aria-label={t("ui.help.topics")} className="w-52 shrink-0">
                <ul className="flex flex-col gap-0.5">
                  {found.map((item) => (
                    <li key={item.id}>
                      <button
                        type="button"
                        aria-current={item.id === topic.id ? "true" : undefined}
                        onClick={() => setChosen(item.id)}
                        className={`w-full rounded-control px-3 py-1.5 text-left text-sm transition-colors duration-[var(--duration-fast)] ${
                          item.id === topic.id
                            ? "bg-accent-soft font-medium text-accent"
                            : "text-fg-secondary hover:bg-sunken hover:text-fg"
                        }`}
                      >
                        {item.title}
                      </button>
                    </li>
                  ))}
                </ul>
              </nav>
            )}
            <article className="flex min-w-0 flex-1 flex-col gap-3 select-text">
              <h3 className="text-base font-semibold text-fg">{topic.title}</h3>
              {topic.paragraphs.map((paragraph, index) => (
                <p key={index} className="text-sm leading-relaxed text-fg-secondary">
                  {paragraph}
                </p>
              ))}
              {topic.open && (
                <div>
                  <Button icon={ArrowRight} onClick={() => topic.open && onOpen(topic.open)}>
                    {t("ui.help.open")}
                  </Button>
                </div>
              )}
            </article>
          </div>
        )}
      </div>
    </Card>
  );
}

/**
 * AboutCard tells the app's version and license and shows, when asked, the
 * licenses of the code from others it carries, as their authors ask. They are
 * long, so they are fetched only to be read.
 */
function AboutCard() {
  const [version, setVersion] = useState("");
  const [notices, setNotices] = useState<NoticeSection[] | null>(null);
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
        setNotices(noticesOf(text));
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
        <div
          tabIndex={0}
          aria-label={t("ui.settings.about.text")}
          className="mt-3 max-h-96 overflow-y-auto rounded-control border border-line bg-surface-sunken select-text"
        >
          {notices.map((notice, index) => (
            <section key={`${index}-${notice.title}`} className="border-b border-line p-3 last:border-b-0">
              <h3 className="mb-2 text-xs font-semibold break-words text-fg">{notice.title}</h3>
              <pre className="font-mono text-xs leading-relaxed break-words whitespace-pre-wrap text-fg-secondary">{notice.text}</pre>
            </section>
          ))}
        </div>
      )}
    </Card>
  );
}
