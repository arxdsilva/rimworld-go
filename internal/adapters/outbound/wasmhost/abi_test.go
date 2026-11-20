// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"bytes"
	"encoding/binary"
	"testing"

	"rimworld-go/internal/application/scripting"
)

func TestABIEventLayout(t *testing.T) {
	ev := scripting.Event{Hook: scripting.OnIncident, Tick: 0x0102030405, Incident: scripting.Incident{Key: "k.x", Category: "nature"}}
	pawns := []scripting.Pawn{{ID: 5, X: -1, Y: 2, Food: 3, Rest: 4, Mood: -5, Downed: true, Drafted: true}}
	b := appendEvent(nil, ev, pawns)
	if len(b) != eventSize(ev, 1) || len(b) != 32+32+3+6 {
		t.Fatalf("len = %d, eventSize = %d", len(b), eventSize(ev, 1))
	}
	le := binary.LittleEndian
	if le.Uint32(b[0:]) != ABIVersion || le.Uint32(b[4:]) != uint32(scripting.OnIncident) || le.Uint64(b[8:]) != 0x0102030405 ||
		le.Uint32(b[16:]) != 1 || le.Uint32(b[20:]) != 3 || le.Uint32(b[24:]) != 6 || le.Uint32(b[28:]) != 0 {
		t.Errorf("header = %x", b[:32])
	}
	rec := b[32:64]
	if le.Uint64(rec) != 5 || int32(le.Uint32(rec[8:])) != -1 || le.Uint32(rec[12:]) != 2 || le.Uint32(rec[16:]) != 3 ||
		le.Uint32(rec[20:]) != 4 || int32(le.Uint32(rec[24:])) != -5 || le.Uint32(rec[28:]) != 3 {
		t.Errorf("record = %x", rec)
	}
	if !bytes.Equal(b[64:], []byte("k.xnature")) {
		t.Errorf("texts = %q", b[64:])
	}
	d := appendEvent(nil, scripting.Event{Hook: scripting.OnColonistDied, Name: "Ann"}, nil)
	if le.Uint32(d[20:]) != 3 || le.Uint32(d[24:]) != 0 || string(d[32:]) != "Ann" {
		t.Errorf("died = %x", d)
	}
	if h := appendEvent(nil, scripting.Event{Hook: scripting.OnHour}, nil); len(h) != 32 {
		t.Errorf("on_hour carries %d bytes, want the 32 byte header", len(h))
	}
}

func TestABIConfigDefaultsAndEngines(t *testing.T) {
	o := Options{}.withDefaults()
	if o.Fuel != DefaultFuel || o.MaxPages != DefaultMaxPages {
		t.Errorf("defaults = %+v", o)
	}
	for _, e := range []Engine{Auto, Compiler, Interpreter} {
		if runtimeConfig(Options{Engine: e}.withDefaults()) == nil {
			t.Errorf("engine %d has no config", e)
		}
	}
}
