// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package saves

import (
	"strings"
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game/savefmt"
)

func tagTexts(r Row) []string {
	var out []string
	for _, t := range r.Tags {
		out = append(out, t.Text)
	}
	return out
}

// A save with the full header: the title and its tags, then the colony's
// summary; the tip holds every field.
func TestFormatRowFullHeader(t *testing.T) {
	t.Parallel()
	f, _ := newFake()
	sl := ports.SaveSlot{Name: "ridge-autosave", Title: "Ridge before winter", ColonyName: "Ridge", Autosave: true,
		Commitment: true, Day: 14, Season: "Fall", Colonists: 5, Storyteller: "Calm", Difficulty: "Rough",
		Biome: "Boreal forest", Build: "0.9.3", MapSize: 250, WorldSeed: "amber-hollow", Version: 16, Compat: int(savefmt.CompatOlder),
		Modified: time.Date(2026, 11, 9, 14, 5, 0, 0, time.UTC)}
	r := FormatRow(f, sl)
	if r.Title != "Ridge before winter" || r.Bad != "" {
		t.Fatalf("row %+v", r)
	}
	if got := strings.Join(tagTexts(r), "|"); got != "Nov 9 14:05|Older version|Commitment|Autosave" {
		t.Fatalf("tags %s", got)
	}
	want := "Ridge · Day 14, Fall · 5 colonists · Calm/Rough · Boreal forest · build 0.9.3"
	if r.Sub != want {
		t.Fatalf("line 2 %q, want %q", r.Sub, want)
	}
	lines := strings.Split(r.Tip, "\n")
	if len(lines) != 3 || lines[1] != want || !strings.HasPrefix(lines[2], "map 250 · seed amber-hollow · format 16") {
		t.Fatalf("tip lines %q", lines)
	}
	for _, s := range []string{"Ridge before winter", "file ridge-autosave", "Autosave", "Older version", want, "map 250 · seed amber-hollow", "format 16"} {
		if !strings.Contains(r.Tip, s) {
			t.Fatalf("tip %q lacks %q", r.Tip, s)
		}
	}
}

// An older header has no day, season or colony: the row leaves them out
// (never "Day 0") and keeps the date line it had.
func TestFormatRowOldHeader(t *testing.T) {
	t.Parallel()
	f, _ := newFake()
	r := FormatRow(f, ports.SaveSlot{Name: "colony-1", Colonists: 1})
	if r.Title != "colony-1" || r.Sub != "1 colonist" || len(r.Tags) != 0 {
		t.Fatalf("row %+v", r)
	}
	if strings.Contains(r.Tip, "seed") {
		t.Fatalf("an older header's tip shows a seed: %q", r.Tip)
	}
	r = FormatRow(f, ports.SaveSlot{Name: "c", Date: "Year 1, Quadrum 2, Day 3", Colonists: 3})
	if r.Sub != "Year 1, Quadrum 2, Day 3 · 3 colonists" || strings.Contains(r.Sub, "Day 0") {
		t.Fatalf("line 2 %q", r.Sub)
	}
}

// A seed shows even when the map size is unknown.
func TestFormatRowSeedWithoutMapSize(t *testing.T) {
	t.Parallel()
	f, _ := newFake()
	r := FormatRow(f, ports.SaveSlot{Name: "c", WorldSeed: "amber-hollow"})
	if !strings.Contains(r.Tip, "seed amber-hollow") || strings.Contains(r.Tip, "map") {
		t.Fatalf("tip %q", r.Tip)
	}
}

// A save that cannot load says why in red: its error, or a newer build.
func TestFormatRowBad(t *testing.T) {
	t.Parallel()
	f, _ := newFake()
	if r := FormatRow(f, ports.SaveSlot{Name: "x", Err: "damaged file"}); r.Bad != "cannot load: damaged file" || !strings.Contains(r.Tip, r.Bad) {
		t.Fatalf("row %+v", r)
	}
	r := FormatRow(f, ports.SaveSlot{Name: "x", Compat: int(savefmt.CompatNewer)})
	if r.Bad != "Made with a newer version (build unknown). Update the game to load it." {
		t.Fatalf("bad %q", r.Bad)
	}
	if r := FormatRow(f, ports.SaveSlot{Name: "x", Compat: int(savefmt.CompatContent), Notice: "a pack is missing"}); r.Bad != "" || !strings.Contains(r.Tip, "a pack is missing") {
		t.Fatalf("content row %+v", r)
	}
}

// The seed comes from a file: control and format runes are dropped, a long
// one is cut at 32 runes with an ellipsis, and nothing left means no part.
func TestFormatRowSeedIsSanitised(t *testing.T) {
	t.Parallel()
	f, _ := newFake()
	long := strings.Repeat("ab", 40)
	for _, c := range []struct{ in, want string }{
		{"amber\u202Ehollow\n\x00\u200b", "seed amberhollow"},
		{long, "seed " + long[:32] + "…"},
		{long[:32], "seed " + long[:32]},
		{"\u202e\x07\u200d\n", ""},
	} {
		tip := FormatRow(f, ports.SaveSlot{Name: "c", WorldSeed: c.in}).Tip
		if c.want == "" {
			if strings.Contains(tip, "seed") {
				t.Fatalf("tip %q for %q", tip, c.in)
			}
			continue
		}
		if !strings.Contains(tip, c.want) || strings.ContainsAny(tip, "\u202e\u200b\x00") || strings.Count(tip, "\n") != 2 || strings.Contains(tip, long[:33]) {
			t.Fatalf("tip %q lacks %q", tip, c.want)
		}
	}
}
