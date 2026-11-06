package gear

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/command"
)

// kitted is a colonist with a weapon, a spare, three garments (one locked,
// one tainted), a smokepop pack and a pack of food, a drug and wood.
func kitted() game.PawnView {
	return game.PawnView{ID: 4, Name: "Ada", Status: "alive",
		Weapon: "rifle", WeaponStats: weapon.View{Name: "Rifle", Damage: 12, Range: 25}, Spare: "knife",
		Smokepops: 2, ShieldPct: 60, ComfortMin: -50, ComfortMax: 300,
		Apparel:       []string{"tuque", "parka", "flak vest"},
		ApparelForced: []bool{false, true, false},
		Worn: []game.WornView{{Kind: "tuque", HP: 50, MaxHP: 100, Grams: 200},
			{Kind: "parka", HP: 90, MaxHP: 100, Grams: 3500, Cover: "torso, neck, shoulders, arms, legs"},
			{Kind: "flak vest", Stuff: "steel", Quality: "good", HP: 40, MaxHP: 200, Grams: 6000, Tainted: true, MovePct: -3, Cover: "torso, neck, shoulders"}},
		ApparelArmor: []armor.Set{{}, {}, {Sharp: 400, Blunt: 300, Heat: 50}},
		Pack: []game.PackItemView{
			{ID: 11, Name: "meal", Qty: 2, Gear: gearview.Pack{Grams: 900, Use: gearview.UseEat}},
			{ID: 12, Name: "beer", Qty: 1, Gear: gearview.Pack{Grams: 300, Use: gearview.UseDrug, Why: "the drug policy forbids it"}},
			{ID: 13, Name: "wood", Qty: 20, Unload: true, Gear: gearview.Pack{Grams: 4000}}},
		PackGrams: 5200, PackLimitGrams: 40000,
		Gear: gearview.Pawn{GearGrams: 19000, CapacityGrams: 75000, OverallArmor: armor.Set{Sharp: 490, Blunt: 200}, MovePct: -3, ApparelPolicyName: "Soldier"},
	}
}

func TestQualifiedTitle(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ title, stuff, q, want string }{
		{"Longsword", "steel", "good", "Steel longsword (good)"},
		{"Steel ×20", "steel", "", "Steel ×20"},
		{"Rifle", "", "", "Rifle"},
	} {
		if got := QualifiedTitle(c.title, c.stuff, c.q); got != c.want {
			t.Fatalf("%q != %q", got, c.want)
		}
	}
}

func TestWeaponTitleAndTip(t *testing.T) {
	t.Parallel()
	p := game.PawnView{Weapon: "longsword", WeaponStuff: "steel", WeaponQuality: 4, WeaponStats: weapon.Stats(weapon.Longsword)}
	if got := WeaponTitle(p); got != "Steel longsword (good)" {
		t.Fatalf("title %q", got)
	}
	tip := WeaponTip(p)
	if !strings.Contains(tip, "Damage 23") || !strings.Contains(tip, "cooldown 2.6s") || !strings.Contains(tip, "per second") {
		t.Fatalf("tip %q", tip)
	}
	if WeaponTip(game.PawnView{}) != "" || WeaponTitle(game.PawnView{Weapon: "fists"}) != "fists" {
		t.Fatal("unarmed")
	}
}

func TestCombatExtraLines(t *testing.T) {
	t.Parallel()
	p := game.PawnView{Kills: 2, DamageDealt: 55, Spare: "knife"}
	if got := strings.Join(CombatExtraLines(p), "|"); got != "Spare weapon: knife|Kills 2   Damage dealt 55" {
		t.Fatalf("lines %q", got)
	}
	if len(CombatExtraLines(game.PawnView{})) != 0 {
		t.Fatal("no record, no lines")
	}
}

