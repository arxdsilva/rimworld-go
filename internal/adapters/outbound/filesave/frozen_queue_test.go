// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package filesave

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/modstate"
	"rimworld-go/internal/application/game/savecanon"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

type frozenProvider struct {
	blobs []modstate.Blob
	calls int
}

func (p *frozenProvider) SaveBlobs() []modstate.Blob { p.calls++; return p.blobs }

func TestQueueFreezeBeforeWorkerEncode(t *testing.T) {
	q := NewQueue(Store{Dir: t.TempDir()})
	defer q.Close()
	g := game.NewColony(game.Config{Seed: 12, Width: 100, Height: 100})
	g.Step(120)
	provider := &frozenProvider{blobs: []modstate.Blob{{Plugin: "test", File: "a", Data: []byte{1, 2, 3}}}}
	game.UseModState(g, provider)
	stamp := savefmt.Stamp{Name: "frozen", Colony: "proof", Autosave: true, Content: savefmt.Content{Plugins: []string{"test"}}}
	var want, raw bytes.Buffer
	if err := game.EncodeSaveAs(g, &raw, stamp); err != nil {
		t.Fatal(err)
	}
	if err := savefmt.Compress(&want, raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	tick, hash := g.Tick(), g.StateHash()
	started, gate := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	defer release()
	first := true
	q.encode = func(f savecanon.Frozen, buf []byte) (savecanon.Captured, error) {
		if first {
			first = false
			close(started)
			<-gate
		}
		return f.Encode(buf)
	}
	if err := q.SaveAs(g, stamp); err != nil {
		t.Fatal(err)
	}
	<-started
	if provider.calls != 2 {
		t.Fatal("provider not captured on caller")
	}
	// All these changes happen before worker encoding; more ticks follow
	// after release and overlap the worker's serialization.
	provider.blobs[0].Data[0] = 9
	stamp.Content.Plugins[0] = "changed"
	g.SetGround(1, 1, 3, 1, "rock", "marble", "silver")
	if _, err := g.SpawnStack(item.Wood, 7, 2, 2); err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 0, Y: 0, X2: 8, Y2: 8})
	g.Step(60)
	if err := q.SaveAs(g, savefmt.Stamp{Name: "other"}); err != nil {
		t.Fatal(err)
	}
	g.Step(5)
	if err := q.SaveAs(g, savefmt.Stamp{Name: "other"}); err != nil {
		t.Fatal(err)
	}
	lastTick := g.Tick()
	q.mu.Lock()
	n := len(q.jobs)
	q.mu.Unlock()
	if n != 1 {
		t.Fatalf("same-slot snapshots did not coalesce: %d", n)
	}
	release()
	g.Step(120)
	q.Wait()
	rs := q.Done()
	if len(rs) != 2 || rs[0].Err != nil || rs[1].Err != nil {
		t.Fatalf("save results: %+v", rs)
	}
	got, err := os.ReadFile(rs[0].Path)
	if err != nil || !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("saved bytes are not capture-time bytes: %v", err)
	}
	h, info, err := game.Load(bytes.NewReader(got))
	if err != nil || h.Tick() != tick || h.StateHash() != hash || info.Content.Plugins[0] != "test" {
		t.Fatalf("wrong frozen state/header: %v", err)
	}
	blobs, _ := game.TakeLoadedModState(h)
	if len(blobs) != 1 || !bytes.Equal(blobs[0].Data, []byte{1, 2, 3}) {
		t.Fatal("provider bytes changed")
	}
	later, _, err := q.store.Load(rs[1].Path)
	if err != nil || later.Tick() != lastTick {
		t.Fatalf("latest coalesced tick lost: %v", err)
	}
	if provider.calls != 4 {
		t.Fatalf("unexpected provider calls: %d", provider.calls)
	}
	q.Close()
	if q.spare != nil || q.canonical != nil || q.jobs != nil {
		t.Fatal("closed queue retained worker storage")
	}
}

