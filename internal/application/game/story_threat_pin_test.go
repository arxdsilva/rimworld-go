// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"hash/fnv"
	"testing"

	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/storyteller"
)

// pinCase is one storyteller-threat situation of the pin: the season and
// colony day the threats fire on, the colony's threat flags, a Dice
// double, a rich colony (threat points >= 300, so the raid face's mech
// branch is in reach).
type pinCase struct {
	name      string
	season    string
	day       int
	noHostile bool
	noBig     bool
	dice      bool
	rich      bool
}

// pinThreatCases are the pinned situations, in a fixed order.
var pinThreatCases = [...]pinCase{
	{name: "all-winter-30", season: "Winter", day: 30},
	{name: "all-summer-9", season: "Summer", day: 9},
	{name: "nohostile-winter-3", season: "Winter", day: 3, noHostile: true},
	{name: "nohostile-winter-9", season: "Winter", day: 9, noHostile: true},
	{name: "nohostile-summer-3", season: "Summer", day: 3, noHostile: true},
	{name: "nohostile-summer-9", season: "Summer", day: 9, noHostile: true},
	{name: "nobig-winter-3", season: "Winter", day: 3, noBig: true},
	{name: "nobig-winter-9", season: "Winter", day: 9, noBig: true},
	{name: "nobig-summer-3", season: "Summer", day: 3, noBig: true},
	{name: "nobig-summer-9", season: "Summer", day: 9, noBig: true},
	{name: "nohostile-winter-30", season: "Winter", day: 30, noHostile: true},
	{name: "nobig-summer-30", season: "Summer", day: 30, noBig: true},
	{name: "dice-winter-30", season: "Winter", day: 30, dice: true},
	{name: "rich-summer-30", season: "Summer", day: 30, rich: true},
}

// pinThreatRun fires 24 storyteller threats for case c on a colony of
// seed and returns a hash of every message they posted (title and text),
// the message count and one draw of the main stream after the run; titles
// tallies the letters' titles (for a failure's report).
func pinThreatRun(t *testing.T, seed uint64, c pinCase) (hash uint64, n, draw int, titles map[string]int) {
	t.Helper()
	g := buildSetup(t, setupFor(t, "ember_clan", func(s *Setup) { s.Seed = seed }))
	stepUntilLanded(t, g)
	if c.rich {
		g.ng.setup.Preset, g.ng.setup.Difficulty = difficulty.Merciless, difficulty.Of(difficulty.Merciless)
		if _, err := g.SpawnStack(item.Silver, 50000, 0, 0); err != nil {
			t.Fatal(err)
		}
	}
	if c.noHostile {
		hostileOnly(g.ng.factions, -1)
	} else if !hostileRaider(g) {
		t.Fatalf("%s: no hostile raider faction", c.name)
	}
	if c.noBig {
		g.ng.setup.Difficulty.BigThreats = false
	}
	// an absolute day in the wanted season at least c.day after day 0;
	// the landing is moved so that it is colony day c.day
	abs := -1
	for d := c.day; d < c.day+120; d++ {
		if g.season(uint64(d)*clock.TicksPerDay) == c.season {
			abs = d
			break
		}
	}
	if abs < 0 {
		t.Fatalf("%s: no %s day", c.name, c.season)
	}
	now := uint64(abs)*clock.TicksPerDay + clock.TicksPerDay/2
	g.ng.startDay = abs - c.day
	g.clock = clock.NewAt(clock.Tick(now))
	if g.relDay(now) != c.day || g.season(now) != c.season {
		t.Fatalf("%s: day %d season %s", c.name, g.relDay(now), g.season(now))
	}
	if c.rich && g.threatPoints() < 300 {
		t.Fatalf("%s: threat points %d, want >= 300", c.name, g.threatPoints())
	}
	h := fnv.New64a()
	titles = map[string]int{}
	for i := 0; i < 24; i++ {
		g.messages = nil
		if c.dice {
			g.ng.plan.Second = storyteller.Raid
		} else {
			g.ng.plan.Second = storyteller.None
		}
		g.fireStoryThreat(now)
		for _, m := range g.messages {
			fmt.Fprintf(h, "%d|%v|%s|%s\n", i, m.Letter, m.Title, m.Text)
			n++
			if m.Letter {
				titles[m.Title]++
			}
		}
	}
	return h.Sum64(), n, g.rng.IntN(1 << 20), titles
}

