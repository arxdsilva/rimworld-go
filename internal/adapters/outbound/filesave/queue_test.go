// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package filesave

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/savecanon"
	"rimworld-go/internal/application/game/savefmt"
)

// raw supplies a real Gob capture for the queue's ordering tests.
func raw(body string) func(string) (savecanon.Frozen, error) {
	return func(_ string) (savecanon.Frozen, error) {
		return savecanon.Freeze(body), nil
	}
}

// TestQueueSaveLoadsAtItsTick: the file holds the colony as it was when
// Save returned, while the colony steps on (on this goroutine) and the
// worker writes (on its own): -race checks the worker never reads live
// state.
func TestQueueSaveLoadsAtItsTick(t *testing.T) {
	t.Parallel()
	s := Store{Dir: t.TempDir()}
	q := NewQueue(s)
	defer q.Close()
	gate := make(chan struct{})
	q.saveTo = func(path string, raw []byte) error { <-gate; return writeSave(path, raw) }
	g := game.NewColony(game.Config{Seed: 3, Width: 40, Height: 30})
	g.Step(200)
	tick, hash := g.Tick(), g.StateHash()
	var want bytes.Buffer
	if err := g.Save(&want, "My-colony"); err != nil {
		t.Fatal(err)
	}
	if err := q.Save(g, "My colony!"); err != nil {
		t.Fatal(err)
	}
	if !q.Busy() {
		t.Fatal("a queued save is not busy")
	}
	g.Step(300)
	close(gate) // the worker compresses and writes while the colony steps
	g.Step(300)
	q.Wait()
	if q.Busy() {
		t.Fatal("busy after Wait")
	}
	rs := q.Done()
	if len(rs) != 1 || rs[0].Err != nil || rs[0].Slot != "My colony!" || filepath.Base(rs[0].Path) != "My-colony"+Ext {
		t.Fatalf("results %+v", rs)
	}
	got, err := os.ReadFile(rs[0].Path)
	if err != nil || !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("queued save differs from synchronous canonical save: %v", err)
	}
	h, _, err := s.Load(rs[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if h.Tick() != tick || h.StateHash() != hash {
		t.Fatalf("loaded tick %d hash %x, want the snapshot's %d %x", h.Tick(), h.StateHash(), tick, hash)
	}
	if len(q.Done()) != 0 {
		t.Fatal("Done returned a result twice")
	}
}

// TestQueueOneInFlightCoalesces: one job is written at a time; a waiting
// job of the same slot and kind is replaced in place; a view follows its
// save.
func TestQueueOneInFlightCoalesces(t *testing.T) {
	t.Parallel()
	q := NewQueue(Store{Dir: t.TempDir()})
	defer q.Close()
	var mu sync.Mutex
	var log []string
	inFlight, most := 0, 0
	entered, release := make(chan struct{}, 8), make(chan struct{})
	enter := func(what string) {
		mu.Lock()
		inFlight++
		most = max(most, inFlight)
		log = append(log, what)
		mu.Unlock()
		entered <- struct{}{}
		<-release
		mu.Lock()
		inFlight--
		mu.Unlock()
	}
	q.saveTo = func(path string, raw []byte) error {
		var body string
		if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&body); err != nil {
			return err
		}
		enter(filepath.Base(path) + "=" + body)
		return nil
	}
	q.viewTo = func(slot string, v ViewFile) error { enter(fmt.Sprintf("view %s %v", slot, v.CX)); return nil }
	if err := q.put("a", raw("1")); err != nil {
		t.Fatal(err)
	}
	<-entered // a=1 is being written
	for _, body := range []string{"2", "3"} {
		if err := q.put("a", raw(body)); err != nil {
			t.Fatal(err)
		}
	}
	q.SaveView("a", ViewFile{CX: 1})
	q.put("b", raw("x"))
	q.SaveView("a", ViewFile{CX: 2})
	go func() {
		for range 3 {
			<-entered
		}
	}()
	close(release)
	q.Wait()
	want := "[a.rwsave=1 a.rwsave=3 view a 2 b.rwsave=x]"
	if got := fmt.Sprint(log); got != want || most != 1 {
		t.Fatalf("writes %s (at most %d at once), want %s one at a time", got, most, want)
	}
	if rs := q.Done(); len(rs) != 4 {
		t.Fatalf("results %+v, want 4", rs)
	}
}

