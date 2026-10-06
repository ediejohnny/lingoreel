package settingsfile

import (
	"encoding/json"
	"fmt"

	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/settings"
	"github.com/ediejohnny/lingoreel/internal/domain/shortcut"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/persistence/jsonstore"
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
	3: freeThePassageMarks,
	4: addLooks,
	5: freshenTheSelection,
}

// freshenTheSelection (version 5 to 6) gives every profile still on the old
// frame around the line picked for a correction the new, cleaner one; a
// frame the user changed stays.
func freshenTheSelection(document map[string]any) error {
	fresh, err := asObject(appearance.DefaultSelectionLook())
	if err != nil {
		return err
	}
	items, _ := document["profiles"].([]any)
	for i, item := range items {
		profile, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("profile %d is not an object", i)
		}
		marks, _ := profile["marks"].(map[string]any)
		if marks == nil {
			continue
		}
		raw, err := json.Marshal(marks["selection"])
		if err != nil {
			return err
		}
		var look appearance.SelectionLook
		if json.Unmarshal(raw, &look) == nil && look == appearance.PreviousSelectionLook() {
			marks["selection"] = fresh
		}
	}
	return nil
}

// addLooks (version 4 to 5) gives every profile the looks that were fixed
// before: the line spacing and the gap between the subtitles, the marks and
// the karaoke mode's indicator, whose bar takes the profile's preview color
// as it did. A value a profile somehow has stays.
func addLooks(document map[string]any) error {
	defaults := settings.DefaultProfile()
	marks, err := asObject(defaults.Marks)
	if err != nil {
		return err
	}
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
			keep(style, "lineSpacing", defaults.Primary.LineSpacing)
		}
		layout, ok := profile["layout"].(map[string]any)
		if !ok {
			return fmt.Errorf("profile %d has no layout", i)
		}
		keep(layout, "lineGap", defaults.Layout.LineGap)
		keep(profile, "marks", marks)
		study, _ := profile["study"].(map[string]any)
		karaoke, ok := study["karaoke"].(map[string]any)
		if !ok {
			return fmt.Errorf("profile %d has no karaoke preferences", i)
		}
		cue := defaults.Study.Karaoke.Cue
		if text, _ := karaoke["previewColor"].(string); text != "" {
			if color, err := appearance.ParseColor(text); err == nil {
				cue.BarColor = color
			}
		}
		object, err := asObject(cue)
		if err != nil {
			return err
		}
		keep(karaoke, "cue", object)
	}
	return nil
}

// keep sets key to value unless the object already has it.
func keep(object map[string]any, key string, value any) {
	if _, present := object[key]; !present {
		object[key] = value
	}
}

// newDefaults are the commands whose default chord came in a later version:
// a profile that already gave that chord to another command keeps it there,
// and the new command starts without a shortcut. Otherwise the two would
// conflict, the whole profile would be refused and, saved again, replaced
// by the defaults.
var newDefaults = []shortcut.Command{shortcut.PassageMarkStart, shortcut.PassageMarkEnd}

// freeThePassageMarks (version 3 to 4) leaves Ctrl+F9 and Ctrl+F10 with the
// command a profile gave them, if any.
func freeThePassageMarks(document map[string]any) error {
	defaults := shortcut.Defaults()
	items, _ := document["profiles"].([]any)
	for i, item := range items {
		profile, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("profile %d is not an object", i)
		}
		keys, _ := profile["shortcuts"].(map[string]any)
		if keys == nil {
			continue
		}
		for _, command := range newDefaults {
			if _, own := keys[string(command)]; own {
				continue
			}
			for _, value := range keys {
				text, _ := value.(string)
				chord, err := shortcut.Parse(text)
				if err == nil && chord == defaults[command] {
					keys[string(command)] = ""
					break
				}
			}
		}
	}
	return nil
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

// addKaraoke (version 1 to 2) gives every profile the karaoke mode's
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
