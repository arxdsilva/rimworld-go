// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modstate_test

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"rimworld-go/internal/application/game/modstate"
)

func blob(plugin, file string, n int) modstate.Blob {
	return modstate.Blob{Plugin: plugin, File: file, Data: make([]byte, n)}
}

// keys lists the (plugin, file) pairs of bs, in order.
func keys(bs []modstate.Blob) []string {
	var out []string
	for _, b := range bs {
		out = append(out, b.Plugin+"/"+b.File)
	}
	return out
}

func noteKeys(ns []modstate.Note) []string {
	var out []string
	for _, n := range ns {
		if n.Msg == "" {
			out = append(out, "EMPTY MSG "+n.Plugin+"/"+n.File)
			continue
		}
		out = append(out, n.Plugin+"/"+n.File)
	}
	return out
}

// TestCheck: every drop rule notes the blob it drops, and the kept blobs
// keep their order.
func TestCheck(t *testing.T) {
	long := strings.Repeat("x", modstate.MaxName+1)
	edge := strings.Repeat("y", modstate.MaxName)
	many := make([]modstate.Blob, modstate.MaxBlobs+2)
	for i := range many {
		many[i] = blob("p", strconv.Itoa(i)+".wasm", 1)
	}
	for _, c := range []struct {
		name        string
		in          []modstate.Blob
		kept, noted []string
	}{
		{"nil", nil, nil, nil},
		{"kept in order", []modstate.Blob{blob("b", "2.wasm", 1), blob("a", "1.wasm", 0), blob("b", "1.wasm", 3)},
			[]string{"b/2.wasm", "a/1.wasm", "b/1.wasm"}, nil},
		{"empty plugin", []modstate.Blob{blob("", "f.wasm", 1), blob("a", "f.wasm", 1)}, []string{"a/f.wasm"}, []string{"/f.wasm"}},
		{"empty file", []modstate.Blob{blob("a", "", 1), blob("a", "f.wasm", 1)}, []string{"a/f.wasm"}, []string{"a/"}},
		{"long plugin, clipped in its note", []modstate.Blob{blob(long, "f", 1), blob(edge, "f", 1)}, []string{edge + "/f"}, []string{long[:modstate.MaxName] + "/f"}},
		{"long file, clipped in its note", []modstate.Blob{blob("a", long, 1), blob("a", edge, 1)}, []string{"a/" + edge}, []string{"a/" + long[:modstate.MaxName]}},
		{"data too big", []modstate.Blob{blob("a", "big", modstate.MaxData+1), blob("a", "max", modstate.MaxData)},
			[]string{"a/max"}, []string{"a/big"}},
		{"duplicate keeps the first", []modstate.Blob{blob("a", "f", 1), blob("b", "f", 1), blob("a", "f", 2)},
			[]string{"a/f", "b/f"}, []string{"a/f"}},
		{"bad blob does not claim its key", []modstate.Blob{blob("a", "f", modstate.MaxData+1), blob("a", "f", 1)},
			[]string{"a/f"}, []string{"a/f"}},
		{"past MaxBlobs", many, keys(many[:modstate.MaxBlobs]), keys(many[modstate.MaxBlobs:])},
		{"notes bounded", append(slices.Repeat([]modstate.Blob{blob("", "f", 1)}, modstate.MaxNotes-1), blob("", "g", 1), blob("", "h", 1)),
			nil, append(slices.Repeat([]string{"/f"}, modstate.MaxNotes-1), "/")},
		{"MaxBlobs counts kept blobs", append([]modstate.Blob{blob("", "bad", 1)}, many[:modstate.MaxBlobs]...),
			keys(many[:modstate.MaxBlobs]), []string{"/bad"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			kept, notes := modstate.Check(c.in)
			if got := keys(kept); !reflect.DeepEqual(got, c.kept) {
				t.Errorf("kept %q, want %q", got, c.kept)
			}
			if got := noteKeys(notes); !reflect.DeepEqual(got, c.noted) {
				t.Errorf("noted %q, want %q", got, c.noted)
			}
		})
	}
}

// TestCheckKeepsTheFirstDuplicate: of two blobs for one file, the first
// one's data is kept.
func TestCheckKeepsTheFirstDuplicate(t *testing.T) {
	kept, _ := modstate.Check([]modstate.Blob{
		{Plugin: "a", File: "f", Data: []byte{1}},
		{Plugin: "a", File: "f", Data: []byte{2}},
	})
	if len(kept) != 1 || kept[0].Data[0] != 1 {
		t.Fatalf("kept %v, want the first blob", kept)
	}
}

// TestCheckLeavesTheInput: Check never writes to its argument.
func TestCheckLeavesTheInput(t *testing.T) {
	in := []modstate.Blob{blob("a", "f", modstate.MaxData+1), blob("a", "g", 1)}
	want := keys(in)
	modstate.Check(in)
	if got := keys(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("input became %q, want %q", got, want)
	}
}

// TestNotesClipNamesOnRuneBoundaries: a clipped name in a note stays valid
// UTF-8 and within MaxName bytes.
func TestNotesClipNamesOnRuneBoundaries(t *testing.T) {
	name := "a" + strings.Repeat("é", modstate.MaxName) // 2-byte runes from byte 1
	_, notes := modstate.Check([]modstate.Blob{blob(name, name, 1)})
	if len(notes) != 1 {
		t.Fatalf("notes %d, want 1", len(notes))
	}
	for _, s := range []string{notes[0].Plugin, notes[0].File} {
		if len(s) > modstate.MaxName || !utf8.ValidString(s) || !strings.HasPrefix(name, s) || len(s) < modstate.MaxName-1 {
			t.Fatalf("clipped to %d bytes (valid %v), want a valid prefix of at most %d", len(s), utf8.ValidString(s), modstate.MaxName)
		}
	}
}

// TestAppendNotesBounded: notes from many saves stay within MaxNotes, the
// last one saying more were dropped.
func TestAppendNotesBounded(t *testing.T) {
	var ns []modstate.Note
	long := strings.Repeat("z", 3*modstate.MaxName)
	for range 3 * modstate.MaxNotes {
		ns = modstate.AppendNotes(ns, []modstate.Note{{Plugin: "p", File: long, Msg: "dropped"}})
	}
	if len(ns) != modstate.MaxNotes {
		t.Fatalf("%d notes, want %d", len(ns), modstate.MaxNotes)
	}
	if last := ns[len(ns)-1]; last.Plugin != "" || last.File != "" || last.Msg == "" {
		t.Fatalf("last note %+v, want the unnamed more-dropped note", last)
	}
	if len(ns[0].File) != modstate.MaxName {
		t.Fatalf("appended note's file is %d bytes, want clipped to %d", len(ns[0].File), modstate.MaxName)
	}
}
