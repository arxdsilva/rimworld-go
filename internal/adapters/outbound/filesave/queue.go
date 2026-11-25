// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package filesave

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/savecanon"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/platform/atomicfile"
)

// Queue writes saves off the simulation's goroutine (MASTER §46, CI-002):
// Save freezes the colony on the caller's goroutine. One worker serializes
// the detached values to Gob, canonicalizes, compresses and atomically writes
// the file (temp file, fsync, rename), then the camera sidecar. A job owns its snapshot outright,
// so the worker never reads live state. One job is written at a time; a job
// still waiting for the same slot and kind (save or view) is replaced in
// place, so a slow disk holds at most one save and one view per slot. A
// view whose save failed is skipped. Delete jumps the queue: it drops the
// slot's waiting writes and runs after the in-flight job and earlier deletes (SAV-010).
// Build it only in the composition root.
type Queue struct {
	store  Store
	saveTo func(path string, raw []byte) error // the save sink (tests gate it)
	encode func(savecanon.Frozen, []byte) (savecanon.Captured, error)
	viewTo func(slot string, v ViewFile) error // the sidecar sink
	delTo  func(slot string) error             // the delete sink

	mu        sync.Mutex
	cond      *sync.Cond // wakes the worker for a job and Wait when it goes idle
	jobs      []job      // waiting, oldest first
	running   bool       // the worker is writing a job
	closed    bool
	results   []Result        // finished since the last Done
	spare     []byte          // worker-only Gob buffer
	canonical []byte          // worker-only scratch for the canonicalized bytes
	failed    map[string]bool // slots whose last save failed: their views are skipped
	busy      atomic.Bool     // a job waits or is being written (Busy, read every frame)
	done      chan struct{}   // closed when the worker returns
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
	capture    savecanon.Frozen // a save's exclusively owned values
	camera     ViewFile
	deleted    chan error // a delete: its result goes here, not to Done
}

// NewQueue starts the worker that writes into s.
func NewQueue(s Store) *Queue {
	q := &Queue{store: s, saveTo: writeSave, encode: savecanon.Frozen.Encode, viewTo: s.SaveView, delTo: s.Delete, failed: map[string]bool{}, done: make(chan struct{})}
	q.cond = sync.NewCond(&q.mu)
	go q.run()
	return q
}

// Save captures g for slot now and queues the file. An error means the save
// did not start (capture failed or the queue is closed); encoding, canonicalization
// and write failures come later from Done.
func (q *Queue) Save(g *game.Game, slot string) error {
	return q.put(slot, func(name string) (savecanon.Frozen, error) {
		return game.FreezeSaveAs(g, savefmt.Stamp{Name: name})
	})
}

// SaveAs is Save with the client's stamp in the header (SAV-003): the slot
// is stamp.Name, and the header keeps it as given.
func (q *Queue) SaveAs(g *game.Game, stamp savefmt.Stamp) error {
	return q.put(stamp.Name, func(_ string) (savecanon.Frozen, error) {
		return game.FreezeSaveAs(g, stamp)
	})
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

// Delete removes slot's save and sidecar (Store.Delete) and returns how
// it went. The slot's waiting save and view jobs are dropped, and the
// delete runs after the job in flight and earlier deletes, ahead of any
// ordinary writes. A save queued after Delete writes the slot again.
func (q *Queue) Delete(slot string) error {
	j := job{slot: sanitize(slot), name: slot, deleted: make(chan error, 1)}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrClosed
	}
	rest := make([]job, 0, len(q.jobs)+1)
	// A pending delete owns a caller waiting for its result. Keep earlier
	// deletes in order; only ordinary save/view jobs may be superseded.
	for _, w := range q.jobs {
		if w.deleted != nil {
			rest = append(rest, w)
		}
	}
	rest = append(rest, j)
	for _, w := range q.jobs {
		if w.deleted != nil {
			continue
		}
		if w.slot == j.slot {
			continue
		}
		rest = append(rest, w)
	}
	q.jobs = rest
	q.busy.Store(true)
	q.cond.Broadcast()
	q.mu.Unlock()
	return <-j.deleted
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

// put freezes a save on the caller and queues its owned values.
func (q *Queue) put(slot string, capture func(name string) (savecanon.Frozen, error)) error {
	name := sanitize(slot)
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrClosed
	}
	q.mu.Unlock()
	c, err := capture(name)
	if err != nil {
		return fmt.Errorf("capture save: %w", err)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrClosed
	}
	q.push(job{slot: name, name: slot, capture: c})
	return nil
}

// push queues j, replacing a waiting job of the same slot and kind in
// place (its order among the others is kept). q.mu is held.
func (q *Queue) push(j job) {
	for i := range q.jobs {
		if q.jobs[i].deleted == nil && q.jobs[i].slot == j.slot && q.jobs[i].view == j.view {
			q.jobs[i] = j
			return
		}
	}
	q.jobs = append(q.jobs, j)
	q.busy.Store(true)
	q.cond.Broadcast()
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
			q.spare, q.canonical, q.jobs = nil, nil, nil
			return
		}
		j := q.jobs[0]
		q.jobs = slices.Delete(q.jobs, 0, 1)
		if j.deleted != nil {
			q.remove(j)
			continue
		}
		skip := j.view && q.failed[j.slot]
		q.running = true
		q.mu.Unlock()
		r := Result{Slot: j.name, View: j.view}
		if !skip {
			r.Path, r.Err = q.write(&j)
		}
		q.mu.Lock()
		q.running = false
		if !j.view {
			q.failed[j.slot] = r.Err != nil
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

// remove runs a delete job: the slot's earlier failure is forgotten, so a
// view after a new save is written again. q.mu is held, and released while
// the files go.
func (q *Queue) remove(j job) {
	q.running = true
	q.mu.Unlock()
	err := q.delTo(j.name)
	q.mu.Lock()
	q.running = false
	delete(q.failed, j.slot)
	j.deleted <- err
	q.busy.Store(len(q.jobs) > 0)
	q.cond.Broadcast()
}

// write writes one job and returns the file's path.
func (q *Queue) write(j *job) (string, error) {
	if j.view {
		return filepath.Join(q.store.Dir, j.slot+ViewExt), q.viewTo(j.name, j.camera)
	}
	path := filepath.Join(q.store.Dir, j.slot+Ext)
	c, err := q.encode(j.capture, q.spare)
	j.capture = savecanon.Frozen{}
	if err != nil {
		return "", fmt.Errorf("encode save: %w", err)
	}
	raw, err := c.CanonicalBytes(q.canonical)
	q.spare = c.Recycle()
	if err != nil {
		return "", fmt.Errorf("canonical save: %w", err)
	}
	q.canonical = raw[:0]
	if err := q.saveTo(path, raw); err != nil {
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
