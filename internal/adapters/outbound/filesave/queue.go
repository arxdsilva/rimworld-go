package filesave

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/platform/atomicfile"
)

// Queue writes saves off the simulation's goroutine (MASTER §46, CI-002):
// Save encodes the colony's raw gob into memory on the caller's goroutine,
// the only part of an autosave left on the frame, and one worker goroutine
// compresses it and writes the file the way Store.Save does (temp file,
// fsync, rename), then the camera sidecar. A job owns its bytes outright,
// so the worker never reads live state. One job is written at a time; a job
// still waiting for the same slot and kind (save or view) is replaced in
// place, so a slow disk holds at most one save and one view per slot. A
// view whose save failed is skipped. Build it only in the composition root.
type Queue struct {
	store  Store
	saveTo func(path string, raw []byte) error // the save sink (tests gate it)
	viewTo func(slot string, v ViewFile) error // the sidecar sink

	mu      sync.Mutex
	cond    *sync.Cond // wakes the worker for a job and Wait when it goes idle
	jobs    []job      // waiting, oldest first
	running bool       // the worker is writing a job
	closed  bool
	results []Result        // finished since the last Done
	spare   []byte          // a written job's buffer, the next Save's
	last    int             // the last save's raw size, the next buffer's capacity
	failed  map[string]bool // slots whose last save failed: their views are skipped
	busy    atomic.Bool     // a job waits or is being written (Busy, read every frame)
	done    chan struct{}   // closed when the worker returns
}

// Result is one finished job.
type Result struct {
	Slot string // the slot as the caller named it
	View bool   // the camera sidecar, not the save
	Path string // the file written ("" when it failed)
	Err  error
}

// ErrClosed is Save after Close.
var ErrClosed = errors.New("save queue closed")

// maxResults bounds the results nobody has collected yet.
const maxResults = 32

type job struct {
	slot, name string // sanitized; as given
	view       bool
	raw        []byte // a save's raw gob
	camera     ViewFile
}

// NewQueue starts the worker that writes into s.
func NewQueue(s Store) *Queue {
	q := &Queue{store: s, saveTo: writeSave, viewTo: s.SaveView, failed: map[string]bool{}, done: make(chan struct{})}
	q.cond = sync.NewCond(&q.mu)
	go q.run()
	return q
}

// Save encodes g for slot now and queues the file. An error means the save
// did not start (it could not be encoded, or the queue is closed); how the
// write went comes later from Done.
func (q *Queue) Save(g *game.Game, slot string) error {
	return q.put(slot, func(w io.Writer, name string) error { return game.EncodeSave(g, w, name) })
}

// SaveAs is Save with the client's stamp in the header (SAV-003): the slot
// is stamp.Name, and the header keeps it as given.
func (q *Queue) SaveAs(g *game.Game, stamp savefmt.Stamp) error {
	return q.put(stamp.Name, func(w io.Writer, _ string) error { return game.EncodeSaveAs(g, w, stamp) })
}

// SaveView queues the camera sidecar for slot, written after any save for
// slot queued before it.
func (q *Queue) SaveView(slot string, v ViewFile) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrClosed
	}
	q.push(job{slot: sanitize(slot), name: slot, view: true, camera: v})
	return nil
}

// Busy reports a job waits or is being written.
func (q *Queue) Busy() bool { return q.busy.Load() }

// Done returns the jobs finished since the last call, oldest first.
func (q *Queue) Done() []Result {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := q.results
	q.results = nil
	return out
}

// Wait blocks until every queued job is written.
func (q *Queue) Wait() {
	q.mu.Lock()
	for len(q.jobs) > 0 || q.running {
		q.cond.Wait()
	}
	q.mu.Unlock()
}

// Close writes what is queued, then stops the worker. Save after Close
// fails with ErrClosed; a second Close only waits.
func (q *Queue) Close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
	<-q.done
}

// put encodes a save into a buffer sized to the last one and queues it.
func (q *Queue) put(slot string, encode func(w io.Writer, name string) error) error {
	name := sanitize(slot)
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrClosed
	}
	buf, last := q.spare, q.last
	q.spare = nil
	q.mu.Unlock()
	if cap(buf) < last {
		buf = make([]byte, 0, last+last/8)
	}
	b := bytes.NewBuffer(buf[:0])
	if err := encode(b, name); err != nil {
		return fmt.Errorf("encode save: %w", err)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrClosed
	}
	q.last = b.Len()
	q.push(job{slot: name, name: slot, raw: b.Bytes()})
	return nil
}

// push queues j, replacing a waiting job of the same slot and kind in
// place (its order among the others is kept). q.mu is held.
func (q *Queue) push(j job) {
	for i := range q.jobs {
		if q.jobs[i].slot == j.slot && q.jobs[i].view == j.view {
			q.keep(q.jobs[i].raw)
			q.jobs[i] = j
			return
		}
	}
	q.jobs = append(q.jobs, j)
	q.busy.Store(true)
	q.cond.Broadcast()
}

// keep holds a buffer no job owns any more for the next Save. q.mu is held.
func (q *Queue) keep(raw []byte) {
	if cap(raw) > cap(q.spare) {
		q.spare = raw[:0]
	}
}

// run is the worker: one job at a time until Close finds the queue empty.
func (q *Queue) run() {
	defer close(q.done)
	q.mu.Lock()
	defer q.mu.Unlock()
	for {
		for len(q.jobs) == 0 && !q.closed {
			q.cond.Wait()
		}
		if len(q.jobs) == 0 {
			return
		}
		j := q.jobs[0]
		q.jobs = append(q.jobs[:0], q.jobs[1:]...)
		skip := j.view && q.failed[j.slot]
		q.running = true
		q.mu.Unlock()
		r := Result{Slot: j.name, View: j.view}
		if !skip {
			r.Path, r.Err = q.write(j)
		}
		q.mu.Lock()
		q.running = false
		if !j.view {
			q.failed[j.slot] = r.Err != nil
			q.keep(j.raw)
		}
		if !skip {
			q.results = append(q.results, r)
			if len(q.results) > maxResults {
				q.results = append(q.results[:0], q.results[len(q.results)-maxResults:]...)
			}
		}
		q.busy.Store(len(q.jobs) > 0)
		q.cond.Broadcast()
	}
}

// write writes one job and returns the file's path.
func (q *Queue) write(j job) (string, error) {
	if j.view {
		return filepath.Join(q.store.Dir, j.slot+ViewExt), q.viewTo(j.name, j.camera)
	}
	path := filepath.Join(q.store.Dir, j.slot+Ext)
	if err := q.saveTo(path, j.raw); err != nil {
		return "", fmt.Errorf("store save: %w", err)
	}
	return path, nil
}

// writeSave is the save sink: raw gob compressed into the file atomically.
func writeSave(path string, raw []byte) error {
	return atomicfile.Write(path, 0o600, func(w io.Writer) error {
		bw := bufio.NewWriterSize(w, 1<<16) // gzip writes in small blocks
		if err := savefmt.Compress(bw, raw); err != nil {
			return err
		}
		return bw.Flush()
	})
}
