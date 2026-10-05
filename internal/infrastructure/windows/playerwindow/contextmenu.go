//go:build windows && amd64

package playerwindow

import (
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"github.com/ediejohnny/lingoreel/internal/application/session"
	"github.com/ediejohnny/lingoreel/internal/application/watch"
	"github.com/ediejohnny/lingoreel/internal/domain/appearance"
	"github.com/ediejohnny/lingoreel/internal/domain/contextmenu"
	"github.com/ediejohnny/lingoreel/internal/domain/controlbar"
	"github.com/ediejohnny/lingoreel/internal/infrastructure/windows/win32"
)

// AppActions are what the menu over the video asks of the app: its
// settings, switching tracks, subtitle files, the clipboard and another
// video. The app implements them; each returns at once, off the window's
// thread, and tells on the video what went wrong.
type AppActions interface {
	// OpenSettings opens the panel's settings at a page ("subtitles-primary").
	OpenSettings(page string)
	SwitchSubtitle(line session.Line, track uint64)
	SwitchAudio(track uint64)
	AddSubtitle(line session.Line)
	ExportSubtitle(line session.Line)
	CopyText(text string)
	// OpenVideo ends this video and asks for another; StopVideo ends it.
	OpenVideo()
	StopVideo()
}

// iconKey is one picture of the menu: an icon at a size, enabled or not.
type iconKey struct {
	icon    contextmenu.Icon
	size    int
	enabled bool
}

// keyboardPoint is where WM_CONTEXTMENU says a key opened the menu.
var keyboardPoint = win32.Point{X: -1, Y: -1}

// openContextMenu shows the menu at a point on screen (keyboardPoint: from
// the keyboard, at the video's center) and carries out the item chosen.
// Runs on the UI thread; the menu's loop runs the thread's jobs meanwhile.
func (w *Window) openContextMenu(at win32.Point) {
	remote := w.currentRemote()
	if remote == nil || w.host == 0 || w.menuOpen || w.drag != controlbar.ActionNone {
		return
	}
	client, err := win32.ClientRectOnScreen(w.host)
	if err != nil {
		return
	}
	if at == keyboardPoint {
		at = win32.Point{X: (client.Left + client.Right) / 2, Y: (client.Top + client.Bottom) / 2}
	}
	// The bar's menus close, and the mouse stays in sight over the menu.
	w.speedMenu, w.openMenu = false, controlbar.MenuNone
	win32.KillTimer(w.host, timerCursor)
	if w.hideCursor {
		w.hideCursor = false
		win32.SetCursor(win32.ArrowCursor())
	}
	items := contextmenu.Build(w.menuState(remote, w.lineUnder(remote, at, client)))
	menu, chosen, err := w.buildMenu(items)
	if err != nil {
		win32.DestroyMenu(menu)
		w.logger.Warn("the context menu could not be built", "error", err)
		return
	}
	w.menuOpen = true
	id := w.track(menu, at)
	w.menuOpen = false
	win32.DestroyMenu(menu)
	if w.host == 0 {
		// The window went while the menu showed: nothing chosen counts.
		if w.iconsLeft {
			w.freeIcons()
		}
		return
	}
	if id > 0 && int(id) <= len(chosen) {
		w.carryOut(chosen[id-1])
	}
	w.pointer(w.cursorInZone())
}

// track shows the menu and returns the ID chosen.
func (w *Window) track(menu windows.Handle, at win32.Point) uint32 {
	if w.options.Choose != nil {
		return w.options.Choose(uintptr(menu), at.X, at.Y)
	}
	return win32.TrackPopupMenu(menu, w.host, at.X, at.Y)
}

// lineUnder is the subtitle line drawn under a point on screen, with the
// text it shows; nil elsewhere.
func (w *Window) lineUnder(remote watch.Remote, at win32.Point, client win32.Rect) *contextmenu.Under {
	if w.overlay == nil {
		return nil
	}
	line, hit := w.overlay.LineAt(float64(at.X-client.Left), float64(at.Y-client.Top))
	if !hit {
		return nil
	}
	shown := remote.Shown()
	events := shown.Primary
	under := &contextmenu.Under{Line: contextmenu.LinePrimary}
	if line == session.LineTranslation {
		events, under.Line = shown.Translation, contextmenu.LineTranslation
	}
	texts := make([]string, len(events))
	for i, event := range events {
		texts[i] = event.Text()
	}
	under.Text = strings.Join(texts, "\n")
	return under
}

// menuState is what the menu shows, from the session.
func (w *Window) menuState(remote watch.Remote, under *contextmenu.Under) contextmenu.State {
	tracks := remote.Tracks()
	return contextmenu.State{
		Bar: w.state(remote), Tracks: tracks.List, Current: tracks.Current,
		Offsets: [2]time.Duration{
			remote.Offset(session.LinePrimary).Duration(), remote.Offset(session.LineTranslation).Duration(),
		},
		CanUndo: remote.CanUndo(), CanRedo: remote.CanRedo(), Under: under, App: w.options.App != nil,
	}
}

// buildMenu makes the native menu of the items, numbering the items that
// can be chosen from 1; chosen lists them in that order.
func (w *Window) buildMenu(items []contextmenu.Item) (windows.Handle, []contextmenu.Item, error) {
	menu, err := win32.CreatePopupMenu()
	if err != nil {
		return 0, nil, err
	}
	var chosen []contextmenu.Item
	if err := w.fill(menu, items, &chosen); err != nil {
		return menu, nil, err
	}
	return menu, chosen, nil
}

