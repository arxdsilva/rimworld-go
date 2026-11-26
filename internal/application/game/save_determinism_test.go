// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"os/exec"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/sim/statehash"
	"strings"
	"testing"
)

// The same state saves to the same bytes: no Go map order, no type numbering
// of the process in the file.
func TestSaveBytesAreDeterministic(t *testing.T) {
	t.Parallel()
	g := busyColony(t)
	var first bytes.Buffer
	if err := g.Save(&first, "colony"); err != nil {
		t.Fatal(err)
	}
	for i := range 12 {
		var again bytes.Buffer
		if err := g.Save(&again, "colony"); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first.Bytes(), again.Bytes()) {
			t.Fatalf("save %d differs from the first (%d vs %d bytes)", i+2, again.Len(), first.Len())
		}
	}
	var raw1, raw2 bytes.Buffer
	if err := EncodeSave(g, &raw1, "colony"); err != nil {
		t.Fatal(err)
	}
	if err := EncodeSave(g, &raw2, "colony"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw1.Bytes(), raw2.Bytes()) {
		t.Fatal("raw gob streams differ")
	}
}

func TestLoadThenSaveIsIdentical(t *testing.T) {
	t.Parallel()
	g := busyColony(t)
	var a bytes.Buffer
	if err := g.Save(&a, "colony"); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := Load(bytes.NewReader(a.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := loaded.Save(&b, "colony"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatalf("load then save changed the bytes (%d vs %d)", a.Len(), b.Len())
	}
}

// Both maps, their distinct store rosters and the currently viewed map
// must survive a byte-identical save/load/save cycle.
func TestCanonicalSaveTwoMaps(t *testing.T) {
	t.Parallel()
	g, _, _, _, _, _ := twoMaps(t)
	g.ViewMap(1)
	var a, b bytes.Buffer
	if err := g.Save(&a, "colony"); err != nil {
		t.Fatal(err)
	}
	before := g.mapState
	if err := g.Save(&b, "colony"); err != nil {
		t.Fatal(err)
	}
	if g.mapState != before || !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("two-map save changed map selection or varied its bytes")
	}
	h, _, err := Load(bytes.NewReader(a.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	var replay bytes.Buffer
	if err := h.Save(&replay, "colony"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), replay.Bytes()) {
		t.Fatalf("two-map load/save varied: original %d bytes, resaved %d", a.Len(), replay.Len())
	}
}

// Fresh test processes preallocate different Gob type IDs, then save an
// equivalent two-map colony with reverse insertion order of composite keys.
func TestCanonicalSaveAcrossProcesses(t *testing.T) {
	const child = "RGO_CANONICAL_SAVE_CHILD"
	if order := os.Getenv(child); order != "" {
		type first struct{ A map[[2]int][]uint64 }
		type second struct {
			B []struct {
				N int
				T string
			}
		}
		values := []any{first{}, second{}}
		if order == "reverse" {
			values[0], values[1] = values[1], values[0]
		}
		enc := gob.NewEncoder(io.Discard)
		for _, v := range values {
			if err := enc.Encode(v); err != nil {
				t.Fatal(err)
			}
		}
		g, _, _, _, _, _ := twoMaps(t)
		keys := [][2]int{{8, 4}, {4, 8}, {6, 4}}
		if order == "reverse" {
			keys[0], keys[2] = keys[2], keys[0]
		}
		g.mines = make(map[[2]int]bool)
		for _, k := range keys {
			g.mines[k] = true
		}
		var raw, compressed, again bytes.Buffer
		if err := EncodeSave(g, &raw, "canonical-child"); err != nil {
			t.Fatal(err)
		}
		if err := g.Save(&compressed, "canonical-child"); err != nil {
			t.Fatal(err)
		}
		frozen, err := FreezeSaveAs(g, savefmt.Stamp{Name: "canonical-child"})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(frozenRaw(t, frozen), raw.Bytes()) {
			t.Fatal("frozen process canonical bytes differ")
		}
		d := gob.NewDecoder(bytes.NewReader(raw.Bytes()))
		var info SaveInfo
		var state saveState
		if err := d.Decode(&info); err != nil {
			t.Fatal(err)
		}
		if err := d.Decode(&state); err != nil {
			t.Fatal(err)
		}
		info.Name = "statehash"
		if got, want := statehash.OfFields(normalisedFields(info, &state)), g.StateHash(); got != want {
			t.Fatalf("direct=%x decoded=%x", want, got)
		}
		loaded, _, err := Load(bytes.NewReader(compressed.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if err := loaded.Save(&again, "canonical-child"); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(compressed.Bytes(), again.Bytes()) {
			t.Fatal("child load/save changed canonical bytes")
		}
		fmt.Printf("CANONICAL %x %x %016x\n", sha256.Sum256(raw.Bytes()), sha256.Sum256(compressed.Bytes()), g.StateHash())
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var previous string
	for _, order := range []string{"forward", "reverse", "forward"} {
		cmd := exec.Command(exe, "-test.run=^TestCanonicalSaveAcrossProcesses$", "-test.count=1") // #nosec G204 -- re-exec of this test binary
		cmd.Env = append(os.Environ(), child+"="+order)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("child %s: %v\n%s", order, err, out)
		}
		var signature string
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "CANONICAL ") {
				signature = line
			}
		}
		if signature == "" || previous != "" && signature != previous {
			t.Fatalf("process %s produced %q, want %q", order, signature, previous)
		}
		previous = signature
		t.Log(order, signature)
	}
}
