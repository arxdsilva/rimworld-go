package filesave

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"rimworld-go/internal/application/game/savefmt"
)

// Verifier checks that save files are whole (SAV-022): each body is
// decompressed to its end, so a cut file or a flipped byte fails the
// deflate stream or its CRC. A file is read again only when its size or
// modification time changes. Build it once, in the composition root.
type Verifier struct {
	mu   sync.Mutex
	memo map[string]verdict // by path: the files of the last Verify
}

type verdict struct {
	size int64
	mod  time.Time
	err  error
}

// NewVerifier returns a verifier that has seen no file.
func NewVerifier() *Verifier { return &Verifier{memo: map[string]verdict{}} }

// Verify sets Err on each slot whose file is damaged (it wraps
// savefmt.ErrCorrupt). Slots that already have an Err are left alone.
// Unseen files are read in parallel, GOMAXPROCS at a time; one Verify runs
// at a time.
func (v *Verifier) Verify(slots []Slot) {
	v.mu.Lock()
	defer v.mu.Unlock()
	next := make(map[string]verdict, len(slots))
	var todo []int
	for i, sl := range slots {
		if sl.Err != nil {
			continue
		}
		if m, ok := v.memo[sl.Path]; ok && m.size == sl.Size && m.mod.Equal(sl.Modified) {
			slots[i].Err, next[sl.Path] = m.err, m
			continue
		}
		todo = append(todo, i)
	}
	errs := make([]error, len(todo))
	work := make(chan int)
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), len(todo)) {
		wg.Go(func() {
			for k := range work {
				errs[k] = verifyFile(slots[todo[k]].Path)
			}
		})
	}
	for k := range todo {
		work <- k
	}
	close(work)
	wg.Wait()
	for k, i := range todo {
		sl := &slots[i]
		sl.Err = errs[k]
		next[sl.Path] = verdict{sl.Size, sl.Modified, sl.Err}
	}
	v.memo = next
}

// verifyFile decompresses path to its end.
func verifyFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := savefmt.NewReader(f)
	if err != nil {
		return fmt.Errorf("%w: %v", savefmt.ErrCorrupt, err)
	}
	return savefmt.Drain(zr)
}
