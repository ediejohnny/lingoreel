// The only module that talks to the Go side: calls, events and the shape of
// what they carry. Screens import from here, never from wailsjs directly.
import * as api from "../../wailsjs/go/app/API";
import type { app } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";

export type Video = app.Video;
export type Track = app.Track;
export type Players = app.Players;
export type PlayerInstall = app.PlayerInstall;
export type Playback = app.Playback;
export type Preferences = app.Preferences;
export type WatchRequest = app.WatchRequest;
export type Line = "primary" | "translation";

export interface Ended {
  error: string;
}

export interface InstallProgress {
  id: string;
  done: boolean;
  error: string;
}

/** Event names, as the Go side sends them. */
const events = {
  dropped: "video:dropped",
  playback: "playback:state",
  ended: "playback:ended",
  install: "player:install",
} as const;

/** messageOf turns a failed call into the sentence the Go side wrote. */
export function messageOf(error: unknown): string {
  if (typeof error === "string" && error !== "") {
    return error;
  }
  if (error instanceof Error && error.message !== "") {
    return error.message;
  }
  return "Algo deu errado. Tente de novo.";
}

export const backend = {
  chooseVideo: () => api.ChooseVideo(),
  initialVideo: () => api.InitialVideo(),
  openVideo: (path: string) => api.OpenVideo(path),
  players: () => api.Players(),
  watch: (request: WatchRequest) => api.Watch(request),
  stop: () => api.StopWatching(),
  togglePause: () => api.TogglePause(),
  seekBy: (seconds: number) => api.SeekBy(seconds),
  seekTo: (seconds: number) => api.SeekTo(seconds),
  setSpeed: (speed: number) => api.SetSpeed(speed),
  nudge: (line: Line, later: boolean) => api.Nudge(line, later),
  resetSync: (line: Line) => api.ResetSync(line),
  setTranslationVisible: (visible: boolean) => api.SetTranslationVisible(visible),
  playbackState: () => api.PlaybackState(),
  preferences: () => api.Preferences(),
  setTheme: (theme: string) => api.SetTheme(theme),
  installPlayer: (id: string) => api.InstallPlayer(id),
  openDownloadPage: (id: string) => api.OpenDownloadPage(id),
  dismissRecommendation: (id: string) => api.DismissRecommendation(id),

  onDropped: (listener: (path: string) => void) => EventsOn(events.dropped, listener),
  onPlayback: (listener: (state: Playback) => void) => EventsOn(events.playback, listener),
  onEnded: (listener: (ended: Ended) => void) => EventsOn(events.ended, listener),
  onInstall: (listener: (progress: InstallProgress) => void) => EventsOn(events.install, listener),
};
