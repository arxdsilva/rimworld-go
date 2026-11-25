// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import (
	"fmt"

	"rimworld-go/internal/domain/economy/item"
)

// OpKind is an operation (append only, saved).
type OpKind uint8

const (
	InstallImplant OpKind = iota
	RemoveImplant
	Amputate
	// Surgery v2 (MED-00 added the kinds, MED-06 implements them):
	// harvesting a natural organ, transplanting one into a missing or
	// damaged organ, euthanasia, anaesthesia alone, and excising a
	// carcinoma (needs the condition from MED-08).
	HarvestOrgan
	InstallOrgan
	Euthanize
	Anesthetize
	ExciseCarcinoma
	// NumOps counts the operation kinds (keep last).
	NumOps
)

// Operation is one queued operation on a part (whole-body operations,
// euthanasia and anaesthesia, use the torso).
type Operation struct {
	Kind OpKind
	Part Part
	Item item.Kind
}

// WholeBody reports an operation on the whole patient rather than a part
// (queued on the torso).
func (k OpKind) WholeBody() bool { return k == Euthanize || k == Anesthetize }

// Outcome is how an operation went.
type Outcome uint8

const (
	Success Outcome = iota
	MinorFail
	Catastrophic
	// Ridiculous (MED-06): the surgeon cut the wrong places; several
	// other parts are wounded.
	Ridiculous
)

// MaxSurgeryChance caps SurgeryChance (permille). wiki: Doctoring (98 %).
const MaxSurgeryChance = 980

// excisable is the condition ExciseCarcinoma removes (MED-08): the
// operation is offered on a part with a carcinoma.
const excisable = CondCarcinoma

// surgeryBase is the Medical Surgery Success Chance stat by Medicine
// level 0..20 (permille). wiki: Doctoring.
var surgeryBase = [21]int32{100, 200, 300, 400, 500, 600, 700, 750, 800, 850, 900,
	920, 940, 960, 980, 1000, 1020, 1040, 1060, 1080, 1100}

// Surgery factors (permille).
const (
	hospitalSurgery = 1150 // wiki: Hospital bed (surgery success ×115 %)
	outdoorSurgery  = 850  // wiki: Doctoring (operating outdoors ×85 %)
	monitorSurgery  = 1050 // wiki: Vitals monitor (surgery success ×105 %)
	cleanDirtiest   = 600  // wiki: Doctoring (cleanliness factor 60 % ..
	cleanSterile    = 1018 // .. 101.8 %, reached at sterileClean)
	sterileClean    = 60   // wiki: Sterile tile (cleanliness +0.6: surgery ×1.02 in a sterile hospital, AX-3)
	darkSurgery     = 750  // wiki: Doctoring (in darkness ×75 %, full from 50 % lit)
	litEnough       = 50   // percent lit from which light costs nothing
)

// SurgeryInput is what goes into one operation's success chance: the
// surgeon's Medicine level, manipulation and sight (permille), the bed
// (a hospital bed or not), an adjacent powered vitals monitor, the room's
// cleanliness (CleanPct, RimWorld cleanliness ×100: 0 a plain clean floor,
// −500 filthy, +60 sterile tile) and light (LightPct, 0..100 lit), whether it
// happens outdoors (cleanliness then counts as outdoor), the operation and
// the medicine used (0: none).
type SurgeryInput struct {
	Skill              int
	Manip, Sight       int32
	HospitalBed        bool
	Monitor            bool
	CleanPct, LightPct int32
	Outdoors           bool
	Op                 OpKind
	Medicine           item.Kind
}

// Factor is one line of the success chance breakdown: the base chance,
// then each multiplier, in percent (the chance tooltip).
type Factor struct {
	Label string
	Pct   int32
}

// opFactor is an operation's own success multiplier (permille). wiki:
// Doctoring (recipe success factors).
func opFactor(k OpKind) int32 {
	switch k {
	case RemoveImplant, Amputate, HarvestOrgan:
		return 1200
	case Anesthetize:
		return 2000
	case Euthanize:
		return 5000
	}
	return 1000 // install implant or organ, excise carcinoma
}

// medicineSurgery is the medicine's success multiplier (permille). The
// wiki gives no medicine factor for surgery: original tuning.
func medicineSurgery(k item.Kind) int32 {
	switch k {
	case item.HerbalMedicine:
		return 1000
	case item.IndustrialMedicine:
		return 1100
	case item.NanomedKit:
		return 1200
	}
	return 700 // none
}

