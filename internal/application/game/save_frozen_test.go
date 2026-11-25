// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"os"
	"reflect"
	"runtime"
	"testing"

	"rimworld-go/internal/application/game/savecanon"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/sim/command"
)

// Reflection is used only by this ownership audit, never to capture or
// serialize a save. Compare all populated references, including fields Gob
// ignores. Strings and zero-size storage are immutable.
type saveReference struct {
	kind      reflect.Kind
	ptr, size uintptr
}

func saveReferences(v reflect.Value, path string, refs map[saveReference]string) {
	switch v.Kind() {
	case reflect.Interface:
		if !v.IsNil() {
			saveReferences(v.Elem(), path, refs)
		}
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if v.IsNil() {
			return
		}
		key := saveReference{kind: v.Kind(), ptr: v.Pointer(), size: 1}
		if v.Kind() == reflect.Slice {
			key.size = uintptr(v.Cap()) * v.Type().Elem().Size()
			if key.size == 0 {
				return
			}
		} else if v.Kind() == reflect.Pointer {
			key.size = v.Type().Elem().Size()
			if key.size == 0 {
				return
			}
		}
		if _, ok := refs[key]; ok {
			return
		}
		refs[key] = path
		switch v.Kind() {
		case reflect.Pointer:
			saveReferences(v.Elem(), path, refs)
		case reflect.Map:
			it := v.MapRange()
			for it.Next() {
				saveReferences(it.Key(), path+"{key}", refs)
				saveReferences(it.Value(), path+"{}", refs)
			}
		case reflect.Slice:
			// The visible prefix is encoded; also inspect the capacity so that a
			// short slice cannot accidentally retain an ignored live payload.
			full := v.Slice(0, v.Cap())
			for i := 0; i < full.Len(); i++ {
				saveReferences(full.Index(i), path+"[]", refs)
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			saveReferences(v.Field(i), path+"."+v.Type().Field(i).Name, refs)
		}
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			saveReferences(v.Index(i), path+"[]", refs)
		}
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		if v.Pointer() != 0 {
			refs[saveReference{v.Kind(), v.Pointer(), 1}] = path
		}
	}
}

func assertDetached(t *testing.T, borrowedInfo SaveInfo, borrowed *saveState, ownedInfo SaveInfo, owned *saveState, live *Game) {
	t.Helper()
	oldRefs, newRefs := make(map[saveReference]string), make(map[saveReference]string)
	saveReferences(reflect.ValueOf(borrowedInfo), "Info", oldRefs)
	saveReferences(reflect.ValueOf(borrowed), "State", oldRefs)
	saveReferences(reflect.ValueOf(live), "Live", oldRefs)
	saveReferences(reflect.ValueOf(ownedInfo), "Info", newRefs)
	saveReferences(reflect.ValueOf(owned), "State", newRefs)
	for a, path := range newRefs {
		if a.kind == reflect.Func || a.kind == reflect.Chan || a.kind == reflect.UnsafePointer {
			t.Errorf("captured runtime reference: %s", path)
			continue
		}
		for b, source := range oldRefs {
			if a.kind == reflect.Map || b.kind == reflect.Map {
				if a.kind != b.kind || a.ptr != b.ptr {
					continue
				}
			} else if a.ptr >= b.ptr+b.size || b.ptr >= a.ptr+a.size {
				continue
			}
			t.Errorf("capture shares mutable storage: %s -> %s", path, source)
			break
		}
	}
	runtime.KeepAlive(borrowed)
	runtime.KeepAlive(borrowedInfo)
	runtime.KeepAlive(owned)
	runtime.KeepAlive(ownedInfo)
	runtime.KeepAlive(live)
	t.Logf("audited %d owned and %d borrowed reference ranges", len(newRefs), len(oldRefs))
}

