// Package kinds holds the eight Core quest kinds (QST-2), one file each,
// for game/quest's root: each implements quest.Kind and changes only the
// quest it is handed. The game passes All to quest.New (quest_host.go).
package kinds

import "rimworld-go/internal/application/game/quest"

// All is every kind, a fresh slice (no package state).
func All() []quest.Kind {
	return []quest.Kind{Joiner{}, OreLump{}, Stash{}, Refugee{}, Camp{}, Rescue{}, Request{}, Talks{}}
}
