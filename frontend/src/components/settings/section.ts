import type { Dispatch, SetStateAction } from "react";
import type { ProfileSettings, SettingsView } from "../../services/backend";
import type { SectionId } from "../../settings";

/** SectionProps is what every section of the settings screen receives. */
export interface SectionProps {
  view: SettingsView;
  /** draft is the active profile being edited; saved by the page. */
  draft: ProfileSettings;
  setDraft: Dispatch<SetStateAction<ProfileSettings | null>>;
  /** dirty tells the draft has unsaved changes. */
  dirty: boolean;
  /** run carries out an action that applies at once, then reloads; done is said when it worked, and it resolves true then. */
  run: (action: () => Promise<unknown>, done?: string | (() => string)) => Promise<boolean>;
  /** open shows another page of the settings. */
  open?: (section: SectionId) => void;
  /** welcome opens the first run's welcome again; absent while a video plays. */
  welcome?: () => void;
}

/** change returns a setter of one part of the draft. */
export function change<K extends keyof ProfileSettings>(setDraft: SectionProps["setDraft"], key: K) {
  return (value: ProfileSettings[K]) => setDraft((draft) => (draft ? { ...draft, [key]: value } : draft));
}
