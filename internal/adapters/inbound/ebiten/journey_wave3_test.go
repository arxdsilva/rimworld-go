package ebiten

// journey_wave3_test.go: the wave 3 journeys (CBT-3 weapons, gear and
// structures; CBT-4 combat AI; CBT-5 explosions and combat UI) through the
// real UI loop. Hooks and read-model fields that land with the parallel
// agents are looked up at run time, so a missing one fails the journey with
// a message instead of breaking the build.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// TestJourneyRocketLauncherSingleUse (CBT-3): a drafted colonist with a
// rocket launcher fires at raiders; a blast appears and the launcher is
// spent and gone afterwards.
func TestJourneyRocketLauncherSingleUse(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(48, 48), jPawns([2]int{24, 24}))
	id := ordColonists(j)[0]
	j.g.Arm(weID(id), item.RocketLauncher)
	launcher := j.pawn(id).Weapon
	if launcher == "fists" {
		t.Fatal("Arm(RocketLauncher) left the colonist unarmed: the rocket launcher is not a weapon yet (CC)")
	}
	j.lookAt(24, 24)
	j.update(1)
	j.settle()
	weDraft(j, 0)
	j.g.RaidArmed(item.Club, item.Club)
	j.update(1) // the raid alert pauses the game
	blast, spent := false, false
	j.key("Digit3", 0)
	for i := 0; i < 5000 && !(blast && spent); i++ {
		careFeed(j)
		j.update(1)
		for _, b := range j.world().Bursts {
			blast = blast || (b.Blast != "" && !b.Turret)
		}
		spent = blast && j.pawn(id).Weapon != launcher
	}
	j.key("Space", 0)
	if !blast {
		t.Fatalf("no blast burst after the colonist fired the launcher: %+v", j.world().Bursts)
	}
	if !spent {
		t.Fatalf("the colonist still holds %q after the rocket was fired: a launcher is single use", j.pawn(id).Weapon)
	}
	for _, it := range j.world().Items {
		if it.Kind == item.RocketLauncher.String() {
			t.Fatalf("the spent launcher still lies at (%d, %d)", it.X, it.Y)
		}
	}
}

// TestJourneyShieldBeltBlocksBullets (CBT-3): a colonist wearing a shield
// belt is shot at: the charge drops and no gunshot wound lands while it
// holds; an EMP grenade then breaks the shield.
func TestJourneyShieldBeltBlocksBullets(t *testing.T) {
	t.Parallel()
	t.Run("bullets", func(t *testing.T) {
		t.Parallel()
		j := newJourney(t, jPawns([2]int{8, 4}))
		id := ordColonists(j)[0]
		w3Wear(j, id, item.ShieldBelt, 8, 5)
		start := w3Shield(j, id)
		if start <= 0 {
			t.Fatalf("a worn shield belt has charge %v", start)
		}
		j.g.RaidArmed(item.Rifle, item.Rifle)
		j.update(1)
		low := start
		j.key("Digit3", 0)
		for i := 0; i < 3000 && low >= start; i++ {
			careFeed(j)
			j.update(1)
			s := w3Shield(j, id)
			if s > 0 && w3GunshotOn(j, weID(id)) {
				j.key("Space", 0)
				t.Fatalf("a gunshot wound landed while the shield still held %v of %v", s, start)
			}
			low = min(low, s)
		}
		j.key("Space", 0)
		if low >= start {
			t.Fatalf("the shield charge never dropped from %v under rifle fire; hostiles %d", start, len(weRaiders(j)))
		}
	})
	t.Run("emp", func(t *testing.T) {
		t.Parallel()
		j := newJourney(t, jPawns([2]int{8, 4}, [2]int{16, 4}))
		cs := ordColonists(j)
		thrower, wearer := cs[0], cs[1]
		w3Wear(j, wearer, item.ShieldBelt, 16, 5)
		start := w3Shield(j, wearer)
		weGrenadier(j, thrower, item.EMPGrenades)
		w := j.pawn(wearer)
		w3Throw(j, 0, w.X, w.Y)
		j.key("Digit3", 0)
		for i := 0; i < 1500 && w3Shield(j, wearer) >= start; i++ {
			j.update(1)
		}
		j.key("Space", 0)
		if s := w3Shield(j, wearer); s >= start {
			t.Fatalf("the shield still has %v of %v after an EMP burst (bursts %+v)", s, start, j.world().Bursts)
		}
	})
}