func TestStatLines(t *testing.T) {
	t.Parallel()
	p := game.PawnView{
		WeaponStats: weapon.View{Name: "Rifle", Damage: 12, ArmorPen: 350, Warmup: 90, Cooldown: 120, Burst: 3, Range: 25,
			Accuracy: [4]int{900, 800, 600, 400}, Blast: "fire", BlastRadius: 15},
		Apparel:      []string{"flak vest", "tuque"},
		Worn:         []game.WornView{{Kind: "flak vest", Stuff: "steel", Quality: "good", Tattered: true, Tainted: true}, {Kind: "tuque"}},
		ApparelArmor: []armor.Set{{Sharp: 400, Blunt: 300, Heat: 50}, {}},
	}
	got := strings.Join(StatLines(p), "|")
	for _, w := range []string{"Damage 12   Armor penetration 35%", "Warmup 1.5s   Cooldown 2.0s   Burst 3   Range 25",
		"touch 90%  short 80%  medium 60%  long 40%", "Blast: fire, radius 1.5",
		"Steel flak vest (good) [tattered, worn by a dead pawn]: sharp 40%  blunt 30%  heat 5%", "Tuque: sharp 0%"} {
		if !strings.Contains(got, w) {
			t.Fatalf("%q lacks %q", got, w)
		}
	}
	melee := WeaponLines(game.PawnView{WeaponStats: weapon.View{Name: "Club", Melee: true, Cooldown: 90}})
	if len(melee) != 2 || strings.Contains(strings.Join(melee, "|"), "Accuracy") {
		t.Fatal(melee)
	}
	if len(StatLines(game.PawnView{})) != 0 {
		t.Fatal("empty")
	}
}

// The summary comes first, then Equipment, Apparel in layer order and the
// pack; each item row has its facts.
func TestRowsSummaryAndSections(t *testing.T) {
	t.Parallel()
	got := Lines(kitted(), newFake())
	want := []string{
		"Mass carried  19.0 / 75.0 kg", "Pack  5.2 / 40.0 kg", "Comfortable  -5C to 30C",
		"Armour  sharp 49%  blunt 20%  heat 0%", "Move speed  -3% from gear", "Apparel policy: Soldier",
		"Equipment", "Rifle  in hand", "Knife  spare", "Shield", "Smokepop uses left: 2",
		"Apparel", "Layer flak vest: Steel flak vest (good)  HP 20%   6.0 kg  tainted",
		"Layer parka: Parka  HP 90%   3.5 kg", "Layer tuque: Tuque  HP 50%   0.2 kg",
		"Pack", "Meal x2  0.9 kg", "Beer  0.3 kg", "Wood x20  4.0 kg",
	}
	joined := strings.Join(got, "\n")
	rest := joined
	for _, w := range want {
		i := strings.Index(rest, w)
		if i < 0 {
			t.Fatalf("row %q missing or out of order in\n%s", w, joined)
		}
		rest = rest[i+len(w):]
	}
	bare := strings.Join(Lines(game.PawnView{Weapon: "fists"}, newFake()), "\n")
	for _, w := range []string{"No weapon", "Everyday clothes only", "Comfortable"} {
		if !strings.Contains(bare, w) {
			t.Fatalf("bare colonist lacks %q:\n%s", w, bare)
		}
	}
	for _, w := range []string{"Mass carried", "Pack", "Move speed", "Apparel policy", "Smokepop"} {
		if strings.Contains(bare, w) {
			t.Fatalf("bare colonist shows %q:\n%s", w, bare)
		}
	}
}

// A garment's tooltip names its armour, insulation, layer, cover, move
// offset, what it does, the lock and the taint (GEAR-010).
func TestGarmentTips(t *testing.T) {
	t.Parallel()
	rows := Rows(kitted(), newFake())
	tip := func(label string) string {
		for _, r := range rows {
			if strings.Contains(r.Label, label) {
				return r.Tip
			}
		}
		t.Fatalf("no row %q", label)
		return ""
	}
	vest := tip("flak vest")
	for _, w := range []string{"Hit points 40 / 200", "Armour: sharp 40%  blunt 30%  heat 5%", "Layer: Layer flak vest",
		"Covers: torso, neck, shoulders", "Move speed -3%", "Tainted"} {
		if !strings.Contains(vest, w) {
			t.Fatalf("vest tip lacks %q:\n%s", w, vest)
		}
	}
	parka := tip("Parka")
	for _, w := range []string{"Insulation: cold 22C", "Much warmer, hot in summer", "Locked"} {
		if !strings.Contains(parka, w) {
			t.Fatalf("parka tip lacks %q:\n%s", w, parka)
		}
	}
	if strings.Contains(parka, "Armour:") {
		t.Fatalf("a garment with no armour lists none:\n%s", parka)
	}
}

