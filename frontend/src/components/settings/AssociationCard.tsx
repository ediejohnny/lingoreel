import { useEffect, useState } from "react";
import { backend, messageOf } from "../../services/backend";
import { Toggle } from "../ui/Field";
import { Card } from "../ui/Feedback";
import { HelpTip } from "../ui/HelpTip";

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
      setStatus(
        next ? "Pronto: clique com o botão direito num vídeo para ver a opção." : "Pronto: o LingoReel saiu do menu dos vídeos.",
      );
    } catch (failure) {
      setStatus(messageOf(failure));
    }
  };

  return (
    <Card title="Abrir vídeos pelo Windows">
      <Toggle
        label="Mostrar o LingoReel no menu dos vídeos e em “Abrir com”"
        checked={on ?? false}
        disabled={on === null}
        onChange={(next) => void change(next)}
        help={
          <HelpTip topic="abrir vídeos pelo Windows">
            Com o botão direito num vídeo (.mkv, .mp4 e outros), aparece “Assistir com o LingoReel”, e o app entra na lista de “Abrir
            com”. O programa padrão de cada vídeo não muda, e desligar tira tudo de volta. Vale só para o seu usuário do Windows.
          </HelpTip>
        }
      />
      {status && (
        <p role="status" className="mt-2 text-sm text-fg-secondary">
          {status}
        </p>
      )}
    </Card>
  );
}