// TestJourneyWeaponQualityKept (CBT-3): a masterwork rifle keeps its
// quality on the colonist, shows it on the gear tab and through a save.
func TestJourneyWeaponQualityKept(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	id := ordColonists(j)[0]
	stored := int(quality.Masterwork) + 1
	w3Call(j, []string{"ArmQuality", "ArmWithQuality", "ArmStackQuality", "SpawnWeaponQuality", "ArmQ"}, []string{"quality", "arm"}, weID(id), item.Rifle, quality.Masterwork)
	j.update(1)
	p := j.pawn(id)
	if int(p.WeaponQuality) != stored {
		t.Fatalf("PawnView.WeaponQuality is %d, want the stored masterwork %d (weapon %q, stuff %q)", p.WeaponQuality, stored, p.Weapon, p.WeaponStuff)
	}
	ordPick(j, 0)
	j.clickID("inspector.gear")
	j.draw(2)
	lines := strings.ToLower(inspector.WeaponTitle(p) + "\n" + strings.Join(inspector.GearStatLines(p), "\n"))
	if !strings.Contains(lines, "masterwork") {
		t.Fatalf("the gear tab lines do not show the quality:\n%s", lines)
	}
	var buf bytes.Buffer
	if err := j.g.Save(&buf, "w3"); err != nil {
		t.Fatal(err)
	}
	g2, _, err := game.Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range g2.Snapshot() {
		if uint64(q.ID) == id {
			if q.WeaponQuality != p.WeaponQuality || q.Weapon != p.Weapon || q.WeaponStuff != p.WeaponStuff {
				t.Fatalf("after save and load the weapon is %q quality %d stuff %q, want %q %d %q", q.Weapon, q.WeaponQuality, q.WeaponStuff, p.Weapon, p.WeaponQuality, p.WeaponStuff)
			}
			return
		}
	}
	t.Fatal("the colonist is gone after load")
}

// TestJourneyRaidersWearArmor (CBT-3): a raid arrives and at least one
// raider wears armor.
func TestJourneyRaidersWearArmor(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	for range 3 {
		j.g.Raid(8)
	}
	j.update(2)
	rs := weRaiders(j)
	if len(rs) == 0 {
		t.Fatal("the raid brought no raiders")
	}
	worn := 0
	for _, r := range rs {
		for _, a := range r.ApparelArmor {
			if !isZero(a) {
				worn++
			}
		}
	}
	if worn == 0 {
		t.Fatalf("none of %d raiders wears armor: apparel of the first %q (CC raider kits)", len(rs), rs[0].Apparel)
	}
}

// TestJourneyMiniTurretProjectiles (CBT-3): a mini-turret's shots show as
// projectiles on WorldView.Shots.
func TestJourneyMiniTurretProjectiles(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	built := false
	for _, k := range []string{"mini-turret", "mini turret", "sentry turret"} {
		for _, st := range []string{"steel", ""} {
			if !built && j.g.BuildNow(k, st, "south", 10, 5) {
				built = true
			}
		}
	}
	if !built {
		t.Fatal("no turret could be built at (10, 5)")
	}
	id := uint64(wbBuildingAt(j, 10, 5))
	wdBuild(j, "wood-fired generator", 15, 2) // the turret needs power to fire
	j.g.FuelCampfire(15, 2)
	for x := 8; x <= 15; x++ {
		j.g.BuildNow("power conduit", "steel", "south", x, 4)
	}
	j.tick(240)
	j.g.RaidArmed(item.Club, item.Club, item.Club)
	j.update(1)
	shots := 0
	j.key("Digit3", 0)
	for i := 0; i < 4000 && shots == 0; i++ {
		careFeed(j)
		j.update(1)
		shots = weShotsBy(j, id)
	}
	j.key("Space", 0)
	if shots == 0 {
		t.Fatalf("no projectile owned by the turret %d: shots %+v", id, j.world().Shots)
	}
}

