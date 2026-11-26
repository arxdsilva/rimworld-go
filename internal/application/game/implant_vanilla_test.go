// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/stain"
)

func TestBlissCircuitMoodAndMultipleBrainOperationBills(t *testing.T) {
	g := New(Config{Seed: 55, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(10, 6, "Patient")
	g.cmdAddOperation(id, int(body.InstallImplant), uint8(body.Brain), uint8(item.BlissCircuit))
	g.cmdAddOperation(id, int(body.InstallImplant), uint8(body.Brain), uint8(item.NerveDampener))
	g.cmdAddOperation(id, int(body.InstallImplant), uint8(body.Brain), uint8(item.BlissCircuit))
	if len(g.operationsOf(id)) != 2 {
		t.Fatalf("different brain operations must coexist, duplicates blocked: %v", g.operationsOf(id))
	}
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	if !body.AddImplant(b, body.Brain, item.BlissCircuit) {
		t.Fatal("bliss circuit install failed")
	}
	g.bodyChanged(id, b, &v)
	seen := false
	for _, th := range g.thoughts(id) {
		if th.Kind == mood.BlissCircuit {
			seen = true
			if th.Value != 30 {
				t.Fatalf("bliss circuit mood value = %d", th.Value)
			}
		}
	}
	if !seen {
		t.Fatal("bliss circuit thought missing")
	}
	hv, ok := g.Health(id)
	if !ok || len(hv.Parts[body.Brain].Implants) != 1 || hv.Parts[body.Brain].Implants[0] != "bliss circuit" {
		t.Fatalf("health tab missing installed implant: %+v", hv.Parts[body.Brain])
	}
}

func TestAcidFailsafeDissolvesEquippedGearOnly(t *testing.T) {
	g := New(Config{Seed: 56, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(10, 6, "Patient")
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	if !body.AddImplant(b, body.Torso, item.AcidFailsafe) {
		t.Fatal("acid failsafe install failed")
	}
	g.bodyChanged(id, b, &v)
	g.equip[id] = item.Rifle
	g.apparel[id] = craft.Outfit{On: [craft.Slots]craft.Piece{item.Body: {Kind: item.Duster}}}
	beforeRifle, beforeDuster := g.stockOf(item.Rifle), g.stockOf(item.Duster)
	g.dieBy(id, health.CauseInjury, 0)
	if g.stockOf(item.Rifle) != beforeRifle || g.stockOf(item.Duster) != beforeDuster {
		t.Fatalf("equipped gear survived acidifier: rifle %d -> %d, duster %d -> %d",
			beforeRifle, g.stockOf(item.Rifle), beforeDuster, g.stockOf(item.Duster))
	}
	if _, armed := g.equip[id]; armed {
		t.Fatal("wielded weapon remains equipped")
	}
}

// A stowed weapon counts as carried inventory, so only the weapon in
// hand and worn apparel dissolve. The pack survives with its contents.
func TestAcidFailsafePreservesStowedGearOnDeath(t *testing.T) {
	g := New(Config{Seed: 58, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(10, 6, "Patient")
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	if !body.AddImplant(b, body.Torso, item.AcidFailsafe) {
		t.Fatal("install acid failsafe")
	}
	g.bodyChanged(id, b, &v)

	g.equip[id] = item.Rifle
	g.combat.Gear.Hold(id, item.Stack{Kind: item.Rifle, Qty: 1})
	g.combat.Drafts.DropWeapon(id, true)
	if k, n, ok := g.combat.Drafts.Spare(id); !ok || k != item.Rifle || n != 1 {
		t.Fatalf("failed to stow spare weapon: %s %d %v", k, n, ok)
	}
	g.equip[id] = item.Spear
	g.combat.Gear.Hold(id, item.Stack{Kind: item.Spear, Qty: 1})
	g.apparel[id] = craft.Outfit{On: [craft.Slots]craft.Piece{item.Body: {Kind: item.Duster}}}
	supplies := stack(t, g, item.Steel, 5, 10, 6)
	packed(t, g, id, supplies, 5)

	g.dieBy(id, health.CauseInjury, 0)
	if got := loose(g, item.Rifle); got != 1 {
		t.Fatalf("spare rifle on ground: %d, want 1", got)
	}
	if got := loose(g, item.Spear); got != 0 {
		t.Fatalf("wielded spear survived: %d", got)
	}
	if got := loose(g, item.Duster); got != 0 {
		t.Fatalf("worn duster survived: %d", got)
	}
	if got := loose(g, item.Steel); got != 5 {
		t.Fatalf("packed steel lost: %d, want 5", got)
	}
}

// The death effect is triggered once, on the corpse, and does not affect
// another pawn or the map's equipment.
func TestAcidFailsafeLeavesChemicalBurnsOnCorpse(t *testing.T) {
	g := New(Config{Seed: 57, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(10, 6, "Patient")
	other, _ := g.SpawnNamedPawn(12, 6, "Nearby")
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	body.AddImplant(b, body.Torso, item.AcidFailsafe)
	g.bodyChanged(id, b, &v)
	g.equip[id] = item.Rifle
	g.apparel[id] = craft.Outfit{On: [craft.Slots]craft.Piece{item.Body: {Kind: item.Duster}}}
	g.dieBy(id, health.CauseInjury, 0)
	n := 0
	for _, w := range b.Wounds {
		if w.Kind == body.CorrosiveBurn {
			n++
			if w.Dmg != 1 {
				t.Fatalf("chemical burn strength: %+v", w)
			}
		}
	}
	if n < 3 || n > 6 {
		t.Fatalf("corpse has %d chemical burns, expected 3..6", n)
	}
	if _, _, corpse := g.bodyOf(id); !corpse {
		t.Fatal("death has no corpse")
	}
	if residue := g.stains.tiles[[2]int{10, 6}]; residue.Kind != stain.CausticResidue || residue.Amount != deathPool {
		t.Fatalf("corpse left no caustic residue: %+v", residue)
	}
	if b2 := g.bodies[other]; b2 != nil && len(b2.Wounds) != 0 {
		t.Fatal("chemical burns spread to another pawn")
	}
}

func TestCoreImprovisedProsthesisUsesRawWood(t *testing.T) {
	cases := []struct {
		k        item.Kind
		p        body.Part
		usesWood bool
	}{
		{item.PegLeg, body.LegL, true},
		{item.WoodenHand, body.HandR, true},
		{item.ProstheticFoot, body.FootR, true},
		{item.Denture, body.Jaw, false},
	}
	for _, tc := range cases {
		t.Run(tc.k.String(), func(t *testing.T) {
			g, patient, doc := legless(t, 20)
			beforeWood := g.countItem(item.Wood)
			beforeMed := g.countItem(item.HerbalMedicine)
			queueOp(g, patient, body.InstallImplant, tc.p, tc.k)
			if len(g.operationsOf(patient)) != 1 {
				t.Fatalf("not queued: %s", tc.k)
			}
			operateUntil(t, g, patient, doc)
			if got := g.bodies[patient].ReplacementAt(tc.p); got != tc.k {
				t.Fatalf("operation installed %s, want %s", got, tc.k)
			}
			wantWood := beforeWood
			if tc.usesWood {
				wantWood--
			}
			if got := g.countItem(item.Wood); got != wantWood {
				t.Fatalf("wood quantity %d want %d", got, wantWood)
			}
			if got := g.countItem(item.HerbalMedicine); got != beforeMed-2 {
				t.Fatalf("medicine quantity %d want %d", got, beforeMed-2)
			}
			if got := g.stockOf(tc.k); got != 0 {
				t.Fatalf("manufactured %s item unexpectedly appeared", tc.k)
			}
		})
	}
}

func TestAdditiveImplantsSurviveGameSave(t *testing.T) {
	g := New(Config{Seed: 77, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(10, 6, "Patient")
	v := g.VitalOf(id)
	b := g.ensureBody(id, &v)
	if !body.AddImplant(b, body.Brain, item.BlissCircuit) ||
		!body.AddImplant(b, body.Brain, item.NerveDampener) ||
		!body.AddImplant(b, body.Torso, item.AcidFailsafe) {
		t.Fatal("failed to install additive implants")
	}
	g.bodyChanged(id, b, &v)
	var buf bytes.Buffer
	if err := g.Save(&buf, "additive implants"); err != nil {
		t.Fatal(err)
	}
	restored, info, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != SaveVersion {
		t.Fatalf("version %d, want %d", info.Version, SaveVersion)
	}
	live := restored.bodies[id]
	if live == nil || len(live.Implants) != 3 || !body.HasImplant(live, body.Brain, item.BlissCircuit) ||
		!body.HasImplant(live, body.Brain, item.NerveDampener) || !body.HasImplant(live, body.Torso, item.AcidFailsafe) {
		t.Fatalf("implants after load: %+v", live)
	}
}

func TestV17FittedConvertsAtLoadBoundary(t *testing.T) {
	g := New(Config{Seed: 78, Width: 20, Height: 12})
	id, _ := g.SpawnNamedPawn(10, 6, "Patient")
	old := savedBody{Missing: 1 << body.LegL, Fitted: make([]item.Kind, int(body.LegL)+1)}
	old.Fitted[body.LegL] = item.PegLeg
	g.importBodies([]entry[savedBody]{{ID: id, V: old}})
	b := g.bodies[id]
	if b == nil || b.ReplacementAt(body.LegL) != item.PegLeg || len(b.Implants) != 0 || len(b.Fitted) != 0 {
		t.Fatalf("v17 replacement migration: %+v", b)
	}
}

// Older colonies can finish surgery with primitive parts made before
// direct-material Core operations were introduced.
func TestCoreLegacyPrefabSurgeriesStillWork(t *testing.T) {
	cases := []struct {
		k item.Kind
		p body.Part
	}{
		{item.PegLeg, body.LegL},
		{item.WoodenHand, body.HandR},
		{item.ProstheticFoot, body.FootR},
		{item.Denture, body.Jaw},
	}
	for _, tc := range cases {
		t.Run(tc.k.String(), func(t *testing.T) {
			g, patient, doc := legless(t, 20)
			for _, sid := range append([]entity.EntityID(nil), g.itemIDs...) {
				if st, ok := g.stacks.Get(sid); ok && st.Kind == item.Wood {
					g.deleteItem(sid)
				}
			}
			g.SpawnStack(tc.k, 1, 2, 9)
			beforeMedicine := g.countItem(item.HerbalMedicine)
			queueOp(g, patient, body.InstallImplant, tc.p, tc.k)
			if len(g.operationsOf(patient)) != 1 {
				t.Fatalf("legacy %s surgery not queued", tc.k)
			}
			operateUntil(t, g, patient, doc)
			if got := g.bodies[patient].ReplacementAt(tc.p); got != tc.k {
				t.Fatalf("legacy surgery fitted %s, wanted %s", got, tc.k)
			}
			if got := g.stockOf(tc.k); got != 0 {
				t.Fatalf("legacy prefab stock remains: %d", got)
			}
			if got := g.countItem(item.HerbalMedicine); got != beforeMedicine-2 {
				t.Fatalf("medicine stock %d, wanted %d", got, beforeMedicine-2)
			}
		})
	}
}