// panel is the inner rectangle of the tab's panel on a 1280x720 screen.
func panel(sw, sh int) gfx.Rect {
	return hudlayout.Compute(sw, sh, hudlayout.Inputs{Colonists: 6, PaneShown: true, ExpandedW: WantW, ExpandedH: WantH}).Expanded.Inset(paneInset)
}

// paneInset is inspector.PaneInset (the panel's text inset).
const paneInset = 12

// Each button sends its command: Drop (Shift queues), Lock, Eat, Info's
// intent and Unload.
func TestButtonsSendTheirCommands(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.unloading = true
	p := kitted()
	r := panel(1280, 720)
	f.kit.Scroll = map[string]float32{}
	decl := func() { New(f, f).Declare(p, r) }
	reveal := func(id string) {
		// scroll until id is declared: the list is taller than the panel
		for off := float32(0); off < 2000; off += 20 {
			f.kit.Scroll[ScrollID] = off
			f.frameWith(decl)
			if _, ok := f.kit.Find(id); ok {
				return
			}
		}
		t.Fatalf("%s never shows", id)
	}
	for _, c := range []struct {
		id    string
		shift bool
		want  command.Command
	}{
		{DropID("weapon", 0), false, command.Command{Kind: command.DropGear, Pawn: 4, Mode: 2}},
		{DropID("worn", 2), true, command.Command{Kind: command.DropGear, Pawn: 4, Mode: 1, Index: 2, Queue: true}},
		{DropID("pack", 13), false, command.Command{Kind: command.DropGear, Pawn: 4, Mode: 0, Index: 13}},
		{LockID(1), false, command.Command{Kind: command.SetApparelForced, Pawn: 4, Index: 1, Enabled: false}},
		{LockID(0), false, command.Command{Kind: command.SetApparelForced, Pawn: 4, Index: 0, Enabled: true}},
		{EatID(11), false, command.Command{Kind: command.ConsumeFromPack, Pawn: 4, Index: 11}},
		{UnloadID("pack", 13), false, command.Command{Kind: command.MarkUnload, Pawn: 4, Mode: 0, Index: 13, Enabled: false}},
		{UnloadID("spare", 0), false, command.Command{Kind: command.MarkUnload, Pawn: 4, Mode: 3, Enabled: true}},
	} {
		f.cmds, f.shift = nil, c.shift
		reveal(c.id)
		if !f.click(c.id, decl) || len(f.cmds) != 1 || f.cmds[0] != c.want {
			t.Fatalf("%s sent %+v, want %+v", c.id, f.cmds, c.want)
		}
	}
	reveal(InfoID("worn", 1))
	if !f.click(InfoID("worn", 1), decl) || len(f.intents) != 1 || f.intents[0] != (ports.OpenInfoCard{Pawn: 4, Mode: 1, Index: 1}) {
		t.Fatalf("Info sent %+v", f.intents)
	}
}

// A refused action is disabled with the simulation's reason; a mental
// break refuses every action with ours, Info stays.
func TestDisabledButtonsSayWhy(t *testing.T) {
	t.Parallel()
	f := newFake()
	p := kitted()
	p.Gear.NoDrop = "downed"
	r := gfx.Rect{X: 0, Y: 0, W: 456, H: 2000} // everything shows
	f.frameWith(func() { New(f, f).Declare(p, r) })
	for id, why := range map[string]string{DropID("weapon", 0): "downed", EatID(12): "the drug policy forbids it"} {
		if w, ok := f.kit.Find(id); !ok || !w.Disabled || w.Reason != why {
			t.Fatalf("%s: %+v, want disabled with %q", id, w, why)
		}
	}
	if w, _ := f.kit.Find(EatID(11)); w.Disabled {
		t.Fatal("the meal can be eaten")
	}
	p.Gear.NoDrop, p.MentalBreak = "", "sad wander"
	f.frameWith(func() { New(f, f).Declare(p, r) })
	for _, id := range []string{DropID("pack", 11), EatID(11), LockID(0)} {
		if w, ok := f.kit.Find(id); !ok || !w.Disabled || !strings.Contains(w.Reason, "mental break") {
			t.Fatalf("%s in a break: %+v", id, w)
		}
	}
	if w, ok := f.kit.Find(InfoID("pack", 11)); !ok || w.Disabled {
		t.Fatal("Info stays open in a break")
	}
}