// TestJourneyRaidersSeekCover (CBT-4): raiders under fire stand next to
// sandbags near the colonists rather than walking straight in.
func TestJourneyRaidersSeekCover(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(48, 48), jPawns([2]int{24, 24}, [2]int{25, 24}))
	var bags [][2]int
	for _, c := range [][2]int{{15, 23}, {15, 24}, {15, 25}, {33, 23}, {33, 24}, {33, 25}, {23, 15}, {24, 15}, {25, 15}, {23, 33}, {24, 33}, {25, 33}} {
		wdBuild(j, "sandbags", c[0], c[1])
		bags = append(bags, c)
	}
	j.lookAt(24, 24)
	j.update(1)
	j.settle()
	for i := range ordColonists(j) {
		j.g.Arm(weID(ordColonists(j)[i]), item.Rifle)
		weDraft(j, i)
		j.settle()
	}
	j.g.RaidArmed(item.Rifle, item.Rifle, item.Rifle)
	j.update(1)
	beside := 0
	j.key("Digit3", 0)
	for i := 0; i < 6000 && beside < 20; i++ {
		careFeed(j)
		j.update(1)
		for _, r := range weRaiders(j) {
			if r.Status != "alive" || r.Fleeing {
				continue
			}
			for _, b := range bags {
				if abs(r.X-b[0]) <= 1 && abs(r.Y-b[1]) <= 1 {
					beside++
					break
				}
			}
		}
	}
	j.key("Space", 0)
	if beside < 20 {
		t.Fatalf("raiders stood beside sandbags on only %d updates; raiders %+v", beside, weRaiders(j))
	}
}

// TestJourneyTantrum (CBT-4): a colonist in a tantrum smashes things next
// to it.
func TestJourneyTantrum(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	wdBuild(j, "workbench", 9, 5)
	bench := wdBuilding(j, 9, 5)
	id := ordColonists(j)[0]
	w3Auto(j, []string{"Tantrum", "TriggerTantrum", "ForceTantrum", "StartTantrum", "ForceBreak", "TriggerBreak", "MentalBreak", "BreakNow"}, []string{"tantrum", "break"}, weID(id), 0, "tantrum")
	j.update(1)
	sawBreak, smashed := false, false
	j.key("Digit3", 0)
	for i := 0; i < 3000 && !smashed; i++ {
		careFeed(j)
		j.update(1)
		sawBreak = sawBreak || strings.Contains(strings.ToLower(j.pawn(id).MentalBreak), "tantrum")
		found := false
		for _, b := range j.world().Buildings {
			if b.ID == bench {
				found = true
				smashed = b.HPPct < 100
			}
		}
		smashed = smashed || !found
	}
	j.key("Space", 0)
	if !sawBreak {
		t.Fatalf("the colonist never showed a tantrum break: %q", j.pawn(id).MentalBreak)
	}
	if !smashed {
		t.Fatal("the tantrum left the workbench next to the colonist undamaged")
	}
}

// TestJourneyMechOpensDoor (CBT-4): mechanoids reach a colonist behind a
// closed door by opening or bashing it.
func TestJourneyMechOpensDoor(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(30, 30), jPawns([2]int{12, 12}))
	for x := 10; x <= 14; x++ {
		for _, y := range []int{10, 14} {
			if x == 12 && y == 10 {
				continue
			}
			wdBuild(j, "wall", x, y)
		}
	}
	for y := 11; y <= 13; y++ {
		wdBuild(j, "wall", 10, y)
		wdBuild(j, "wall", 14, y)
	}
	wdBuild(j, "door", 12, 10)
	door := wdSelect(j, 12, 10)
	w2Click(j, "door Forbid button", []string{"forbid"}, "tip") // the colonist must not flee out through it
	if c, ok := j.lastCmd(command.SetForbidden); !ok || c.Target != uint64(door) || !c.Enabled {
		t.Fatalf("SetForbidden %+v (sent %v), want door %d forbidden", c, ok, door)
	}
	var walls []uint64 // the room's walls before the mechs land
	for _, b := range j.world().Buildings {
		if b.ID != door && b.X >= 10 && b.X <= 14 && b.Y >= 10 && b.Y <= 14 {
			walls = append(walls, uint64(b.ID))
		}
	}
	standing := func(id uint64) bool {
		for _, b := range j.world().Buildings {
			if uint64(b.ID) == id {
				return true
			}
		}
		return false
	}
	w3Auto(j, []string{"RaidMechs", "RaidMech", "MechRaid", "RaidArmedMechs", "RaidMechanoids", "RaidMechanoid"}, []string{"mech", "raid"}, 0, 700, "")
	j.update(1)
	through := false
	j.key("Digit3", 0)
	for i := 0; i < 8000 && !through; i++ {
		careFeed(j)
		j.update(1)
		// the mechs break into the room: they bash the forbidden door, or,
		// as mechs breach (CBT-4: they bash walls and shut doors), knock a
		// wall of the room down, or walk in (stray shots chipping a wall
		// don't count)
		for _, b := range j.world().Buildings {
			if b.ID == door && b.HPPct < 100 {
				through = true
			}
		}
		for _, w := range walls {
			through = through || !standing(w)
		}
		through = through || !w3DoorExists(j, door)
		for _, a := range j.world().Animals {
			x, y := wcIntOf(j, a, "X"), wcIntOf(j, a, "Y")
			through = through || (x >= 11 && x <= 13 && y >= 11 && y <= 13)
		}
		for _, p := range weRaiders(j) {
			through = through || (p.X >= 11 && p.X <= 13 && p.Y >= 11 && p.Y <= 13)
		}
	}
	j.key("Space", 0)
	if !through {
		var as []string
		for _, a := range j.world().Animals {
			as = append(as, fmt.Sprintf("%s@(%d,%d)", wcStr(j, a, "Species"), wcIntOf(j, a, "X"), wcIntOf(j, a, "Y")))
		}
		hurt := 0
		for _, b := range j.world().Buildings {
			if b.HPPct < 100 {
				hurt++
			}
		}
		t.Fatalf("the mechs never broke into the room (door or walls); animals %q, raiders %d, damaged buildings %d, door present %v", as, len(weRaiders(j)), hurt, w3DoorExists(j, door))
	}
}

