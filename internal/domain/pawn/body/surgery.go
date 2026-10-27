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
	cleanSterile    = 1018 // .. 101.8 %)
	darkSurgery     = 750  // wiki: Doctoring (in darkness ×75 %, full from 50 % lit)
	litEnough       = 50   // percent lit from which light costs nothing
)

// SurgeryInput is what goes into one operation's success chance: the
// surgeon's Medicine level, manipulation and sight (permille), the bed
// (a hospital bed or not), an adjacent powered vitals monitor, the room's
// cleanliness (CleanPct, RimWorld cleanliness ×100: 0 a plain clean floor,
// −500 filthy, +100 sterile) and light (LightPct, 0..100 lit), whether it
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
// at +100 (sterile); outdoors 850. wiki: Doctoring (the end points); the
// straight segments between them are original tuning.
func CleanFactor(cleanPct int32, outdoors bool) int32 {
	if outdoors {
		return outdoorSurgery
	}
	c := clamp32(cleanPct, -500, 100)
	if c < 0 {
		return 1000 + (1000-cleanDirtiest)*c/500
	}
	return 1000 + (cleanSterile-1000)*c/100
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
	case (op.Kind == InstallImplant || op.Kind == InstallOrgan) && op.Part == Heart:
		return 250
	}
	return 0
}

// MedicineNeed is the medicine an operation uses up and the lowest kind
// allowed: two herbal or better for installs, removals, harvests and
// amputations, one for anaesthesia and euthanasia, and four industrial
// or better for a carcinoma excision. wiki: Doctoring, Carcinoma.
func MedicineNeed(k OpKind) (qty int, least item.Kind) {
	switch k {
	case Anesthetize, Euthanize:
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

// condOn reports a condition of kind k on part p.
func (b *Body) condOn(k CondKind, p Part) bool {
	for _, c := range b.Conds {
		if c.Kind == k && c.Part == p {
			return true
		}
	}
	return false
}

// CanOperate reports the operation is valid on the current body.
// Surgery is for the humanlike body: no operation is valid on an animal
// plan (MED-13; animal surgery is out of scope).
func (b *Body) CanOperate(op Operation) bool {
	if b.Anatomy != Humanlike || op.Part >= NumParts {
		return false
	}
	switch op.Kind {
	case InstallImplant:
		return Fits(op.Item, op.Part) && b.CanReplace(op.Part)
	case RemoveImplant: // taking out an artificial heart kills
		return b.fitted(op.Part) && !defs[op.Part].Vital
	case Amputate: // organs are harvested, not amputated
		return !b.isMissing(op.Part) && !defs[op.Part].Vital && !Inner(op.Part)
	case HarvestOrgan: // a natural organ still in place
		return OrganOf(op.Part) != 0 && !b.isMissing(op.Part) && !b.blockedAbove(op.Part)
	case InstallOrgan: // into a missing (or artificial) or a damaged organ
		return FitsOrgan(op.Item, op.Part) && !b.blockedAbove(op.Part) &&
			(b.isMissing(op.Part) || b.partDmg(op.Part) > 0)
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
		if !b.isMissing(op.Part) {
			b.destroy(op.Part)
		}
		return b.Install(op.Part, op.Item)
	case RemoveImplant:
		_, ok := b.Remove(op.Part)
		return ok
	case InstallOrgan:
		p := op.Part
		b.SetImplant(p, 0)
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
