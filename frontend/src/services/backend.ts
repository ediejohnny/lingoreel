// The only module that talks to the Go side: calls, events and the shape of
// what they carry. Screens import from here, never from wailsjs directly.
import * as api from "../../wailsjs/go/app/API";
import type { app } from "../../wailsjs/go/models";
import { ClipboardSetText, EventsOn } from "../../wailsjs/runtime/runtime";
import { t } from "../i18n";

export type Video = app.Video;
export type Track = app.Track;
export type Players = app.Players;
export type PlayerInstall = app.PlayerInstall;
export type Playback = app.Playback;
export type Preferences = app.Preferences;
export type WatchRequest = app.WatchRequest;
export type PassageRequest = app.PassageRequest;
export type PassageTimes = app.PassageTimes;
export type Choice = app.Option;
export type Editing = app.Editing;
export type SavedPassage = app.SavedPassage;
export type PassagePlan = Plain<app.PassagePlan>;
export type PassageOptions = Plain<app.PassageOptions>;
export type EditRequest = Plain<app.EditRequest>;
export type Diagnostics = Plain<app.Diagnostics>;

/** Plain is a generated class's data without its methods, as the screens build it. */
type Plain<T> = T extends (infer U)[]
  ? Plain<U>[]
  : T extends object
    ? { [K in keyof T as T[K] extends (...args: never[]) => unknown ? never : K]: Plain<T[K]> }
    : T;

export type SettingsView = Plain<app.SettingsView>;
export type ProfileSettings = Plain<app.ProfileSettings>;
export type StyleSettings = Plain<app.StyleSettings>;
export type LayoutSettings = Plain<app.LayoutSettings>;
export type KaraokeSettings = Plain<app.KaraokeSettings>;
export type VoiceSettings = Plain<app.VoiceSettings>;
export type MarkSettings = Plain<app.MarkSettings>;
export type NoticeSettings = Plain<app.NoticeSettings>;
export type SelectionSettings = Plain<app.SelectionSettings>;
export type ShortcutSetting = Plain<app.ShortcutSetting>;
export type PlayerSetting = Plain<app.PlayerSetting>;
export type SettingsOptions = Plain<app.SettingsOptions>;
export type PreviewRequest = Plain<app.PreviewRequest>;
export type AppearanceSettings = Plain<app.AppearanceSettings>;
export type BarSettings = Plain<app.BarSettings>;
export type EditSettings = Plain<app.EditSettings>;
export type BarPreviewRequest = Plain<app.BarPreviewRequest>;
export type InterfaceSettings = Plain<app.InterfaceSettings>;
export type VideoLayout = Plain<app.VideoLayout>;
export type PanelState = Plain<app.PanelState>;
export type FillCheck = Plain<app.FillCheck>;

/** PanelRequest asks to open the panel beside the video on a section: "editor" or "settings" (with a page). */
export interface PanelRequest {
  section: string;
  page?: string;
}
export type Line = "primary" | "translation";

export interface Ended {
  error: string;
  /** resume tells the session can start again where it was. */
  resume: boolean;
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
  fullscreen: "window:fullscreen",
  panel: "panel:open",
  video: "video:changed",
} as const;

