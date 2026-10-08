package ptbr

import "github.com/ediejohnny/lingoreel/internal/domain/player"

// PlayerBenefit is the one-sentence reason to choose a player, in the
// interface's language; a plugin the catalog does not know keeps its own
// sentence.
func PlayerBenefit(id player.ID, own string) string {
	switch id {
	case player.VLC:
		return T("player.benefit.vlc")
	case player.PotPlayer:
		return T("player.benefit.potplayer")
	}
	return own
}

// AssociationNames word what the Explorer shows for the app: the kind of
// file and the entry of the videos' context menu.
func AssociationNames() (kind, menu string) {
	return T("association.type"), T("association.menu")
}