// CleanFactor is the room cleanliness multiplier (permille): linear from
// 600 at −500 (filthy) to 1000 at 0 (a clean plain floor), then to 1018
// at +60 (a room in sterile tile, the cleanest floor); outdoors 850. wiki:
// Doctoring, Sterile tile (the end points); the straight segments between
// them are original tuning.
func CleanFactor(cleanPct int32, outdoors bool) int32 {
	if outdoors {
		return outdoorSurgery
	}
	c := clamp32(cleanPct, -500, sterileClean)
	if c < 0 {
		return 1000 + (1000-cleanDirtiest)*c/500
	}
	return 1000 + (cleanSterile-1000)*c/sterileClean
}

// LightFactor is the light multiplier (permille): full from 50 % lit,
// falling linearly to 750 in darkness. wiki: Doctoring.
func LightFactor(lightPct int32) int32 {
	l := clamp32(lightPct, 0, 100)
	if l >= litEnough {
		return 1000
	}
	return darkSurgery + (1000-darkSurgery)*l/litEnough
}

// SurgeryChance is an operation's permille success chance with its
// breakdown: the Medicine level's base × manipulation × sight (at most
// 100 %) × bed × cleanliness × light × vitals monitor × medicine × the
// operation's own factor, capped at 98 %. wiki: Doctoring.
func SurgeryChance(in SurgeryInput) (int32, []Factor) {
	s := min(max(in.Skill, 0), 20)
	c := int64(surgeryBase[s])
	fs := make([]Factor, 0, 10)
	fs = append(fs, Factor{fmt.Sprintf("Medicine skill %d", s), surgeryBase[s] / 10})
	mul := func(label string, f int32) {
		c = c * int64(f) / 1000
		fs = append(fs, Factor{label, f / 10})
	}
	mul("Manipulation", max(in.Manip, 0))
	mul("Sight", clamp32(in.Sight, 0, 1000))
	if in.HospitalBed {
		mul("Hospital bed", hospitalSurgery)
	}
	if in.Outdoors {
		mul("Outdoors", outdoorSurgery)
	} else {
		mul("Cleanliness", CleanFactor(in.CleanPct, false))
	}
	mul("Light", LightFactor(in.LightPct))
	if in.Monitor {
		mul("Vitals monitor", monitorSurgery)
	}
	medLabel := "No medicine"
	if in.Medicine.IsMedicine() {
		medLabel = in.Medicine.String()
	}
	mul(medLabel, medicineSurgery(in.Medicine))
	if f := opFactor(in.Op); f != 1000 {
		mul("Operation", f)
	}
	if c > MaxSurgeryChance {
		c = MaxSurgeryChance
		fs = append(fs, Factor{"Capped", MaxSurgeryChance / 10})
	}
	return int32(c), fs
}

// RidiculousShare is the permille of failures that are ridiculous: 10 %
// for a surgeon under Medicine 6, else 2 % (original tuning).
func RidiculousShare(skill int) int {
	if skill < 6 {
		return 100
	}
	return 20
}

// RollOutcome decides an operation of success chance chance (permille)
// by a surgeon of Medicine skill: a failure is ridiculous by
// RidiculousShare, else minor 80:20 against catastrophic.
func RollOutcome(chance int32, skill int, roll func(int) int) Outcome {
	if int32(roll(1000)) < chance {
		return Success
	}
	if roll(1000) < RidiculousShare(skill) {
		return Ridiculous
	}
	if roll(100) < 80 {
		return MinorFail
	}
	return Catastrophic
}

// FailDeathChance is the permille chance a failed op kills the patient
// outright: a heart transplant (natural or artificial) or a carcinoma
// excision 25 %. wiki: Doctoring.
func FailDeathChance(op Operation) int32 {
	switch {
	case op.Kind == ExciseCarcinoma:
		return 250
	case op.Kind == InstallOrgan && op.Part == Heart:
		return 250
	case op.Kind == InstallImplant && op.Part == Heart && op.Item == item.ValveHeart:
		return 250
	}
	return 0
}

// OperationMaterial is the stock item a doctor carries into surgery.
// wiki: Peg leg, Wooden hand, Wooden foot (one wood), Denture (medicine
// only). Old item IDs stay readable in inventory and saves, but new
// procedures use the raw material directly.
func OperationMaterial(op Operation) item.Kind {
	if op.Kind != InstallImplant && op.Kind != InstallOrgan {
		return 0
	}
	if op.Kind == InstallImplant {
		switch op.Item {
		case item.PegLeg, item.WoodenHand, item.ProstheticFoot:
			return item.Wood
		case item.Denture:
			return 0
		}
	}
	return op.Item
}