/** messageOf turns a failed call into the sentence the Go side wrote. */
export function messageOf(error: unknown): string {
  if (typeof error === "string" && error !== "") {
    return error;
  }
  if (error instanceof Error && error.message !== "") {
    return error.message;
  }
  return t("ui.common.error");
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
  setStudyMode: (mode: string) => api.SetStudyMode(mode),
  shortcuts: (): Promise<ShortcutSetting[]> => api.Shortcuts(),
  runShortcut: (command: string, repeat: boolean) => api.RunShortcut(command, repeat),
  reveal: (pressed: boolean) => api.Reveal(pressed),
  playbackState: () => api.PlaybackState(),
  setKaraoke: (enabled: boolean) => api.SetKaraoke(enabled),
  setKaraokeIndicator: (indicator: string) => api.SetKaraokeIndicator(indicator),
  setLineRepeats: (times: number) => api.SetLineRepeats(times),
  replayLine: () => api.ReplayLine(),
  studyPassage: (request: PassageRequest) => api.StudyPassage(request),
  passageOfLine: () => api.PassageOfLine(),
  stopPassage: () => api.StopPassage(),
  resumePassage: () => api.ResumePassage(),
  savePassage: (name: string) => api.SavePassage(name),
  deletePassage: (name: string) => api.DeletePassage(name),
  renamePassage: (from: string, to: string) => api.RenamePassage(from, to),
  savedPassages: () => api.SavedPassages(),
  studySavedPassage: (name: string) => api.StudySavedPassage(name),
  setPassageOptions: (options: PassageOptions) => api.SetPassageOptions(options as app.PassageOptions),
  resetProject: () => api.ResetProject(),
  switchSubtitle: (line: Line, track: number) => api.SwitchSubtitle(line, track),
  switchAudio: (track: number) => api.SwitchAudio(track),
  /** addSubtitleFile asks for a subtitle file and puts it on a line; an empty path means the user cancelled. */
  addSubtitleFile: (line: Line): Promise<Video> => api.AddSubtitleFile(line),
  selectLine: (line: Line) => api.SelectLine(line),
  clearSelection: () => api.ClearSelection(),
  editLine: (request: EditRequest) => api.EditLine(request as app.EditRequest),
  restoreLine: () => api.RestoreLine(),
  undoEdit: () => api.UndoEdit(),
  redoEdit: () => api.RedoEdit(),
  exportSubtitle: (line: Line) => api.ExportSubtitle(line),
  preferences: () => api.Preferences(),
  setTheme: (theme: string) => api.SetTheme(theme),
  setLanguage: (tag: string) => api.SetLanguage(tag),
  installPlayer: (id: string) => api.InstallPlayer(id),
  openDownloadPage: (id: string) => api.OpenDownloadPage(id),
  dismissRecommendation: (id: string) => api.DismissRecommendation(id),
  showRecommendations: () => api.ShowRecommendations(),
  settings: (): Promise<SettingsView> => api.Settings(),
  // The Go side reads plain data: the generated classes only add helpers.
  saveProfile: (profile: ProfileSettings) => api.SaveProfile(profile as app.ProfileSettings),
  previewLook: (profile: ProfileSettings) => api.PreviewLook(profile as app.ProfileSettings),
  endPreview: () => api.EndPreview(),
  setSyncNotice: (show: boolean) => api.SetSyncNotice(show),
  setAlwaysOnTop: (on: boolean) => api.SetAlwaysOnTop(on),
  setVoice: (voice: VoiceSettings) => api.SetVoice(voice as app.VoiceSettings),
  chooseFFmpeg: () => api.ChooseFFmpeg(),
  clearFFmpeg: () => api.ClearFFmpeg(),
  installFFmpeg: () => api.InstallFFmpeg(),
  switchProfile: (name: string) => api.SwitchProfile(name),
  createProfile: (name: string) => api.CreateProfile(name),
  renameProfile: (from: string, to: string) => api.RenameProfile(from, to),
  deleteProfile: (name: string) => api.DeleteProfile(name),
  resetProfile: () => api.ResetProfile(),
  applyPreset: (style: StyleSettings, preset: string): Promise<StyleSettings> => api.ApplyPreset(style as app.StyleSettings, preset),
  checkShortcut: (chord: string) => api.CheckShortcut(chord),
  checkFill: (text: string, fill: string): Promise<FillCheck> => api.CheckFill(text, fill),
  previewSubtitles: (request: PreviewRequest) => api.PreviewSubtitles(request as app.PreviewRequest),
  setAppearance: (look: AppearanceSettings) => api.SetAppearance(look as app.AppearanceSettings),
  setInterface: (look: InterfaceSettings) => api.SetInterface(look as app.InterfaceSettings),
  previewBar: (request: BarPreviewRequest) => api.PreviewBar(request as app.BarPreviewRequest),
  choosePlayerProgram: (id: string) => api.ChoosePlayerProgram(id),
  clearPlayerPath: (id: string) => api.ClearPlayerPath(id),
  openSettingsFolder: () => api.OpenSettingsFolder(),
  diagnostics: (): Promise<Diagnostics> => api.Diagnostics(),
  fileAssociation: () => api.FileAssociation(),
  setFileAssociation: (on: boolean) => api.SetFileAssociation(on),
  copyText: (text: string) => ClipboardSetText(text),
  setVideoLayout: (place: VideoLayout) => api.SetVideoLayout(place as app.VideoLayout),
  setFullscreen: (on: boolean) => api.SetFullscreen(on),
  setPanel: (state: PanelState) => api.SetPanel(state as app.PanelState),

  onDropped: (listener: (path: string) => void) => EventsOn(events.dropped, listener),
  onPlayback: (listener: (state: Playback) => void) => EventsOn(events.playback, listener),
  onEnded: (listener: (ended: Ended) => void) => EventsOn(events.ended, listener),
  onInstall: (listener: (progress: InstallProgress) => void) => EventsOn(events.install, listener),
  onFullscreen: (listener: (on: boolean) => void) => EventsOn(events.fullscreen, listener),
  onPanel: (listener: (request: PanelRequest) => void) => EventsOn(events.panel, listener),
  onVideo: (listener: (video: Video) => void) => EventsOn(events.video, listener),
};