// Guests get the tab read-only (Info only); a prisoner may drop but has
// no lock and no Unload.
func TestWhoMayActOnTheGear(t *testing.T) {
	t.Parallel()
	r := gfx.Rect{X: 0, Y: 0, W: 456, H: 2000}
	f := newFake()
	f.unloading = true
	guest := kitted()
	guest.Visitor = true
	f.frameWith(func() { New(f, f).Declare(guest, r) })
	if !ReadOnly(guest) {
		t.Fatal("a guest is read-only")
	}
	for _, id := range []string{DropID("weapon", 0), LockID(0), EatID(11), UnloadID("pack", 11)} {
		if _, ok := f.kit.Find(id); ok {
			t.Fatalf("a guest's tab offers %s", id)
		}
	}
	if _, ok := f.kit.Find(InfoID("worn", 0)); !ok {
		t.Fatal("a guest's gear keeps Info")
	}
	pris := kitted()
	pris.Prisoner, pris.Visitor = true, true
	f.frameWith(func() { New(f, f).Declare(pris, r) })
	if _, ok := f.kit.Find(DropID("worn", 0)); !ok {
		t.Fatal("a prisoner's garment can be dropped")
	}
	for _, id := range []string{LockID(0), UnloadID("worn", 0)} {
		if _, ok := f.kit.Find(id); ok {
			t.Fatalf("a prisoner's tab offers %s", id)
		}
	}
}

// At UI scales 75 to 200 % (the logical screen shrinks as the scale
// grows) every widget of the tab stays inside its panel, the labels keep
// room beside the buttons, the summary starts at the top, and scrolling
// reaches the last pack row (GEAR-013).
func TestTabFitsAtUIScales(t *testing.T) {
	for _, s := range []float64{0.75, 1, 1.25, 1.5, 2} {
		old := gfx.CurrentScale()
		gfx.SetScale(gfx.Scale{Dev: 1, UI: s, S: s})
		sw, sh := int(1920/s), int(1080/s)
		r := panel(sw, sh)
		f := newFake()
		f.unloading = true
		p := kitted()
		decl := func() { New(f, f).Declare(p, r) }
		f.frameWith(decl)
		check := func(when string) {
			for _, w := range f.kit.Cur {
				if w.R.X < r.X-0.01 || w.R.Y < r.Y-0.01 || w.R.Right() > r.Right()+0.01 || w.R.Bottom() > r.Bottom()+0.01 {
					t.Errorf("scale %v %s: %s at %+v leaves the panel %+v", s, when, w.ID, w.R, r)
				}
			}
		}
		check("top")
		if _, ok := f.kit.Find("gear.row.0"); !ok {
			t.Errorf("scale %v: the first summary line is not at the top", s)
		}
		rows := Rows(p, f)
		if ContentH(rows) > r.H {
			f.kit.Scroll[ScrollID] = 1e6
			f.frameWith(decl)
			check("bottom")
		}
		if _, ok := f.kit.Find(DropID("pack", 13)); !ok {
			t.Errorf("scale %v (%dx%d, panel %+v): the last pack row is out of reach", s, sw, sh, r)
		}
		ac := accessOf(p, true)
		for _, row := range rows {
			if row.Item() && r.W-uikit.ScrollBarW-iconSz-6-buttonsW(row, ac) < 120 {
				t.Errorf("scale %v: %q has under 120 px of label", s, row.Label)
			}
		}
		gfx.SetScale(old)
	}
}