func frozenRaw(t *testing.T, f savecanon.Frozen) []byte {
	t.Helper()
	c, err := f.Encode(nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := c.CanonicalBytes(nil)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func auditFrozen(t *testing.T, g *Game) {
	t.Helper()
	src := &countingSource{blobs: twoBlobs()}
	UseModState(g, src)
	stamp := savefmt.Stamp{Name: "capture", Colony: "audit", Content: savefmt.Content{Plugins: []string{"audit"}}}
	ai, a, err := snapshotForSave(g, stamp, src)
	if err != nil {
		t.Fatal(err)
	}
	bi, b, err := snapshotForSaveMode(g, stamp, src, true)
	if err != nil {
		t.Fatal(err)
	}
	assertDetached(t, ai, a, bi, b, g)
	var want bytes.Buffer
	if err := EncodeSaveAs(g, &want, stamp); err != nil {
		t.Fatal(err)
	}
	// Snapshot is exactly the existing wire values, with no second schema.
	if got := frozenRaw(t, savecanon.Freeze(bi, b)); !bytes.Equal(got, want.Bytes()) {
		t.Fatal("owned snapshot changed canonical bytes")
	}
}

func TestFrozenSaveOwnership(t *testing.T) {
	t.Run("grown", func(t *testing.T) { auditFrozen(t, busyColony(t)) })
	t.Run("two-maps", func(t *testing.T) { g, _, _, _, _, _ := twoMaps(t); auditFrozen(t, g) })
	t.Run("market", func(t *testing.T) {
		g := worldGame(t)
		tile := nearDest(t, g)
		pawn := g.pawnIDs[0]
		cid := visitingMarket(t, g, tile, pawn)
		travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 800})
		travelOf(g).Carry(cid, item.Stack{Kind: item.Parka, Qty: 1, Stuff: item.Cloth, Quality: 4, Wear: 10})
		g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: trade.EncodeOffer([]trade.Delta{{Kind: item.Parka, Qty: -1}})})
		g.ApplyPending()
		auditFrozen(t, g)
	})
	t.Run("local-trader", func(t *testing.T) { g, _, _, _, _, _, _ := paidSwordCaravan(t); auditFrozen(t, g) })
	for _, name := range []string{"v12", "upstream-v20-ff37", "historical-v20-offmap-construction", "historical-v20-prosthetic-active", "historical-v20-pending-floor-swap"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + name + ".sav")
			if err != nil {
				t.Fatal(err)
			}
			g, _, err := Load(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			auditFrozen(t, g)
		})
	}
}

func TestFrozenSaveCaptureTick(t *testing.T) {
	g, _, _, stack, _, deer := twoMaps(t)
	src := &countingSource{blobs: twoBlobs()}
	UseModState(g, src)
	stamp := savefmt.Stamp{Name: "before", Colony: "frozen", Build: "audit", Content: savefmt.Content{Plugins: []string{"tally", "alpha"}}}
	var want bytes.Buffer
	if err := EncodeSaveAs(g, &want, stamp); err != nil {
		t.Fatal(err)
	}
	beforeTick, hash := g.Tick(), g.StateHash()
	f, err := FreezeSaveAs(g, stamp)
	if err != nil {
		t.Fatal(err)
	}
	if src.calls != 2 {
		t.Fatalf("provider called %d times, want 2", src.calls)
	}
	// Mutate provider and header buffers as well as the extra map before
	// allowing serialization to start. No worker ever reads the provider.
	src.blobs[0].Data[0] ^= 0xff
	stamp.Content.Plugins[0] = "changed"
	g.ViewMap(1)
	st, _ := g.stacks.Get(stack)
	st.Qty++
	g.putStack(stack, st)
	ev := g.exposure[deer]
	ev.Hypo++
	g.exposure[deer] = ev
	g.SetGround(1, 1, 2, 1, "rock", "slate", "")
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 0, Y: 0, X2: 3, Y2: 3})
	ready, done := make(chan struct{}), make(chan struct{})
	var c savecanon.Captured
	go func() { close(ready); c, err = f.Encode(nil); close(done) }()
	<-ready
	for i := 0; i < 60; i++ {
		g.Step(1)
	}
	<-done
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.CanonicalBytes(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want.Bytes()) {
		t.Fatal("capture-time values changed after simulation resumed")
	}
	var a, b bytes.Buffer
	if err = savefmt.Compress(&a, want.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err = savefmt.Compress(&b, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("gzip changed")
	}
	x, info, err := Load(&a)
	if err != nil {
		t.Fatal(err)
	}
	y, _, err := Load(&b)
	if err != nil {
		t.Fatal(err)
	}
	if x.Tick() != beforeTick || y.Tick() != beforeTick || x.StateHash() != hash || y.StateHash() != hash || info.Content.Plugins[0] != "tally" {
		t.Fatal("captured tick/hash/header changed")
	}
	if src.calls != 2 {
		t.Fatal("worker or StateHash queried provider")
	}
	for i := 0; i < 6; i++ {
		x.Step(100)
		y.Step(100)
		if x.StateHash() != y.StateHash() {
			t.Fatalf("continuation diverged at %d", x.Tick())
		}
	}
}