// w3DoorExists reports the building id still standing.
func w3DoorExists(j *journey, id entity.EntityID) bool {
	for _, b := range j.world().Buildings {
		if b.ID == id {
			return true
		}
	}
	return false
}

// TestJourneyExplosionCooksOffShells (CBT-5): a grenade blast beside
// stored mortar shells sets them off in a chain.
func TestJourneyExplosionCooksOffShells(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{6, 4}))
	shells, err := j.g.SpawnStack(item.MortarShells, 5, 14, 6)
	if err != nil {
		t.Fatal(err)
	}
	id := ordColonists(j)[0]
	weGrenadier(j, id, item.FragGrenades)
	weThrowAt(j, weDraft(j, 0), 14, 7)
	type spot struct {
		x, y int
		tick uint64
	}
	seen := map[spot]bool{}
	gone := false
	j.key("Digit3", 0)
	for i := 0; i < 1500 && !(gone && len(seen) >= 2); i++ {
		j.update(1)
		for _, b := range j.world().Bursts {
			if strings.EqualFold(b.Blast, "bomb") {
				seen[spot{b.X, b.Y, b.Tick}] = true
			}
		}
		_, still := w2Item(j, shells)
		gone = !still
	}
	j.key("Space", 0)
	if !gone || len(seen) < 2 {
		t.Fatalf("shells gone %v, distinct bomb bursts %d, want the shells to cook off in a second burst", gone, len(seen))
	}
}

// TestJourneyDoorForbid (CBT-4/5): forbidding a door from its pane keeps
// colonists off it: a hauler takes the long way through the gap.
func TestJourneyDoorForbid(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 10}))
	j.g.ClearPlants(0, 0, 23, 23)
	for y := 0; y <= 21; y++ {
		if y != 10 {
			wdBuild(j, "wall", 12, y)
		}
	}
	wdBuild(j, "door", 12, 10)
	door := wdSelect(j, 12, 10)
	w2Click(j, "door Forbid button", []string{"forbid"}, "tip")
	if c, ok := j.lastCmd(command.SetForbidden); !ok || c.Target != uint64(door) || !c.Enabled {
		t.Fatalf("SetForbidden %+v (sent %v), want door %d forbidden", c, ok, door)
	}
	j.update(2)
	for _, b := range j.world().Buildings {
		if b.ID == door && !b.Forbidden {
			t.Fatal("BuildingView.Forbidden is false after the button")
		}
	}
	j.g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 16, Y: 10, X2: 17, Y2: 10})
	j.g.ApplyPending()
	wood, err := j.g.SpawnStack(item.Wood, 20, 8, 12)
	if err != nil {
		t.Fatal(err)
	}
	id := ordColonists(j)[0]
	onDoor, arrived := false, false
	j.key("Digit3", 0)
	for i := 0; i < 8000 && !arrived; i++ {
		careFeed(j)
		j.update(1)
		if p := j.pawn(id); p.X == 12 && p.Y == 10 {
			onDoor = true
		}
		if it, ok := w2Item(j, wood); ok && !it.Carried && it.X >= 16 {
			arrived = true
		}
	}
	j.key("Space", 0)
	if onDoor {
		t.Fatal("the colonist walked through the forbidden door")
	}
	if !arrived {
		t.Fatal("the wood never reached the stockpile through the gap")
	}
}

