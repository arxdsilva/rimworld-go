package profiling

import (
	"os"
	"path/filepath"
	"testing"
)

// The profiles land where the flags say, and empty paths write nothing.
func TestProfilesAreWrittenWhereAsked(t *testing.T) {
	dir := t.TempDir()
	trace, heap := filepath.Join(dir, "run.trace"), filepath.Join(dir, "heap.prof")
	Start("", trace)()
	WriteHeap(heap)
	WriteHeap("")
	for _, p := range []string{trace, heap} {
		if fi, err := os.Stat(p); err != nil || fi.Size() == 0 {
			t.Errorf("%s: %v", p, err)
		}
	}
}
