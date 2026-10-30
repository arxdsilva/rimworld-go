package game

import (
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/application/game/hit"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
)

// CBT-1: the damage model. A hit lands on a part picked by coverage, the
// garments covering it take it outermost first and wear, the form picks
// the wound, overkill may destroy the part, and stopping power staggers.

func hitOf(k weapon.Kind) damage.Hit { return weapon.HitOf(k, weapon.DefOf(k)) }

// deflectAll is an armor roll that deflects whenever a garment covers.
func deflectAll(int) int { return 0 }

// TestRifleHitBleeds: a rifle round leaves a bleeding gunshot.
func TestRifleHitBleeds(t *testing.T) {
	g := New(Config{Seed: 2, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	rifle := hitOf(weapon.Rifle)
	rifle.Amount = 8 // under every part's hit points: a wound, never a loss
	hits(g).HurtHit(ada, hits(g).Absorb(ada, rifle, g.rng.IntN), 0)
	b := g.bodies[ada]
	if b == nil || len(b.Wounds) != 1 || b.Wounds[0].Kind != body.Gunshot || body.BleedRate(b.Wounds[0]) == 0 {
		t.Fatalf("a rifle hit bleeds: %+v", b)
	}
	if woundKindLabel(body.Gunshot) != "gunshot" || woundKindLabel(body.Stab) != "stab" {
		t.Fatal("the Health tab names the new wounds")
	}
}

// armorPass is how a hit with an always-deflecting roll fared on each
// part over many hits: covered[p] counts deflections, bare[p] hits that
// passed whole.
func armorPass(t *testing.T, kinds ...item.Kind) (covered, bare map[body.Part]int) {
	t.Helper()
	g := New(Config{Seed: 5, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	covered, bare = map[body.Part]int{}, map[body.Part]int{}
	cut := damage.Hit{Form: damage.FormCut, Amount: 1}
	for range 600 {
		var o craft.Outfit
		for _, k := range kinds {
			o.On[k.Slot()] = craft.Piece{Kind: k}
		}
		g.apparel[ada] = o // fresh each time: wear never destroys them
		l := hits(g).Absorb(ada, cut, deflectAll)
		if !l.Placed {
			t.Fatal("a humanlike hit is placed on a part")
		}
		if l.Outcome == armor.Deflected {
			covered[body.Part(l.Part)]++
		} else {
			bare[body.Part(l.Part)]++
		}
	}
	return covered, bare
}

// TestHelmetProtectsOnlyHead: a helmet stops hits on the brain and ears
// and nothing below the neck.
func TestHelmetProtectsOnlyHead(t *testing.T) {
	covered, bare := armorPass(t, item.SimpleHelmet)
	for p := range covered {
		if p != body.Brain && p != body.EarL && p != body.EarR {
			t.Fatalf("the helmet covered %v", p)
		}
	}
	if covered[body.Brain] == 0 || bare[body.Brain] != 0 || bare[body.Torso] == 0 {
		t.Fatalf("covered %v bare %v", covered, bare)
	}
}

// TestFlakVestTorsoNotLegs: a flak vest covers the torso and its organs,
// never the legs.
func TestFlakVestTorsoNotLegs(t *testing.T) {
	covered, bare := armorPass(t, item.FlakVest)
	if covered[body.Torso] == 0 || bare[body.Torso] != 0 || covered[body.LegL]+covered[body.LegR]+covered[body.FootL] != 0 ||
		bare[body.LegL]+bare[body.LegR] == 0 {
		t.Fatalf("covered %v bare %v", covered, bare)
	}
	for p := range covered {
		if armor.GroupOf(p)&armor.CoverTorso == 0 {
			t.Fatalf("the vest covered %v", p)
		}
	}
}

// TestLayeredArmorWearsOutermostFirst: a duster over flak pants takes a
// leg hit first; both wear by what reached them, and a deflection by the
// duster spares the pants.
func TestLayeredArmorWearsOutermostFirst(t *testing.T) {
	g := New(Config{Seed: 9, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	big := damage.Hit{Form: damage.FormCut, Amount: 40}
	dress := func() {
		g.apparel[ada] = craft.Outfit{On: [craft.Slots]craft.Piece{item.Body: {Kind: item.Duster}, item.Legs: {Kind: item.FlakPants}}}
	}
	seq := func(rs ...int) func(int) int { return func(int) int { r := rs[0]; rs = rs[1:]; return r } }
	dress()
	l := hits(g).LandOn(ada, body.LegL, big, seq(0)) // the duster deflects: one roll only
	if o := g.apparel[ada]; !l.Stopped() || o.On[item.Body].Wear != 10 || o.On[item.Legs].Wear != 0 {
		t.Fatalf("the duster takes it first: %+v outfit %+v", l, o)
	}
	dress()
	l = hits(g).LandOn(ada, body.LegR, big, seq(999, 300)) // the duster passes 40, the pants (A=550) halve it
	if o := g.apparel[ada]; l.Amount != 20 || l.Form != damage.FormBlunt || o.On[item.Body].Wear != 10 || o.On[item.Legs].Wear != 10 {
		t.Fatalf("stacked: %+v outfit %+v", l, o)
	}
	dress()
	if l = hits(g).LandOn(ada, body.Torso, big, seq(999)); l.Amount != 40 || g.apparel[ada].On[item.Legs].Wear != 0 {
		t.Fatalf("the pants do not cover the torso: %+v", l)
	}
}

// TestApparelWearsAndBreaks: absorbed hits wear a garment down to ratty
// and finally destroy it.
func TestApparelWearsAndBreaks(t *testing.T) {
	g := New(Config{Seed: 4, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	g.apparel[ada] = craft.Outfit{On: [craft.Slots]craft.Piece{item.Head: {Kind: item.MarineHelmet}}}
	club := damage.Hit{Form: damage.FormBlunt, Amount: 40}
	for range 4000 {
		hits(g).Absorb(ada, club, deflectAll)
		if g.apparel[ada].On[item.Head].Kind == 0 {
			if v := g.World(); len(v.Messages) == 0 {
				t.Fatal("no message for the destroyed helmet")
			}
			return
		}
	}
	t.Fatalf("40-damage blows never broke the helmet: %+v", g.apparel[ada].On[item.Head])
}

// TestHeadshotDestroysBrainAndKills: a heavy round on the brain overkills
// it past the bullet window: the brain is destroyed and the pawn dies.
func TestHeadshotDestroysBrainAndKills(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	shot := hitOf(weapon.SniperRifle)
	shot.Amount = 60 // the brain has 25: overkill 140 %, past 70 %
	hits(g).HurtHit(ada, armor.Landed{Hit: shot, Part: uint8(body.Brain), Placed: true}, 0)
	if v := g.VitalOf(ada); v.Status != health.Dead || !g.bodies[ada].VitalLost() {
		t.Fatalf("a destroyed brain kills: %+v", v)
	}
	// the same round on an arm destroys the arm and leaves her alive
	bo, _ := g.SpawnNamedPawn(6, 4, "Bo")
	hits(g).HurtHit(bo, armor.Landed{Hit: shot, Part: uint8(body.ArmL), Placed: true}, 0)
	if v := g.VitalOf(bo); v.Status == health.Dead || !g.bodies[bo].PartMissing(body.ArmL) {
		t.Fatalf("an arm is lost, not a life: %+v", v)
	}
}

// TestBluntNeverDestroysUnderWindow: a blunt blow overkilling a part by
// less than 40 % of its hit points never destroys it; the part is left
// at one hit point.
func TestBluntNeverDestroysUnderWindow(t *testing.T) {
	for seed := uint64(1); seed <= 30; seed++ {
		g := New(Config{Seed: seed, Width: 12, Height: 8})
		ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
		mx := body.DefOf(body.ArmL).MaxHP
		club := damage.Hit{Form: damage.FormBlunt, Amount: mx + mx*39/100}
		hits(g).HurtHit(ada, armor.Landed{Hit: club, Part: uint8(body.ArmL), Placed: true}, 0)
		b := g.bodies[ada]
		if b.PartMissing(body.ArmL) || body.PartDamage(b, body.ArmL) != mx-1 {
			t.Fatalf("seed %d: blunt under its window destroyed the arm (damage %d)", seed, body.PartDamage(b, body.ArmL))
		}
	}
}

// TestBluntBrainStuns: a hard blunt blow to the brain stuns: no step and
// no attack until it passes.
func TestBluntBrainStuns(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	now := uint64(g.clock.Now())
	club := damage.Hit{Form: damage.FormBlunt, Amount: (body.DefOf(body.Brain).MaxHP + 1) / 2}
	hits(g).HurtHit(ada, armor.Landed{Hit: club, Part: uint8(body.Brain), Placed: true}, 0)
	if m := g.motionOf(ada); m.NextMoveAt < now+hit.BluntStunTicks {
		t.Fatalf("not stunned: %+v", m)
	}
	bo, _ := g.SpawnNamedPawn(6, 4, "Bo")
	hits(g).HurtHit(bo, armor.Landed{Hit: damage.Hit{Form: damage.FormStun, Amount: 3}}, 0)
	if m := g.motionOf(bo); m.NextMoveAt < now+3*hit.StunPerPoint || g.bodies[bo] != nil && len(g.bodies[bo].Wounds) != 0 {
		t.Fatalf("stun damage stuns without a wound: %+v", m)
	}
}

// TestStaggerSlowsAndExpires: a shotgun round staggers a man (stopping
// power 3 against body size 1): his steps cost six times as long for 95
// ticks, then normal again; an autopistol's 0.5 does not stagger him.
func TestStaggerSlowsAndExpires(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	now := uint64(g.clock.Now())
	hits(g).HurtHit(ada, armor.Landed{Hit: hitOf(weapon.Autopistol), Part: uint8(body.ArmL), Placed: true}, 0)
	if hit.Staggered(g.motionOf(ada).StaggerUntil, now) {
		t.Fatal("0.5 stopping power does not stagger a man")
	}
	shot := hitOf(weapon.PumpShotgun)
	shot.Amount = 2
	hits(g).HurtHit(ada, armor.Landed{Hit: shot, Part: uint8(body.ArmL), Placed: true}, 0)
	m := g.motionOf(ada)
	if !hit.Staggered(g.motionOf(ada).StaggerUntil, now) || m.StaggerUntil != now+hit.StaggerTicks {
		t.Fatalf("staggered until %d", m.StaggerUntil)
	}
	m.stepped(now, TicksPerCell)
	if m.StepTicks != TicksPerCell*hit.StaggerSlow {
		t.Fatalf("staggered step %d", m.StepTicks)
	}
	m.stepped(now+hit.StaggerTicks, TicksPerCell)
	if m.StepTicks != TicksPerCell || hit.Staggered(g.motionOf(ada).StaggerUntil, now+hit.StaggerTicks) {
		t.Fatalf("the stagger ends after 95 ticks: step %d", m.StepTicks)
	}
	var pv PawnView
	j, _ := g.jobs.Get(ada)
	g.fillPawnAction(&pv, ada, j)
	if !pv.Staggered {
		t.Fatal("the read model shows the stagger")
	}
}

// TestStaggerNoFlap: rounds landing during a stagger never extend it, so
// a body under steady fire is slowed in whole 95-tick windows: staggered
// every tick of the window, free at its end, and slowed again only by a
// round after it.
func TestStaggerNoFlap(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	start := uint64(g.clock.Now())
	hits(g).Stagger(ada, start)
	for k := uint64(0); k < hit.StaggerTicks; k++ {
		hits(g).Stagger(ada, start+k) // a round every tick
		if !hit.Staggered(g.motionOf(ada).StaggerUntil, start+k) {
			t.Fatalf("the stagger flapped off at +%d", k)
		}
	}
	if hit.Staggered(g.motionOf(ada).StaggerUntil, start+hit.StaggerTicks) {
		t.Fatal("re-application extended the stagger")
	}
	hits(g).Stagger(ada, start+hit.StaggerTicks+10)
	if !hit.Staggered(g.motionOf(ada).StaggerUntil, start+hit.StaggerTicks+10) || g.motionOf(ada).StaggerUntil != start+2*hit.StaggerTicks+10 {
		t.Fatal("a round after the window staggers again")
	}
}
