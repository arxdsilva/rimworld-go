package hit

import (
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
)

// fake is every role over plain maps.
type fake struct {
	bodies  map[entity.EntityID]*body.Body
	mechs   map[entity.EntityID]armor.Set
	sizes   map[entity.EntityID]int
	outfits map[entity.EntityID]craft.Outfit
	until   map[entity.EntityID]uint64
	held    map[entity.EntityID]uint64
	blows   []Blow
	lost    []item.Kind
	rolls   []int
	now     uint64
}

func newFake() *fake {
	return &fake{bodies: map[entity.EntityID]*body.Body{}, mechs: map[entity.EntityID]armor.Set{}, sizes: map[entity.EntityID]int{},
		outfits: map[entity.EntityID]craft.Outfit{}, until: map[entity.EntityID]uint64{}, held: map[entity.EntityID]uint64{}, now: 100}
}

func (f *fake) Body(id entity.EntityID) *body.Body { return f.bodies[id] }
func (f *fake) Plating(id entity.EntityID) (armor.Set, bool) {
	s, ok := f.mechs[id]
	return s, ok
}
func (f *fake) Size(id entity.EntityID) int {
	if s, ok := f.sizes[id]; ok {
		return s
	}
	return 1000
}
func (f *fake) Roll(n int) int {
	if len(f.rolls) == 0 {
		return 0
	}
	r := f.rolls[0] % n
	f.rolls = f.rolls[1:]
	return r
}
func (f *fake) Outfit(id entity.EntityID) craft.Outfit       { return f.outfits[id] }
func (f *fake) SetOutfit(id entity.EntityID, o craft.Outfit) { f.outfits[id] = o }
func (f *fake) Destroyed(_ entity.EntityID, k item.Kind)     { f.lost = append(f.lost, k) }
func (f *fake) Now() uint64                                  { return f.now }
func (f *fake) Wound(_ entity.EntityID, b Blow, _ entity.EntityID) {
	f.blows = append(f.blows, b)
}
func (f *fake) StaggerUntil(id entity.EntityID) uint64       { return f.until[id] }
func (f *fake) SetStaggerUntil(id entity.EntityID, u uint64) { f.until[id] = u }
func (f *fake) Hold(id entity.EntityID, until uint64)        { f.held[id] = max(f.held[id], until) }
func (f *fake) wounds() Wounds                               { return Wounds{B: f, W: f, H: f} }
func fixed(r int) func(int) int                              { return func(int) int { return r } }
func placed(h damage.Hit, p body.Part) armor.Landed {
	return armor.Landed{Hit: h, Part: uint8(p), Placed: true}
}

func TestAbsorbByTarget(t *testing.T) {
	t.Parallel()
	f := newFake()
	cut := damage.Hit{Form: damage.FormCut, Amount: 10}
	f.mechs[1] = armor.Set{Sharp: 400}
	if l := f.wounds().Absorb(1, cut, fixed(100)); l.Outcome != armor.Deflected || l.Placed {
		t.Fatalf("mech plating %+v", l)
	}
	if l := f.wounds().Absorb(2, cut, fixed(0)); l.Amount != 10 || l.Placed {
		t.Fatalf("an animal's body picks its own part: %+v", l)
	}
	f.bodies[3] = &body.Body{}
	f.outfits[3] = craft.Outfit{On: [craft.Slots]craft.Piece{item.Body: {Kind: item.FlakVest}}}
	f.rolls = []int{body.DefOf(body.Brain).Coverage + 20 + 20 + 10 + 10 + 10 + 20 + 1} // the torso
	if l := f.wounds().Absorb(3, cut, fixed(0)); !l.Placed || body.Part(l.Part) != body.Torso || !l.Stopped() || f.outfits[3].On[item.Body].Wear != 3 {
		t.Fatalf("placed on the torso, under the vest: %+v wear %d", l, f.outfits[3].On[item.Body].Wear)
	}
	f.bodies[4] = &body.Body{Missing: ^uint32(0)}
	if l := f.wounds().Absorb(4, cut, fixed(0)); l.Placed {
		t.Fatal("nothing left to strike")
	}
}

func TestLandOnDestroysWornOut(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.outfits[1] = craft.Outfit{On: [craft.Slots]craft.Piece{item.Head: {Kind: item.Tuque, Wear: craft.Piece{Kind: item.Tuque}.MaxHP() - 1}}}
	l := f.wounds().LandOn(1, body.Brain, damage.Hit{Form: damage.FormBlunt, Amount: 8}, fixed(999))
	if l.Amount != 8 || f.outfits[1].On[item.Head].Kind != 0 || len(f.lost) != 1 || f.lost[0] != item.Tuque {
		t.Fatalf("the last hit point goes: %+v lost %v", l, f.lost)
	}
	if l := f.wounds().LandOn(1, body.Brain, damage.Hit{Form: damage.FormBlunt, Amount: 8}, fixed(0)); l.Amount != 8 {
		t.Fatal("bare now")
	}
}