func (w *Window) fill(menu windows.Handle, items []contextmenu.Item, chosen *[]contextmenu.Item) error {
	keys := w.shortcuts()
	for _, item := range items {
		native := win32.MenuItem{Separator: item.Kind == contextmenu.KindSeparator}
		if !native.Separator {
			if words := w.options.Labels.Menu; words != nil {
				text := words(item, keys)
				native.Text = menuText(text)
			}
			native.Radio, native.Checked, native.Disabled = item.Radio, item.Checked, !item.Enabled
			native.Bitmap = w.icon(item.Icon, item.Enabled)
		}
		switch item.Kind {
		case contextmenu.KindSubmenu:
			sub, err := win32.CreatePopupMenu()
			if err != nil {
				return err
			}
			native.SubMenu = sub
			if err := win32.AppendMenuItem(menu, native); err != nil {
				win32.DestroyMenu(sub)
				return err
			}
			if err := w.fill(sub, item.Children, chosen); err != nil {
				return err
			}
			continue
		case contextmenu.KindItem:
			*chosen = append(*chosen, item)
			native.ID = uint32(len(*chosen))
		}
		if err := win32.AppendMenuItem(menu, native); err != nil {
			return err
		}
	}
	return nil
}

// menuText is an item's text with its key on the right; an "&" in a text
// (a file's name) shows as itself, not as an access key.
func menuText(text contextmenu.Text) string {
	words := strings.ReplaceAll(text.Text, "&", "&&")
	if text.Key == "" {
		return words
	}
	return words + "\t" + text.Key
}

// icon is an icon's picture at the monitor's size, in the menu's text
// color, or gray for an unavailable item; zero for no icon or when it
// cannot be drawn (the item then shows without one).
func (w *Window) icon(icon contextmenu.Icon, enabled bool) windows.Handle {
	if icon == contextmenu.IconNone || w.renderer == nil {
		return 0
	}
	key := iconKey{icon: icon, size: win32.SmallIconSize(win32.DpiForWindow(w.host)), enabled: enabled}
	if bitmap, found := w.icons[key]; found {
		return bitmap
	}
	dib, err := win32.NewDIB(key.size, key.size)
	if err != nil {
		return 0
	}
	index := win32.COLOR_MENUTEXT
	if !enabled {
		index = win32.COLOR_GRAYTEXT
	}
	r, g, b := win32.SysColor(index)
	if err := w.renderer.DrawIcon(dib, icon, appearance.RGB(r, g, b)); err != nil {
		dib.Close()
		w.logger.Warn("a menu icon could not be drawn", "error", err)
		return 0
	}
	if w.icons == nil {
		w.icons = map[iconKey]windows.Handle{}
	}
	w.icons[key] = dib.Release()
	return w.icons[key]
}

// freeIcons frees the menu's pictures; never while a menu shows them.
func (w *Window) freeIcons() {
	for key, bitmap := range w.icons {
		win32.DeleteBitmap(bitmap)
		delete(w.icons, key)
	}
	w.iconsLeft = false
}

// carryOut does what an item chosen says: in the window itself, through the
// session, or through the app, off the window's thread (ending the video
// waits on this thread).
func (w *Window) carryOut(item contextmenu.Item) {
	app := w.options.App
	line := session.LinePrimary
	if item.Line == contextmenu.LineTranslation {
		line = session.LineTranslation
	}
	switch item.Action {
	case contextmenu.ActionCommand:
		w.run(item.Command, "", false)
	case contextmenu.ActionEntry:
		w.choose(item.Entry)
	case contextmenu.ActionSpeed:
		speed := item.Speed
		w.do(func(r watch.Remote) error { return r.Controls().SetSpeed(speed) })
	case contextmenu.ActionPreset:
		field, duration, count := item.Field, item.Duration, item.Count
		w.do(func(r watch.Remote) error { return set(r, field, duration, count) })
	case contextmenu.ActionEditHere:
		w.editHere(line)
	case contextmenu.ActionClose:
		_ = win32.PostMessage(w.frame(), win32.WM_SYSCOMMAND, win32.SC_CLOSE, 0)
	}
	if app == nil {
		return
	}
	track := item.Track.Number
	switch item.Action {
	case contextmenu.ActionAudio:
		go app.SwitchAudio(track)
	case contextmenu.ActionSubtitle:
		go app.SwitchSubtitle(line, track)
	case contextmenu.ActionAddSubtitle:
		go app.AddSubtitle(line)
	case contextmenu.ActionExport:
		go app.ExportSubtitle(line)
	case contextmenu.ActionAppearance, contextmenu.ActionSettings:
		go app.OpenSettings(string(item.Page))
	case contextmenu.ActionCopyHere:
		go app.CopyText(item.Text)
	case contextmenu.ActionOpenVideo:
		go app.OpenVideo()
	case contextmenu.ActionStopVideo:
		go app.StopVideo()
	}
}

// editHere pauses and opens the box that corrects a line, in its place.
func (w *Window) editHere(line session.Line) {
	if w.overlay == nil {
		return
	}
	spot, ok := w.overlay.SpotOf(line)
	if !ok {
		return
	}
	w.beginEdit(line, spot, true)
}
