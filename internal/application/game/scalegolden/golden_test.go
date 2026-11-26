// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scalegolden

import (
	"fmt"
	"testing"

	"rimworld-go/internal/application/game"
)

// Golden state hashes (NG-1 §6.2): the complete state after 20 000 ticks
// of a generated colony, pinned before the engine-scale work (E1) so every
// behaviour-preserving optimisation is proven bit-identical. An intended
// behaviour change re-pins these, with the reason in the handoff.
//
// Re-pin: go test -race -count=1 -v ./internal/application/game/scalegolden/
// and copy the "got" values.
//
// Re-pinned (ENG-003 M3 social core): the founders get families (off-map
// relatives take entity ids, the social stream rolls), raiders and
// colonists are registered people, colonists interact every ~1250 ticks
// (opinion memories, insult and kind-words moods, a social log), idle
// chats stand both pawns together, social relaxing at tables and fires,
// and mood is smoothed (mental breaks follow the smoothed value). Three
// identical runs; taken on a tree that also carries the in-flight M1/M2
// host-system changes (4 of 6 pins had moved with social disabled).
//
// Re-pinned (ENG-003): StateHash encodes saved values sparsely (struct
// fields by name, zero ones skipped; arrays without trailing zeros), so
// the social contract's appended record fields and work slot leave pins
// alone. Encoding only: computed on f957fc8a with no other change, and
// unchanged by the contract's enum and field additions.
//
// Re-pinned (PLAY-010): animals react to every attack (fight or flee,
// penned ones inside the pen), predators charge colonists within 3 tiles,
// unarmed colonists flee from 12 tiles, colonists and tame animals walk
// through each other, and grazed grass regrows from shoots. Checked field
// by field against 4b12749d after 20 000 ticks of all six colonies: only
// Animals, Positions, Motion, Jobs, Carried, Claims, Postponed, Hits,
// Strikes, Wounds, Stains, Logs, Skills, Joy, Need, TendedUntil, Vital,
// RNG and the new GrassGrowth moved; no colonist dies (two are hurt).
//
// Re-pinned (NG-1 E4, plant_growth.go): plants grow per chunk once per
// climate interval (plant.GrowSpan, one rate per window) instead of every
// tick, and frost kills in the same pass. Checked field by field against
// f815d545 after 1, 250, 1 000, 5 000 and 20 000 ticks of all six
// colonies: every saved field is identical except Plants, and within the
// plants only Growth differs (ids, species, positions and marks equal).
//
// Re-pinned (NG-1 S3): colonists now save their bios and skills (new
// state); every other saved field is identical to the previous pins, so
// behaviour is unchanged (checked field by field against 4d68b9aa).
//
// Re-pinned (ENG-003 polish): conversation memories now land at the
// speaker's Social impact (spec §13.2), so opinions move; with the scale
// forced to 1000 the old pins hold. Stable over -count=3, with and
// without -race.
//
// Re-pinned (ENG-003 M9b): stale since 8a84be08 (M5-M10); M9b leaves
// them as they were at that commit (checked on both, -count=3).
//
// Re-pinned earlier: save format (research slice), behaviour unchanged. StateHash
// became format-independent (normalised fields) at the same time; these
// values are identical when computed on the contract commit (fbc76fd6)
// and on the new save format.
// Re-pinned (PLAY-015): new colonists start with Firefight on (priority
// 3); checked field by field on all six runs, only Prios and Labor differ.
// Re-pinned (PLAY-017): a charging or hunting animal now warns the player
// (a danger message, and the warned colonist on the animal). Checked field
// by field against c7d739ac6 on all six runs: only Animals (ChaseWarnedAt,
// ChaseWarnedTarget) and Messages (e.g. "A timber wolf charges Cora!")
// differ; seeds 7 (48x40) and 1 (120x120) see no charge and keep their pin.
// Re-pinned (PLAY-020 bed rest): a hurt colonist rests until well. The
// start colonies have no beds, so one with an untended wound lies down
// where it is until another colonist tends it (before: only one in pain
// or sick), and a hurt one finding no bed records its retry (BedRest).
// Checked field by field on 0a403eb88 plus only the PLAY-020 sim files:
// the three runs with wounded colonists move in what tending and healing
// touch (Wounds, TendedUntil, Vital, Bodies, Jobs, Positions, Motion,
// SleptAt, Logs, Need, Joy, Skills, Social, then the RNG); 48x40/seed42's
// colonist that ended downed at 69 injury is tended and healed. The three
// runs where no one is hurt keep their pins. Stable over -count=2.
// Re-pinned (MED-01 bleeding): open cuts, bites and scratches bleed until
// tended, blood loss lowers consciousness and drains back after, pain is
// 12.5 per HP whether tended or not, downing follows consciousness,
// moving and pain shock, doctors tend the fastest bleeder first, and the
// hourly medical phase runs blood loss before the infections. The three
// runs with a hurt colonist move (48x40/seed1 and seed42, 120x120/seed7:
// each ends with a colonist alive at 25, 51 and 105 permille blood loss);
// the three with no one hurt keep their pins. Same-seed runs identical
// (TestStepIsDeterministicForSameSeed, TestBodiesAreDeterministic, and
// TestScaleGolden -count=2 printed the same values).
// Re-pinned (MED-02 tend quality and immunity): tends roll a quality on the
// medical stream (no instant heal), wounds heal per wound at the wiki's
// 8 HP a day (faster tended and lying) in the hourly medical pass instead
// of a point every 300 ticks, scars are rolled on the wound's depth, and
// infection rolls come 6–18 h after the wound on the medical stream (no
// longer g.rng). The same three runs with a hurt colonist move; the three
// with no one hurt keep their pins. Same-seed runs identical
// (TestStepIsDeterministicForSameSeed, TestBodiesAreDeterministic, and
// TestScaleGolden -count=2 printed the same values).
// Re-pinned (MED-03 care policy, MED-05 organs): the body has kidneys,
// liver and spine (torso coverage 380 -> 296 shifts where hits land) and
// three new capacities, and the doctor's work is split into ordered steps
// with care levels capping medicine. 48x40/seed42 and 120x120/seed7 move;
// MED-05 alone gives the same values on a clean copy. Same-seed runs
// identical (TestStepIsDeterministicForSameSeed, TestScaleGolden -count=2).
// Re-pinned (PLAY-022: animals amble at half speed when idle): a wandering
// animal's steps take twice the ticks (errands walk at the old step,
// fleeing at the flee step, hunting and charging keep the old step), the
// slow speed class is wider (+12 ticks, was +6), and every animal saves
// two new fields (Errand, StepGait). With the amble and the slow class
// set back to the old numbers only the two new fields move the hash, so
// the rest is behaviour by design. Same-seed runs identical (-count=2).
// Re-pinned (2026-10-30, stale pin): this test skips under -race and the
// determinism gate's -run filter did not match it, so the pins went stale
// across the intended behaviour changes of the SIM-06x, BED/PRD/DEF,
// HAUL-002 and FEED-001 commits, each of which re-pinned TestGrownGolden
// with its reason. The values below are those of commit 121821cac; the
// STO-1 storage index and the STO/RES/CBT contract give the same values
// (proved on a clean HEAD copy). The gate now runs it without -race.
// Re-pinned (wave 2: RES-1, STO-2, CBT-1, CBT-2): partial-stack claims let
// eaters, crafters and haulers share stacks; vanilla stockpile defaults
// and wiki stack limits; body-part-first layered armor, gunshot and stab
// wounds that bleed, vital-part overkill, apparel hit wear and stagger;
// the wiki melee hit and dodge curves, melee tools, Euclidean ranges,
// weather and cover-angle accuracy, auto-undraft and auto-engage. The two
// 48x40 runs with no fighting keep their pins. Same-seed runs identical
// (-count=2).
// Re-pinned (wave 3: CBT-3, CBT-4, CBT-5): equipped weapons keep quality
// and stuff, raiders spawn with armor kits, turret shots fly as
// projectiles, raiders seek cover and target by threat, colonists follow
// their threat response, explosions damage items and cook off stores.
// 48x40/seed1, 48x40/seed7 and 120x120/seed42 keep their pins. Same-seed
// runs identical (-count=2).
// Re-pinned (TEMP-1..4): wiki room equalisation through walls, roofs,
// doors and vents with wiki heat sources and body heat; wiki biome
// extremes and climate events; wiki exposure (16-26 C comfort, hypothermia
// and heatstroke stages, frostbite) for colonists, prisoners, raiders and
// animals; layered clothing (everyone starts in a T-shirt and pants).
// Same-seed runs identical.
// Before that, re-pinned (FOOD-6): food binges bounded to one meal an hour, eating
// takes 500 ticks, wiki hunger offsets and immunity by hunger band,
// per-species breeding, the butcher spot's yield, roofed sowing only
// under sun lamps, eclipse and toxic fallout. Same-seed runs identical.
// Before that, re-pinned (FOOD-4): recipes measured in nutrition, fueled stoves, the
// bench temperature penalty, haulers filling hoppers and barrels, wiki rot
// and outdoor deterioration, food policies, choice by preference and
// distance, wiki meal moods and poisoning. Same-seed runs identical.
// Before that, re-pinned (FOOD-1..3): the vanilla nutrition scale and hunger bands with
// malnutrition, wiki crop, tree and wild-plant growth with night rest, the
// 51 % light rule and partial harvests, butchering by body size, per-species
// products, taming food and auto-slaughter. All six runs move. Same-seed
// runs identical (-count=2).
// Re-pinned (OPT-001 Wave 4): saved messages and letters carry Msg, their
// catalog key and args. With Msg blanked in the hash all six old pins hold;
// the two 48x40 runs that end with no message saved keep theirs.
// Re-pinned (GAR-006): the periodic work is spread (game/cadence). Eggs,
// troughs and pens visit each entity on its own tick of the period; rot,
// wear, pots, medical, census, story and the day's work run on their own
// slots off the hour. Each entity keeps exactly its rate
// (TestStaggeredSitesKeepTheirRate); the tick it fires on moves. Three of
// the six runs move; same-seed runs identical (-count=2).
// Re-pinned (PERF-012): animal repath back-off. An animal whose search
// around a blocker fails doubles its wait (45 to 720) until it steps, and a
// wild wanderer re-picks its goal; colonists and raiders keep the eager
// rule (TestAnimalRepathBackoff). Two of the six runs move; same-seed runs
// identical (-count=2).
// Re-pinned (this run, EAI-014): the wealth tracker sweeps a slice every 25
// ticks, and its cursor, hour and partial sum are saved state (commit
// cc8ba7a0d). The pins went stale there unseen, because the test skipped
// under -race and CI runs only -race; its parent 61967cbd7 still gives the
// old pins. Nothing after it moves them: bd41e9500, 02b37c56b (ally band,
// beacon power, offer quantities) and cb5e5abbb give these values under
// -race on arm64, and amd64 (Rosetta) gives the same at bd41e9500 and
// cb5e5abbb. The test moved here from package game and runs under -race in
// its own CI job.
// Re-pinned (DEF 3.1, 2026-11-21): Kit.DamageTaken, LogEntry.Kind, blast
// credit and LastBy moved 48x40/seed42 and 120x120/seed1.
// Re-pinned (SHT, #75, 2026-11-23): the new Scrapper trait shifts new-game
// trait draws, presets now hold FriendlyFirePct 100 (a hashed field), the
// 21-point accuracy curve, manipulation points and downed interceptors/targets
// move every shot, and the soft daily XP cap and per-level decay table move
// skills. All six seeds move.
// Re-pinned (SHT-105, #273, 2026-11-23): the appended LightFooted trait shifts
// new-game trait draws and Nimble moves melee dodges. All six seeds move again.
// Re-pinned (CBT multi-part hits, 2026-11-24): multi-part hit rules for scratch, stab and bullet (W1) and animal blows through PickPart + dealSpread (W0b): they add body-stream draws, which
// shift every run where a scratch, stab, bullet or animal blow lands.
// 48x40/seed42 and 120x120/seed1 move; the other four are bit-identical.
// Re-pinned (V3 ecosystem-weight wildlife): each generated colony
// has a new, deterministic wild-animal roster. The biome density/area
// budget and species weights replace head counts, changing animal actions,
// their RNG use and dependent saved game state after 20,000 ticks.
// All six original map/seed fixtures were run unshort at 5a1960ba7.
// Re-pinned (WRK-101/102, v20): all founders now have five new work
// priorities. Basic owns switch/casket jobs; Smith and Tailor take their
// corresponding bills before general Craft. The labor flags are also
// saved in the wide field. Map sizes, seeds and 20k-tick duration unchanged.
// Artistic and Brawler integration:
// Generated founders now draw one additional Artistic skill, preserving
// all original skill IDs and appending ID 11. The extra generation draw
// changes downstream generation streams, and the newly saved 12-skill
// arrays change the authoritative world hash. All six existing seeded
// worlds ran the original 20,000 ticks, full native engine under -race.
// The legacy-pawn hash (0x833d997dc5201ca7) independently
// pins the intended character-generator change. No world/tick/seed
// dimension or assertion is removed.
// SIM-071: Mood Base, 6 wealth-expectation
// bands and natural-only catharsis make intentional changes to the
// simulated pawn moods, break schedules, RNG and initial wealth snapshot.
// All six real worlds were rerun unshort for 20,000 ticks under x86 -race;
// no fixtures, seeds or hash checks were removed.
// Semantic event letters: 48x40/seed7 preserves
// the same 20,000-tick simulated world and raid text, while a posted
// letter now saves its stable source Code and typed Fields. All other
// five seeds have their prior canonical hashes unchanged. No RNG,
// tick count, map size, source rule or assertion was changed.
// Re-pinned (C6 R2 Guard flight): 120x120/seed7 changes at the 18,900
// tick sample after retaining safe native flight choices and rejecting
// dangerous routes. A baseline-Guard overlay reproduced the old pin.
// Only Animals, Carried, Claims, Jobs, Motion and Positions differ at
// tick 20,000. RNG and the other five pins are unchanged.
// Public-source correction: body-size wildlife budgeting changes seeded
// species composition and RNG evolution; corrected need and mood rules
// change simulation decisions. Two full 20k-tick runs for every existing
// size/seed produced the same new pins. No dimensions or assertions removed.
// Residual wildlife budget correction: starting groups now select a species
// that fits the remaining budget and stop when none can fit, instead of
// consuming unproductive retries. This intentionally changes the generated
// roster and subsequent RNG. Reverting only animal/ecosystem.go to the
// preceding candidate restored all six original 20,000-tick hashes on the
// combined corrected source. The same sizes, seeds and assertions remain.
// The upstream CBT 3.4b/c reconciliation (2026-11-25) keeps all six
// pins below unchanged in the Core candidate; the new forms have their
// own native split regression tests.
var scaleGolden = map[string]uint64{
	"48x40/seed1":    0x4360e3bf00250daa,
	"48x40/seed7":    0x79fdb5e8da22cef6,
	"48x40/seed42":   0x26f08275d1f48917,
	"120x120/seed1":  0x683fdf5f5fdb73b3,
	"120x120/seed7":  0x4c006521f85ce75e,
	"120x120/seed42": 0xd17e5235177a34c8,
}

const scaleGoldenTicks = 20000

func TestScaleGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("long golden run (20000 ticks per colony)")
	}
	for _, size := range [][2]int{{48, 40}, {120, 120}} {
		for _, seed := range []uint64{1, 7, 42} {
			name := fmt.Sprintf("%dx%d/seed%d", size[0], size[1], seed)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				g := game.NewColony(game.Config{Seed: seed, Width: size[0], Height: size[1]})
				g.Step(scaleGoldenTicks)
				got := g.StateHash()
				t.Logf("%q: %#x,", name, got)
				if want := scaleGolden[name]; got != want {
					t.Fatalf("state hash = %#x, want %#x", got, want)
				}
			})
		}
	}
}