func TestHurtHitForms(t *testing.T) {
	t.Parallel()
	f := newFake()
	w := f.wounds()
	w.HurtHit(1, placed(damage.Hit{Form: damage.FormBullet, Amount: 9}, body.LegL), 7)
	if len(f.blows) != 1 || f.blows[0] != (Blow{Dmg: 9, Kind: body.Gunshot, Part: body.LegL, Placed: true, Lo: 0, Hi: 700}) {
		t.Fatalf("a bullet's blow %+v", f.blows)
	}
	w.HurtHit(1, armor.Landed{Hit: damage.Hit{Form: damage.FormStun, Amount: 2}}, 0)
	w.HurtHit(1, armor.Landed{Hit: damage.Hit{Form: damage.FormEMP, Amount: 50}}, 0)
	w.HurtHit(1, armor.Landed{Hit: damage.Hit{Form: damage.FormCut, Amount: 0}}, 0)
	if len(f.blows) != 1 || f.held[1] != f.now+2*StunPerPoint {
		t.Fatalf("stun holds without a wound; EMP and stopped hits do nothing: %v held %d", f.blows, f.held[1])
	}
	brain := body.DefOf(body.Brain).MaxHP
	w.HurtHit(2, placed(damage.Hit{Form: damage.FormBlunt, Amount: (brain + 1) / 2}, body.Brain), 0)
	if f.held[2] != f.now+BluntStunTicks {
		t.Fatal("a hard blow to the brain stuns")
	}
	w.HurtHit(3, placed(damage.Hit{Form: damage.FormBlunt, Amount: brain / 2}, body.Brain), 0)
	torso := body.DefOf(body.Torso).MaxHP
	f.rolls = []int{15}
	w.HurtHit(3, placed(damage.Hit{Form: damage.FormBlunt, Amount: torso * 9 / 10}, body.Torso), 0)
	if f.held[3] != 0 {
		t.Fatal("a weaker brain blow and a torso roll of 15 do not stun")
	}
	f.rolls = []int{14}
	w.HurtHit(3, placed(damage.Hit{Form: damage.FormBlunt, Amount: torso * 9 / 10}, body.Torso), 0)
	if f.held[3] != f.now+BluntStunTicks {
		t.Fatal("a torso blow of 90 % stuns 15 % of the time")
	}
}

// TestStaggerNoFlap: stopping power at or over body size staggers; a
// stagger is not extended while it lasts, so it stays on for the whole
// window and ends on time; a hit after it staggers again.
func TestStaggerNoFlap(t *testing.T) {
	t.Parallel()
	f := newFake()
	w := f.wounds()
	f.sizes[2] = 2000
	shot := placed(damage.Hit{Form: damage.FormBullet, Amount: 1, Stopping: 1500}, body.ArmL)
	w.HurtHit(2, shot, 0)
	if f.until[2] != 0 {
		t.Fatal("too big to stagger")
	}
	start := f.now
	for k := uint64(0); k < StaggerTicks; k++ {
		f.now = start + k
		w.HurtHit(1, shot, 0)
		if !Staggered(f.until[1], f.now) || f.until[1] != start+StaggerTicks {
			t.Fatalf("+%d: until %d", k, f.until[1])
		}
	}
	if Staggered(f.until[1], start+StaggerTicks) {
		t.Fatal("ends on time")
	}
	f.now = start + StaggerTicks + 5
	w.HurtHit(1, shot, 0)
	if f.until[1] != f.now+StaggerTicks {
		t.Fatal("staggers again after the window")
	}
	if StepCost(15, f.until[1], f.now) != 15*StaggerSlow || StepCost(15, f.until[1], f.until[1]) != 15 {
		t.Fatal("StepCost")
	}
}

// shieldFake stops every hit on its one target (CBT-3).
type shieldFake struct{ on entity.EntityID }

func (s shieldFake) Shield(target entity.EntityID, _ damage.Hit) bool { return target == s.on }

// TestShieldStopsBeforeTheBody: a shield that takes the hit lets nothing
// through and picks no part; anyone else is struck as before.
func TestShieldStopsBeforeTheBody(t *testing.T) {
	t.Parallel()
	f := newFake()
	w := f.wounds()
	w.S = shieldFake{on: 1}
	bullet := damage.Hit{Form: damage.FormBullet, Amount: 12}
	f.bodies[1] = &body.Body{}
	if l := w.Absorb(1, bullet, fixed(0)); !l.Stopped() || l.Placed || l.Outcome != armor.Deflected {
		t.Fatalf("shielded %+v", l)
	}
	if l := w.Absorb(2, bullet, fixed(0)); l.Amount != 12 {
		t.Fatalf("unshielded %+v", l)
	}
}