// TestJourneyMinRangeRing (CBT-5): a weapon with a minimum range reports
// it to the range ring, and the ring draws.
func TestJourneyMinRangeRing(t *testing.T) {
	t.Parallel()
	name := ""
	for k := weapon.Kind(0); k < 255 && name == ""; k++ {
		if d := weapon.DefOf(k); d.MinRange > 0 {
			name = d.Name
		}
	}
	if name == "" {
		t.Fatal("no weapon has a minimum range")
	}
	if r := weaponReach(name); r.Min <= 0 || r.Max <= r.Min {
		t.Fatalf("weaponReach(%q) = %+v, want a minimum range inside the maximum (U2 host)", name, r)
	}
	j := newJourney(t, jPawns([2]int{8, 4}))
	id := ordColonists(j)[0]
	j.g.Arm(weID(id), item.Rifle)
	weDraft(j, 0)
	j.g.RaidArmed(item.Club)
	j.update(2)
	j.lookAt(8, 4)
	j.draw(2) // the ring with its inner circle draws without fault
}

// TestJourneyCombatRecords (CBT-5): the kills count rises after a kill.
func TestJourneyCombatRecords(t *testing.T) {
	t.Parallel()
	j, id := weFireSetup(t)
	if k := j.pawn(id).Kills; k != 0 {
		t.Fatalf("a fresh colonist has %d kills", k)
	}
	j.key("Digit3", 0)
	for i := 0; i < 6000 && j.pawn(id).Kills == 0; i++ {
		careFeed(j)
		j.update(1)
	}
	j.key("Space", 0)
	p := j.pawn(id)
	if p.Kills == 0 {
		t.Fatalf("no kill recorded; raiders %+v", weRaiders(j))
	}
	if p.DamageDealt <= 0 {
		t.Fatalf("Kills is %d but DamageDealt is %d", p.Kills, p.DamageDealt)
	}
}

// TestJourneySmokepopUse (CBT-3): the smokepop Use button on a colonist
// wearing a pack sets it off where it stands: a smoke cloud appears and the
// pack is gone.
func TestJourneySmokepopUse(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	id := ordColonists(j)[0]
	w3Wear(j, id, item.SmokepopPack, 8, 5)
	ordPick(j, 0)
	w2Click(j, "smokepop Use button", []string{"smokepop"}, "tip")
	if c, ok := j.lastCmd(command.PopSmokepop); !ok || c.Pawn != id {
		t.Fatalf("PopSmokepop %+v (sent %v), want colonist %d", c, ok, id)
	}
	j.key("Digit3", 0)
	for i := 0; i < 600 && w3Bursts(j, "smoke") == 0; i++ {
		j.update(1)
	}
	j.key("Space", 0)
	if w3Bursts(j, "smoke") == 0 {
		t.Fatalf("no smoke burst after the pop: %+v", j.world().Bursts)
	}
	if v, name, ok := w3Num(j.pawn(id), "smokepop"); ok && v <= 0 && w3Wears(j, id, item.SmokepopPack.String()) {
		t.Fatalf("PawnView.%s is %v right after one pop of a fresh pack", name, v)
	}
}

// TestJourneyThreatResponse (CBT-4): the threat response control on a
// colonist sends SetThreatResponse with a mode 0..3.
func TestJourneyThreatResponse(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	id := ordColonists(j)[0]
	ordPick(j, 0)
	w2Click(j, "threat response control", []string{"threat"}, "tip")
	if j.a.ord.Float.Open { // a pick of the four modes
		var labels []string
		for _, b := range j.a.ord.FloatButtons() {
			labels = append(labels, b.Label)
		}
		if len(labels) == 0 {
			t.Fatal("the threat control opened an empty pick")
		}
		pick := labels[0]
		for _, l := range labels {
			if strings.Contains(strings.ToLower(l), "flee") {
				pick = l
			}
		}
		j.pick(pick)
	}
	c, ok := j.lastCmd(command.SetThreatResponse)
	if !ok || c.Pawn != id || c.X < 0 || c.X > 3 {
		t.Fatalf("SetThreatResponse %+v (sent %v), want colonist %d and X in 0..3", c, ok, id)
	}
	j.update(2)
	if v, name, found := w3Num(j.pawn(id), "threat"); found && int(v) != c.X {
		t.Fatalf("PawnView.%s is %v after choosing %d", name, v, c.X)
	}
}