// TestQueueFailureSkipsTheView: a save that cannot be written is reported
// and its view is not written; Close writes what waits, then Save fails.
func TestQueueFailureSkipsTheView(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	q := NewQueue(Store{Dir: filepath.Join(file, "saves")}) // a folder under a regular file
	views := 0
	q.viewTo = func(string, ViewFile) error { views++; return nil }
	if err := q.put("col", raw("body")); err != nil {
		t.Fatal(err)
	}
	q.SaveView("col", ViewFile{})
	q.Close()
	rs := q.Done()
	if len(rs) != 1 || rs[0].Err == nil || rs[0].Path != "" || views != 0 {
		t.Fatalf("results %+v, views written %d: want one failed save and no view", rs, views)
	}
	if err := q.put("col", raw("body")); err != ErrClosed {
		t.Fatalf("save after Close: %v", err)
	}
	if err := q.SaveView("col", ViewFile{}); err != ErrClosed {
		t.Fatalf("view after Close: %v", err)
	}
	q.Close() // a second Close only waits
}

// TestQueueReusesTheBuffer: the next save encodes into the last one's
// buffer once it is written.
func TestQueueReusesTheBuffer(t *testing.T) {
	t.Parallel()
	q := NewQueue(Store{Dir: t.TempDir()})
	defer q.Close()
	var bufs, inputs []*byte
	q.saveTo = func(_ string, raw []byte) error { bufs = append(bufs, &raw[:1][0]); return nil }
	for range 2 {
		if err := q.put("a", raw("same size")); err != nil {
			t.Fatal(err)
		}
		q.Wait()
		inputs = append(inputs, &q.spare[:1][0])
	}
	if len(bufs) != 2 || bufs[0] != bufs[1] || inputs[0] != inputs[1] {
		t.Fatal("the second save did not reuse both input and output buffers")
	}
}

// SaveAs must freeze the exact stamp and state even if an earlier job blocks
// the worker. Coalescing may discard an older capture, but not mutate the
// newest one or the in-flight capture through buffer reuse.
func TestQueueCanonicalCaptureCoalescing(t *testing.T) {
	t.Parallel()
	q := NewQueue(Store{Dir: t.TempDir()})
	defer q.Close()
	started, gate := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	defer release()
	q.saveTo = func(path string, raw []byte) error {
		if filepath.Base(path) == "block"+Ext {
			started <- struct{}{}
			<-gate
		}
		return writeSave(path, raw)
	}
	g := game.NewColony(game.Config{Seed: 7, Width: 40, Height: 30})
	g.Step(1)
	if err := q.Save(g, "block"); err != nil {
		t.Fatal(err)
	}
	<-started
	stamp := savefmt.Stamp{Name: "My slot!", Colony: "test"}
	if err := q.SaveAs(g, stamp); err != nil {
		t.Fatal(err)
	}
	g.Step(50)
	var canonical, want bytes.Buffer
	if err := game.EncodeSaveAs(g, &canonical, stamp); err != nil {
		t.Fatal(err)
	}
	if err := savefmt.Compress(&want, canonical.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := q.SaveAs(g, stamp); err != nil {
		t.Fatal(err)
	}
	g.Step(50)
	release()
	q.Wait()
	results := q.Done()
	if len(results) != 2 || results[0].Err != nil || results[1].Err != nil {
		t.Fatalf("results: %+v", results)
	}
	got, err := os.ReadFile(results[1].Path)
	if err != nil || !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("coalesced capture differs from newest canonical save: %v", err)
	}
}

// An unsupported Gob interface may be captured but must fail canonicalization
// on the worker without replacing the last valid file or writing its view.
func TestQueueCanonicalizationFailureIsAtomic(t *testing.T) {
	t.Parallel()
	q := NewQueue(Store{Dir: t.TempDir()})
	defer q.Close()
	if err := q.put("a", raw("previous")); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	rs := q.Done()
	if len(rs) != 1 || rs[0].Err != nil {
		t.Fatalf("initial save: %+v", rs)
	}
	want, err := os.ReadFile(rs[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.put("a", func(_ string) (savecanon.Frozen, error) {
		return savecanon.Freeze(struct{ Value any }{Value: 42}), nil
	}); err != nil {
		t.Fatalf("capture itself failed: %v", err)
	}
	if err := q.SaveView("a", ViewFile{CX: 1}); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	failed := q.Done()
	if len(failed) != 1 || failed[0].Err == nil || failed[0].Path != "" {
		t.Fatalf("expected one failed save and skipped view: %+v", failed)
	}
	got, err := os.ReadFile(rs[0].Path)
	if err != nil || !bytes.Equal(got, want) || exists(t, filepath.Join(q.store.Dir, "a"+ViewExt)) {
		t.Fatalf("failed canonicalization changed previous save or view: %v", err)
	}
}