// LegacyMaterial lets an older save use its manufactured primitive parts
// in surgery. New surgeries prefer raw stock; previous item IDs remain
// usable until depleted without adding new crafting recipes.
func LegacyMaterial(op Operation) item.Kind {
	if op.Kind != InstallImplant {
		return 0
	}
	switch op.Item {
	case item.PegLeg, item.WoodenHand, item.ProstheticFoot, item.Denture:
		return op.Item
	}
	return 0
}

func AcceptsMaterial(op Operation, k item.Kind) bool {
	return k != 0 && (k == OperationMaterial(op) || k == LegacyMaterial(op))
}

// OperationWork is surgical work before doctor speed, in game ticks.
// wiki: Peg leg, Wooden hand, Denture (1,500), Wooden foot (1,000),
// Bionic leg, Painstopper (2,500), and prosthetic removal (2,000).
// Zero leaves general surgery on the existing project work schedule.
func OperationWork(op Operation) int {
	switch op.Kind {
	case InstallImplant:
		switch op.Item {
		case item.ProstheticFoot:
			return 1000
		case item.PegLeg, item.WoodenHand, item.Denture:
			return 1500
		default:
			return 2500
		}
	case RemoveImplant:
		if op.Item == item.NerveDampener {
			return 2500
		}
		return 2000
	case InstallOrgan:
		return 2500 // wiki: Kidney, Lung, Heart, Liver
	case ExciseCarcinoma:
		return 4500 // wiki: Carcinoma
	case HarvestOrgan:
		return 2000 // wiki: Kidney, Lung, Heart, Liver (removal)
	}
	return 0
}

// MedicineNeed is the medicine an operation uses up and the lowest kind
// allowed: two herbal or better for installs and harvests, one for removal and
// amputations, anaesthesia and euthanasia, and four industrial
// or better for a carcinoma excision. wiki: Doctoring, Carcinoma.
func MedicineNeed(k OpKind) (qty int, least item.Kind) {
	switch k {
	case Anesthetize, Euthanize, RemoveImplant, HarvestOrgan:
		// wiki: Kidney, Lung, Heart, Liver (each harvest uses one dose).
		return 1, item.HerbalMedicine
	case ExciseCarcinoma:
		return 4, item.IndustrialMedicine
	}
	return 2, item.HerbalMedicine
}

// MinSkill is the Medicine level an operation needs (0: any). wiki:
// Carcinoma (excision needs Medicine 10).
func MinSkill(k OpKind) int {
	if k == ExciseCarcinoma {
		return 10
	}
	return 0
}

// OperationMinSkill includes part-specific implant requirements.
func OperationMinSkill(op Operation) int {
	if (op.Kind == InstallImplant || op.Kind == RemoveImplant) && op.Item.IsAddedImplant() {
		return MedicalSkill(op.Item)
	}
	if op.Kind == InstallImplant {
		return MedicalSkill(op.Item)
	}
	if op.Kind == InstallOrgan {
		if op.Part == Heart {
			return 8
		} // wiki: Heart
		return 5 // wiki: Kidney, Lung, Liver
	}
	return MinSkill(op.Kind)
}

// condOn reports a condition of kind k on part p.
func (b *Body) condOn(k CondKind, p Part) bool {
	for _, c := range b.Conds {
		if c.Kind == k && c.Part == p {
			return true
		}
	}
	return false
}

// operationsConflict prevents duplicate or mutually exclusive requests,
// while allowing distinct additive implants on the same body part.
func OperationsConflict(a, b Operation) bool {
	if a.Kind != b.Kind || a.Part != b.Part {
		return false
	}
	if a.Kind == InstallImplant || a.Kind == RemoveImplant {
		if a.Item.IsAddedImplant() && b.Item.IsAddedImplant() {
			return a.Item == b.Item
		}
	}
	return true
}

// SurgeryDrop is a recovered item or material after a successful operation.
// Recovery is decided before Apply destroys the replaced part.
type SurgeryDrop struct {
	Kind item.Kind
	Qty  int
}

