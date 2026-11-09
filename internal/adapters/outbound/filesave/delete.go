package filesave

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"rimworld-go/internal/platform/atomicfile"
)

// Delete removes the save in slot name, then its camera sidecar (SAV-010).
// Either file already gone is not an error: the load list may be stale.
// Each removal syncs the folder.
func (s Store) Delete(name string) error {
	slot := sanitize(name)
	for _, ext := range [...]string{Ext, ViewExt} {
		if err := atomicfile.Remove(filepath.Join(s.Dir, slot+ext)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("delete save: %w", err)
		}
	}
	return nil
}