func TestQueueWorkerEncodeFailurePreservesSlot(t *testing.T) {
	q := NewQueue(Store{Dir: t.TempDir()})
	defer q.Close()
	if err := q.put("a", raw("good")); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	good := q.Done()
	if len(good) != 1 || good[0].Err != nil {
		t.Fatalf("initial save: %+v", good)
	}
	want, err := os.ReadFile(good[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if err = q.put("a", func(string) (savecanon.Frozen, error) { return savecanon.Freeze(make(chan int)), nil }); err != nil {
		t.Fatalf("export unexpectedly failed: %v", err)
	}
	if err = q.SaveView("a", ViewFile{CX: 42}); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	failed := q.Done()
	if len(failed) != 1 || failed[0].Err == nil || failed[0].Path != "" {
		t.Fatalf("encode failure not reported: %+v", failed)
	}
	got, err := os.ReadFile(good[0].Path)
	if err != nil || !bytes.Equal(got, want) || exists(t, q.store.Dir+"/a"+ViewExt) {
		t.Fatalf("failed encoding changed valid slot or sidecar: %v", err)
	}
}

func TestQueueCloseDuringCapture(t *testing.T) {
	q := NewQueue(Store{Dir: t.TempDir()})
	started, gate := make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- q.put("a", func(string) (savecanon.Frozen, error) { close(started); <-gate; return savecanon.Freeze("late"), nil })
	}()
	<-started
	q.Close()
	close(gate)
	if err := <-result; !errors.Is(err, ErrClosed) {
		t.Fatalf("capture after Close: %v", err)
	}
	if exists(t, q.store.Dir+"/a"+Ext) || q.Busy() || len(q.Done()) != 0 {
		t.Fatal("closed queue accepted a late capture")
	}
}

func TestQueueDeleteDuringCapture(t *testing.T) {
	q := NewQueue(Store{Dir: t.TempDir()})
	defer q.Close()
	if err := q.put("a", raw("old")); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	q.Done()
	started, gate := make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- q.put("a", func(string) (savecanon.Frozen, error) { close(started); <-gate; return savecanon.Freeze("later"), nil })
	}()
	<-started
	// A save's enqueue is its linearization point. Delete may finish
	// while the caller is still freezing the next save.
	if err := q.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if exists(t, q.store.Dir+"/a"+Ext) {
		t.Fatal("delete did not complete")
	}
	close(gate)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	q.Wait()
	rs := q.Done()
	if len(rs) != 1 || rs[0].Err != nil || !exists(t, q.store.Dir+"/a"+Ext) {
		t.Fatalf("save after delete: %+v", rs)
	}
}

// Exercise the actual queue on a migrated historical multi-map colony, not
// just the frozen-value API. Its captured view and both map states must
// survive changes to the main pawn and a secondary map before worker encode.
func TestQueueHistoricalMultiMapCapture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "application", "game", "testdata", "v12.sav"))
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := game.Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	maps := g.Maps()
	if len(maps) < 2 {
		t.Fatal("historical fixture no longer has two maps")
	}
	g.ViewMap(maps[0].ID)
	pawns := g.Snapshot()
	if len(pawns) == 0 {
		t.Fatal("missing main-map pawn")
	}
	stamp := savefmt.Stamp{Name: "historic", Colony: "two-maps", Build: "freeze"}
	var raw, want bytes.Buffer
	if err := game.EncodeSaveAs(g, &raw, stamp); err != nil {
		t.Fatal(err)
	}
	if err := savefmt.Compress(&want, raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	hash, tick, view := g.StateHash(), g.Tick(), g.ViewedMap()
	q := NewQueue(Store{Dir: t.TempDir()})
	defer q.Close()
	entered, gate := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	defer release()
	q.encode = func(f savecanon.Frozen, buf []byte) (savecanon.Captured, error) {
		close(entered)
		<-gate
		return f.Encode(buf)
	}
	if err := q.SaveAs(g, stamp); err != nil {
		t.Fatal(err)
	}
	<-entered
	g.InjureCut(pawns[0].ID, 3)
	g.Step(30)
	g.ViewMap(maps[1].ID)
	if !g.SetGround(1, 1, 1, 1, "rock", "slate", "") {
		t.Fatal("secondary terrain mutation failed")
	}
	if _, err := g.SpawnStack(item.Wood, 9, 2, 2); err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 0, Y: 0, X2: 3, Y2: 3})
	release()
	g.Step(60)
	q.Wait()
	rs := q.Done()
	if len(rs) != 1 || rs[0].Err != nil {
		t.Fatalf("queue results: %+v", rs)
	}
	got, err := os.ReadFile(rs[0].Path)
	if err != nil || !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("historical capture changed: %v", err)
	}
	h, _, err := game.Load(bytes.NewReader(got))
	if err != nil || h.StateHash() != hash || h.Tick() != tick || h.ViewedMap() != view || len(h.Maps()) != len(maps) {
		t.Fatalf("incorrect historical map capture: %v", err)
	}
	reference, _, err := game.Load(bytes.NewReader(want.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for range 6 {
		h.Step(100)
		reference.Step(100)
		if h.StateHash() != reference.StateHash() {
			t.Fatal("historical continuation diverged")
		}
	}
}
