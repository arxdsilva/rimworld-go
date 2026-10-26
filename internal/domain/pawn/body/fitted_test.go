package body

import (
	"bytes"
	"encoding/gob"
	"slices"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestImplantAccessor: Fitted grows only as far as the last fitted part
// and goes back to nil once nothing is fitted, so a body without implants
// saves (and hashes) as before.
func TestImplantAccessor(t *testing.T) {
	var b Body
	if b.Implant(LegL) != 0 || b.Fitted != nil {
		t.Fatal("a fresh body has no implants")
	}
	b.SetImplant(EyeR, 0)
	if b.Fitted != nil {
		t.Fatal("clearing an empty slot must not grow Fitted")
	}
	b.SetImplant(EyeL, item.GlassEye)
	if len(b.Fitted) != int(EyeL)+1 || b.Implant(EyeL) != item.GlassEye || b.Implant(LegL) != 0 {
		t.Fatalf("fitted %v", b.Fitted)
	}
	b.SetImplant(LegL, item.PegLeg)
	if len(b.Fitted) != int(LegL)+1 || b.Implant(LegL) != item.PegLeg || b.ArtificialParts() != 2 {
		t.Fatalf("fitted %v", b.Fitted)
	}
	b.SetImplant(LegL, 0)
	if len(b.Fitted) != int(EyeL)+1 {
		t.Fatalf("trailing empty slots are trimmed: %v", b.Fitted)
	}
	b.SetImplant(EyeL, 0)
	if b.Fitted != nil {
		t.Fatalf("no implants left: %v", b.Fitted)
	}
	b.SetImplant(NumParts, item.PegLeg)
	if b.Fitted != nil || b.Implant(NumParts) != 0 {
		t.Fatal("an unknown part takes nothing")
	}
}

// TestInstallRemoveUseFitted: the checked install and removal keep Fitted
// as small as SetImplant does.
func TestInstallRemoveUseFitted(t *testing.T) {
	var b Body
	b.destroy(FootR)
	if !b.Install(FootR, item.ProstheticFoot) || b.Implant(FootR) != item.ProstheticFoot || len(b.Fitted) != int(FootR)+1 {
		t.Fatalf("install: %v", b.Fitted)
	}
	if b.Install(FootR, item.ProstheticFoot) {
		t.Fatal("a fitted part takes no second implant")
	}
	if k, ok := b.Remove(FootR); !ok || k != item.ProstheticFoot || b.Fitted != nil {
		t.Fatalf("remove: %v %v %v", k, ok, b.Fitted)
	}
	if _, ok := b.Remove(FootR); ok {
		t.Fatal("nothing left to remove")
	}
}

// TestFittedFromLegacyArray: an older save's fixed-size per-part array
// becomes the same implants in Fitted.
func TestFittedFromLegacyArray(t *testing.T) {
	var legacy [20]item.Kind
	if FittedFrom(legacy[:]) != nil {
		t.Fatal("an empty array has no implants")
	}
	legacy[LegL], legacy[Nose] = item.PegLeg, item.WoodenNose
	f := FittedFrom(legacy[:])
	b := Body{Fitted: f}
	if len(f) != int(LegL)+1 || b.Implant(LegL) != item.PegLeg || b.Implant(Nose) != item.WoodenNose || b.Implant(LegR) != 0 {
		t.Fatalf("fitted %v", f)
	}
}

// TestBodyGobKeepsMedicalFields: the appended wound and body fields
// survive a gob round trip.
func TestBodyGobKeepsMedicalFields(t *testing.T) {
	b := Body{
		Wounds:    []Wound{{Part: ArmL, Dmg: 4, Kind: Cut, TendQ: 650, TendAt: 7, InfectAt: 99, Bleed: 120}},
		BloodLoss: 210,
		Conds:     []Condition{{Kind: 3, Part: Torso, Sev: 400, Immunity: 200, TendQ: 500, TendSum: 900, TendUntil: 5, Since: 2, Stage: 1}},
	}
	b.SetImplant(EarL, item.HearingTrumpet)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&b); err != nil {
		t.Fatal(err)
	}
	var got Body
	if err := gob.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Wounds, b.Wounds) || got.BloodLoss != b.BloodLoss || !slices.Equal(got.Conds, b.Conds) || !slices.Equal(got.Fitted, b.Fitted) {
		t.Fatalf("%+v", got)
	}
}

// TestNewOperationsNotYetOperable: the operations MED-06 implements are
// in the enum (saved numbers fixed) but cannot be carried out yet.
func TestNewOperationsNotYetOperable(t *testing.T) {
	var b Body
	if NumOps != ExciseCarcinoma+1 || HarvestOrgan != Amputate+1 {
		t.Fatal("operation kinds are append only")
	}
	for k := HarvestOrgan; k < NumOps; k++ {
		for p := Part(0); p < NumParts; p++ {
			if b.CanOperate(Operation{Kind: k, Part: p}) {
				t.Fatalf("operation %d on part %d must not be operable yet", k, p)
			}
		}
	}
}
