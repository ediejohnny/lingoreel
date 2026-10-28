import { useCallback, useEffect, useRef, useState } from "react";
import { backend, messageOf } from "../services/backend";

/** ffmpeg is the id under which installing FFmpeg is told; every other id is a player's. */
const ffmpeg = "ffmpeg";

/**
 * useInstall follows installing a tool with winget, for a screen that offers either FFmpeg or the players: which one
 * is being installed, and why the last one failed. onInstalled runs once one of them is in place, from this screen or
 * another.
 */
export function useInstall(tools: "ffmpeg" | "players", onInstalled: () => void) {
  const [installing, setInstalling] = useState("");
  const [error, setError] = useState("");
  const latest = useRef(onInstalled);
  useEffect(() => {
    latest.current = onInstalled;
  });

  useEffect(
    () =>
      backend.onInstall((progress) => {
        if ((progress.id === ffmpeg) !== (tools === "ffmpeg")) {
          return;
        }
        setInstalling("");
        setError(progress.error);
        if (!progress.error) {
          latest.current();
        }
      }),
    [tools],
  );

  /** start installs a player by its id, or FFmpeg; the outcome comes later, told by the Go side. */
  const start = useCallback(
    (id: string) => {
      setInstalling(id);
      setError("");
      (tools === "ffmpeg" ? backend.installFFmpeg() : backend.installPlayer(id)).catch((failure: unknown) => {
        setInstalling("");
        setError(messageOf(failure));
      });
    },
    [tools],
  );

  return { installing, error, start };
}
