package settingsfile

import (
	"encoding/json"
	"fmt"

	"lingoreel/internal/domain/appearance"
	"lingoreel/internal/domain/settings"
	"lingoreel/internal/infrastructure/persistence/jsonstore"
)

// profilesMigrations upgrade older profiles.json documents, one version at
// a time.
//
// Fields added later need no migration when their zero value, or the
// default a document is decoded over, already means what an older document
// meant: the sync notice (config.json) and the shortcuts (a profile without
// them uses the defaults, shortcut.Map).
var profilesMigrations = map[int]jsonstore.Migration{
	1: addKaraoke,
	2: useTheAppFont,
}

// useTheAppFont (version 2 to 3) moves the subtitles of a profile still on
// the old default font to the app's own, which is the new default. A font
// the user picked is never touched.
func useTheAppFont(document map[string]any) error {
	items, _ := document["profiles"].([]any)
	for i, item := range items {
		profile, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("profile %d is not an object", i)
		}
		for _, name := range []string{"primaryStyle", "translationStyle"} {
			style, ok := profile[name].(map[string]any)
			if !ok {
				return fmt.Errorf("profile %d has no %s", i, name)
			}
			if family, _ := style["fontFamily"].(string); family == appearance.PreviousDefaultFont {
				style["fontFamily"] = appearance.AppFont
			}
		}
	}
	return nil
}

// addKaraoke (version 1 to 2) gives every profile the Modo Karaokê's
// defaults; a profile that somehow has them keeps its own.
func addKaraoke(document map[string]any) error {
	defaults, err := asObject(settings.DefaultKaraoke())
	if err != nil {
		return err
	}
	items, _ := document["profiles"].([]any)
	for i, item := range items {
		profile, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("profile %d is not an object", i)
		}
		study, ok := profile["study"].(map[string]any)
		if !ok {
			return fmt.Errorf("profile %d has no study preferences", i)
		}
		if _, present := study["karaoke"]; !present {
			study["karaoke"] = defaults
		}
	}
	return nil
}

// asObject turns a value into the generic form a migration edits.
func asObject(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	return object, nil
}
