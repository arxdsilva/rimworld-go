package modloader

import (
	"os"
	"path/filepath"

	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/platform/assetroot"
)

// DefaultRoots are the game's plugin folders, in this order: the mods
// folder shipped next to the game (found like the assets), then
// <user config dir>/rimworld-go/mods where players drop their own.
func DefaultRoots() []string {
	var roots []string
	if p, err := assetroot.Find("mods", true); err == nil {
		roots = append(roots, p)
	}
	if base, err := os.UserConfigDir(); err == nil {
		roots = append(roots, filepath.Join(base, "rimworld-go", "mods"))
	}
	return roots
}

// Open is New as a modding.Source, for wiring that takes a source maker.
func Open(roots ...string) modding.Source { return New(roots...) }