// SurgeryYields returns items recovered by a successful installation,
// removal or amputation. Installed organs remain separate harvest operations.
func SurgeryYields(b *Body, op Operation) []SurgeryDrop {
	if b == nil {
		return nil
	}
	switch op.Kind {
	case InstallImplant, RemoveImplant, InstallOrgan, Amputate:
	default:
		return nil
	}
	var drops []SurgeryDrop
	if op.Kind == RemoveImplant && op.Item.IsAddedImplant() {
		k, n := RemovalYield(op.Item)
		if n > 0 {
			drops = append(drops, SurgeryDrop{Kind: k, Qty: n})
		}
		return drops
	}
	for p, r := range b.Replacements {
		if r.Kind == 0 || !b.under(Part(p), op.Part) {
			continue
		}
		if op.Kind == RemoveImplant && Part(p) != op.Part {
			continue
		}
		k, n := RemovalYield(r.Kind)
		if n > 0 {
			drops = append(drops, SurgeryDrop{Kind: k, Qty: n})
		}
	}
	// A healthy natural heart removed to fit an artificial heart can be
	// transplanted, without the organ-harvest thought (replacement surgery).
	if op.Kind == InstallImplant && op.Part == Heart && b.ReplacementAt(Heart) == 0 &&
		!b.isMissing(Heart) && b.partDmg(Heart) == 0 {
		drops = append(drops, SurgeryDrop{Kind: item.Heart, Qty: 1})
	}
	return drops
}

// AvailableOperations lists every valid procedure in the Health tab's
// stable surgery order. The simulation owns validity; UI only adds stock,
// doctor and medicine availability and filters queued operations.
func AvailableOperations(b *Body) []Operation {
	var out []Operation
	add := func(op Operation) {
		if b.CanOperate(op) {
			out = append(out, op)
		}
	}
	for p := Part(0); p < NumParts; p++ {
		for k, def := range artificialDefs {
			if def.Parts&bit(p) != 0 && !def.Legacy {
				add(Operation{Kind: InstallImplant, Part: p, Item: item.Kind(k)})
			}
		}
	}
	for p := Part(0); p < NumParts; p++ {
		add(Operation{Kind: RemoveImplant, Part: p})
		for _, imp := range b.Implants {
			if imp.Part == p {
				add(Operation{Kind: RemoveImplant, Part: p, Item: imp.Kind})
			}
		}
	}
	for p := Part(0); p < NumParts; p++ {
		if !Inner(p) {
			add(Operation{Kind: Amputate, Part: p})
		}
	}
	for p := Part(0); p < NumParts; p++ {
		if OrganOf(p) != 0 {
			add(Operation{Kind: HarvestOrgan, Part: p})
		}
	}
	for p := Part(0); p < NumParts; p++ {
		if k := OrganOf(p); k != 0 {
			add(Operation{Kind: InstallOrgan, Part: p, Item: k})
		}
	}
	add(Operation{Kind: Euthanize, Part: Torso})
	add(Operation{Kind: Anesthetize, Part: Torso})
	for p := Part(0); p < NumParts; p++ {
		add(Operation{Kind: ExciseCarcinoma, Part: p})
	}
	return out
}

// OpCode packs a surgery bill into a stable job identifier.
func OpCode(op Operation) int { return int(op.Kind)<<16 | int(op.Part)<<8 | int(op.Item) }

// CanOperate reports the operation is valid on the current body.
// Surgery is for the humanlike body: no operation is valid on an animal
// plan (MED-13; animal surgery is out of scope).
func (b *Body) CanOperate(op Operation) bool {
	if b.Anatomy != Humanlike || op.Part >= NumParts {
		return false
	}
	switch op.Kind {
	case InstallImplant:
		if op.Item.IsAddedImplant() {
			return CanAddImplant(b, op.Part, op.Item)
		}
		return FitsReplacement(op.Item, op.Part) && b.CanReplace(op.Part)
	case RemoveImplant:
		if op.Item.IsAddedImplant() {
			return artificial(op.Item).Removable && HasImplant(b, op.Part, op.Item)
		}
		return op.Item == 0 && b.hasReplacement(op.Part)
	case Amputate: // organs are harvested, not amputated
		return !b.isMissing(op.Part) && !defs[op.Part].Vital && !Inner(op.Part)
	case HarvestOrgan: // a natural organ still in place
		return OrganOf(op.Part) != 0 && !b.isMissing(op.Part) && !b.blockedAbove(op.Part)
	case InstallOrgan: // into a missing (or artificial) or a damaged organ
		return FitsOrgan(op.Item, op.Part) && !b.blockedAbove(op.Part) &&
			(b.isMissing(op.Part) || b.partDmg(op.Part) > 0 ||
				(op.Item == item.Lung && b.condOn(CondAsthma, op.Part)))
	case Euthanize, Anesthetize:
		return op.Part == Torso
	case ExciseCarcinoma:
		return excisable != CondNone && b.condOn(excisable, op.Part)
	}
	return false
}