// TestStoryThreatSequencePinned pins a setup colony's storyteller threats
// (what they posted and the main stream after them), captured on the code
// before the threats went through the incident framework (#12): seeds 1-3
// across seasons, colony days and threat flags, a Dice double and a rich
// colony.
func TestStoryThreatSequencePinned(t *testing.T) {
	t.Parallel()
	type pinned struct {
		hash uint64
		msgs int
		draw int
	}
	want := map[string]pinned{ // captured at 52b6650f1
		"1/all-summer-9":        {0xc036260e468c70cc, 26, 383139},
		"1/all-winter-30":       {0x15331c5eba87d220, 25, 917816},
		"1/dice-winter-30":      {0x175dd87789c72db2, 53, 259674},
		"1/nobig-summer-30":     {0x5d6473daf12a7ae8, 23, 127754},
		"1/nobig-summer-3":      {0xedfdc8ba0b40ca3b, 24, 954717},
		"1/nobig-summer-9":      {0xabef8d1615ddd09f, 24, 655592},
		"1/nobig-winter-3":      {0xd38266b127169903, 24, 144097},
		"1/nobig-winter-9":      {0x481cfd05e6ef82a0, 24, 397118},
		"1/nohostile-summer-3":  {0xedfdc8ba0b40ca3b, 24, 954717},
		"1/nohostile-summer-9":  {0xabef8d1615ddd09f, 24, 655592},
		"1/nohostile-winter-30": {0x189ace749ce8265b, 22, 270290},
		"1/nohostile-winter-3":  {0xd38266b127169903, 24, 144097},
		"1/nohostile-winter-9":  {0x481cfd05e6ef82a0, 24, 397118},
		"1/rich-summer-30":      {0x704193d9550c2d4d, 26, 966345},
		"2/all-summer-9":        {0xfdd60fb6d0718062, 26, 563104},
		"2/all-winter-30":       {0xb583c3a9f55eb404, 25, 781832},
		"2/dice-winter-30":      {0xbe0e44d84aff0211, 49, 1021111},
		"2/nobig-summer-30":     {0x310cbaa47ee125f3, 18, 73551},
		"2/nobig-summer-3":      {0xe18eac8e1c134f50, 24, 779273},
		"2/nobig-summer-9":      {0xb5ed6bf64a7ed0ed, 19, 838822},
		"2/nobig-winter-3":      {0x8c8c3eed6c1be830, 24, 1023663},
		"2/nobig-winter-9":      {0xfb98e8252b0b7dee, 20, 844293},
		"2/nohostile-summer-3":  {0xe18eac8e1c134f50, 24, 779273},
		"2/nohostile-summer-9":  {0xb5ed6bf64a7ed0ed, 19, 838822},
		"2/nohostile-winter-30": {0x42f8c6f71cbae2c3, 21, 887031},
		"2/nohostile-winter-3":  {0x8c8c3eed6c1be830, 24, 1023663},
		"2/nohostile-winter-9":  {0xfb98e8252b0b7dee, 20, 844293},
		"2/rich-summer-30":      {0x4221f69aeb37c071, 26, 922498},
		"3/all-summer-9":        {0x189344a47825d680, 24, 758954},
		"3/all-winter-30":       {0x6363915298a44253, 25, 1043268},
		"3/dice-winter-30":      {0xd0b130e51439a5e, 49, 221921},
		"3/nobig-summer-30":     {0x19de20526ff10d30, 19, 1035427},
		"3/nobig-summer-3":      {0xe978b07d536edc06, 24, 524430},
		"3/nobig-summer-9":      {0x6cfb071913e80831, 23, 758954},
		"3/nobig-winter-3":      {0x75984d75b217ac70, 24, 589262},
		"3/nobig-winter-9":      {0xc8ba3410a6e11ca, 23, 944260},
		"3/nohostile-summer-3":  {0xe978b07d536edc06, 24, 524430},
		"3/nohostile-summer-9":  {0x6cfb071913e80831, 23, 758954},
		"3/nohostile-winter-30": {0x884e675bdc9502f, 21, 970251},
		"3/nohostile-winter-3":  {0x75984d75b217ac70, 24, 589262},
		"3/nohostile-winter-9":  {0xc8ba3410a6e11ca, 23, 944260},
		"3/rich-summer-30":      {0xc081b88517e5dc9d, 25, 81462},
	}
	for seed := uint64(1); seed <= 3; seed++ {
		for _, c := range pinThreatCases {
			t.Run(fmt.Sprintf("seed%d-%s", seed, c.name), func(t *testing.T) {
				t.Parallel()
				hash, msgs, draw, titles := pinThreatRun(t, seed, c)
				w, ok := want[fmt.Sprintf("%d/%s", seed, c.name)]
				if !ok || w != (pinned{hash, msgs, draw}) {
					t.Errorf("seed %d %s: got {%#x, %d, %d}, want %+v; letters %v", seed, c.name, hash, msgs, draw, w, titles)
				}
			})
		}
	}
}