// Apply carries out op with the given outcome and reports whether the
// intended change happened. Failures wound the patient: minor ones with a
// 5-15 damage surgical wound (never destroying the part), catastrophic ones
// with 20-40 damage that may destroy it, ridiculous ones with 2-4 surgical
// wounds of 5-15 on random parts. A failed spinal brace destroys the
// spine. Euthanasia and anaesthesia change nothing here (the game kills
// or sedates).
func (b *Body) Apply(op Operation, o Outcome, roll func(int) int, now uint64) bool {
	if !b.CanOperate(op) {
		return false
	}
	target := op.Part
	for b.isMissing(target) && defs[target].Parent != target {
		target = defs[target].Parent
	}
	switch o {
	case Success:
		return b.succeed(op)
	case MinorFail:
		b.minorWound(target, roll, now)
	case Ridiculous:
		for n := 2 + roll(3); n > 0; n-- {
			if p, ok := b.PickPart(roll); ok {
				b.minorWound(p, roll, now)
			}
		}
	default:
		b.hurtPart(target, 20+roll(21), Surgical, now)
	}
	if op.Kind == InstallImplant && op.Item == item.SpinalBrace && !b.isMissing(Spine) {
		b.destroy(Spine) // wiki: Bionic spine (a failure destroys the spine)
	}
	return false
}

// succeed applies a successful operation.
func (b *Body) succeed(op Operation) bool {
	defer b.dropAgingAfter(op) // aging.go (MED-08): a replaced or removed part takes its age conditions along
	switch op.Kind {
	case InstallImplant:
		if op.Item.IsAddedImplant() {
			return AddImplant(b, op.Part, op.Item)
		}
		if !b.isMissing(op.Part) || b.hasReplacement(op.Part) {
			b.destroy(op.Part)
		}
		if !b.InstallReplacement(op.Part, op.Item) {
			return false
		}
		if op.Item == item.SynthStomach {
			b.RemoveCond(CondGutWorms) // wiki: Bionic stomach, cures existing gut worms
		}
		return true
	case RemoveImplant:
		if op.Item.IsAddedImplant() {
			return DetachImplant(b, op.Part, op.Item)
		}
		_, ok := b.RemoveReplacement(op.Part)
		return ok
	case InstallOrgan:
		p := op.Part
		b.SetReplacement(p, 0)
		b.Missing &^= bit(p)
		b.Fresh &^= bit(p)
		kept := b.Wounds[:0]
		for _, w := range b.Wounds {
			if w.Part != p {
				kept = append(kept, w)
			}
		}
		b.Wounds = kept
		return true
	case Euthanize, Anesthetize:
		return true
	case ExciseCarcinoma:
		kept := b.Conds[:0]
		for _, c := range b.Conds {
			if c.Kind != excisable || c.Part != op.Part {
				kept = append(kept, c)
			}
		}
		clear(b.Conds[len(kept):])
		b.Conds = kept
		if len(b.Conds) == 0 {
			b.Conds = nil
		}
		return true
	}
	b.destroy(op.Part) // amputate, harvest
	return true
}

// minorWound cuts part p for 5-15 damage, never destroying it.
func (b *Body) minorWound(p Part, roll func(int) int, now uint64) {
	dmg := 5 + roll(11)
	if room := b.maxHP(p) - 1 - b.partDmg(p); dmg > room {
		dmg = room
	}
	if dmg > 0 && !b.isMissing(p) {
		b.Wounds = append(b.Wounds, Wound{Part: p, Dmg: dmg, Kind: Surgical, At: now, Size: int32(dmg)})
	}
}

// DoneText is the log wording of operation op's outcome o (moved from the
// game package, AX-8).
func (op Operation) DoneText(o Outcome) string {
	part := op.Part.Label()
	switch o {
	case MinorFail:
		return "slipped during an operation on the " + part
	case Catastrophic:
		return "badly botched an operation on the " + part
	case Ridiculous:
		return "made a ridiculous mess of an operation on the " + part
	}
	switch op.Kind {
	case InstallImplant:
		return "fitted a " + op.Item.String() + " to the " + part
	case RemoveImplant:
		if op.Item.IsAddedImplant() {
			return "removed the " + op.Item.String() + " from the " + part
		}
		return "removed the artificial part from the " + part
	case HarvestOrgan:
		return "harvested the " + part
	case InstallOrgan:
		return "transplanted a " + op.Item.String() + " into the " + part
	case Anesthetize:
		return "anesthetized the patient"
	case ExciseCarcinoma:
		return "excised the carcinoma from the " + part
	}
	return "amputated the " + part
}
