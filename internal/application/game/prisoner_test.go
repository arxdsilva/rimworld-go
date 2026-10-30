package game

import (
	"bytes"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/prison"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// spawnRaiderAt puts one raider of faction fac (1 + index; 0: none) on
// (x, y), as raidFrom does.
func spawnRaiderAt(t *testing.T, g *Game, x, y, fac int) entity.EntityID {
	t.Helper()
	id := g.entities.Next()
	g.raiderIDs = append(g.raiderIDs, id)
	g.positions.Set(id, Position{X: x, Y: y})
	g.names.Set(id, "Krell")
	g.jobs.Set(id, job.Record{})
	g.need.Set(id, needs.NewFull())
	g.vital.Set(id, health.Record{Status: health.Alive})
	g.equip[id] = item.Rifle
	g.raiders[id] = raid.Raider{Arrived: uint64(g.clock.Now())}
	g.raiderPerson(id, fac)
	return id
}

// prisonGame is a colony of Ada (and Bo) beside a prisoner bed at (3, 3),
// and a downed raider at (12, 6). Everyone works around the clock.
func prisonGame(t *testing.T) (g *Game, ada, bo, rid, bed entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 5, Width: 24, Height: 14})
	ada, _ = g.SpawnNamedPawn(8, 6, "Ada")
	bo, _ = g.SpawnNamedPawn(9, 9, "Bo")
	scheduleAll(g, schedule.Work)
	placeBuilt(t, g, construction.SleepingSpot, construction.South, 3, 3)
	bed = g.bldgIDs[len(g.bldgIDs)-1]
	g.Enqueue(command.Command{Kind: command.SetBedForPrisoners, Target: uint64(bed), Enabled: true})
	g.Step(1)
	if b, _ := g.bldgs.Get(bed); !b.ForPrisoners {
		t.Fatal("SetBedForPrisoners should mark the bed")
	}
	rid = spawnRaiderAt(t, g, 12, 6, 0)
	g.hurt(rid, health.DownedInjury+10, 0)
	g.Step(1)
	if v, _ := g.vital.Get(rid); v.Status != health.Downed {
		t.Fatalf("raider should be downed: %+v", v)
	}
	return g, ada, bo, rid, bed
}

// fed keeps the colonists' needs full.
func fed(g *Game, ids ...entity.EntityID) {
	for _, id := range ids {
		g.SetNeeds(id, needs.Full, needs.Full)
	}
}

// capture runs a capture of rid by ada to the end.
func capture(t *testing.T, g *Game, ada, bo, rid entity.EntityID) {
	t.Helper()
	g.Enqueue(command.Command{Kind: command.Capture, Pawn: uint64(ada), Target: uint64(rid)})
	for i := 0; i < 2000 && !g.prison.Is(rid); i++ {
		g.Step(1)
		fed(g, ada, bo)
	}
	if !g.prison.Is(rid) {
		j, _ := g.jobs.Get(ada)
		t.Fatalf("raider never became a prisoner (captor job %+v)", j)
	}
}

func TestCaptureCarriesDownedRaiderToPrisonBed(t *testing.T) {
	g, ada, bo, rid, bed := prisonGame(t)
	g.Enqueue(command.Command{Kind: command.Capture, Pawn: uint64(ada), Target: uint64(rid)})
	carried := false
	for i := 0; i < 2000 && !g.prison.Is(rid); i++ {
		g.Step(1)
		fed(g, ada, bo)
		if g.care.Carry.Carrier(rid) == ada {
			carried = true
			if pp, ap := g.positionOf(rid), g.positionOf(ada); pp != ap {
				t.Fatal("a captive moves with its captor")
			}
		}
	}
	if !carried || !g.prison.Is(rid) {
		t.Fatalf("capture: carried %v, prisoner %v", carried, g.prison.Is(rid))
	}
	if !g.inBed(rid) || g.sleptAt[rid] != bed {
		t.Fatal("the prisoner lies in the prisoner bed")
	}
	if p := g.personOf(rid); p == nil || p.Where != wherePrisoner {
		t.Fatalf("registry: %+v", p)
	}
	st := g.prison.State(rid)
	if st.Mode != prisoner.Recruit || st.Resistance < prisoner.MinInitialResistance || st.Resistance > prisoner.MaxInitialResistance ||
		st.Captor != uint64(ada) {
		t.Fatalf("state %+v", *st)
	}
	if _, armed := g.equip[rid]; armed {
		t.Fatal("a captive is disarmed")
	}
	if _, n := g.Raiders(); n != 0 || g.standingRaider(rid) {
		t.Fatal("a prisoner is no raider")
	}
	var pv *PawnView
	for _, v := range g.Snapshot() {
		if v.ID == rid {
			pv = &v
		}
	}
	if pv == nil || !pv.Prisoner || pv.Hostile || !pv.InBed {
		t.Fatalf("prisoner view %+v", pv)
	}
	if v, ok := g.Prisoner(rid); !ok || !v.ForPrisonersBed || v.Mode != uint8(prisoner.Recruit) || v.Resistance <= 0 {
		t.Fatalf("Prisoner(%d) = %+v %v", rid, v, ok)
	}
	if !slices.ContainsFunc(pv.Thoughts, func(th ThoughtView) bool { return th.Label == mood.DefOf(mood.Imprisoned).Label }) {
		t.Fatalf("a prisoner thinks it is imprisoned: %+v", pv.Thoughts)
	}
	// it no longer bleeds out
	for i := 0; i < int(6*clock.TicksPerHour)/500; i++ {
		g.Step(500)
		fed(g, ada, bo)
		g.SetNeeds(rid, needs.Full, needs.Full)
	}
	if v, _ := g.vital.Get(rid); v.Status == health.Dead {
		t.Fatal("a prisoner in bed does not bleed out")
	}
}

func (g *Game) positionOf(id entity.EntityID) Position {
	p, _ := g.positions.Get(id)
	return p
}

func TestCaptureNeedsAPrisonerBed(t *testing.T) {
	g, ada, _, rid, bed := prisonGame(t)
	g.Enqueue(command.Command{Kind: command.SetBedForPrisoners, Target: uint64(bed), Enabled: false})
	g.Enqueue(command.Command{Kind: command.Capture, Pawn: uint64(ada), Target: uint64(rid)})
	g.Step(1)
	if j, _ := g.jobs.Get(ada); j.Kind == job.Capture {
		t.Fatal("no prisoner bed: no capture")
	}
	ms := g.World().Messages
	if len(ms) == 0 || ms[len(ms)-1].Text != "No free prisoner bed for Krell." {
		t.Fatalf("messages %+v", ms)
	}
}

func TestResistanceFactorEndpointsDriveAVisit(t *testing.T) {
	for _, c := range []struct {
		op   int32
		mood int
		want int32
	}{{-10000, 50, 500}, {0, 50, 1000}, {10000, 50, 1500}, {0, 0, 200}, {0, 100, 1500}} {
		if got := prisoner.ResistanceFactor(c.op, c.mood); got != c.want {
			t.Fatalf("factor(%d, %d) = %d, want %d", c.op, c.mood, got, c.want)
		}
	}
	g, ada, bo, rid, _ := prisonGame(t)
	capture(t, g, ada, bo, rid)
	st := g.prison.State(rid)
	st.Mode, st.Resistance = prisoner.Reduce, 20000
	want := prisoner.Reduction(g.negotiation(bo), prisoner.ResistanceFactor(g.opinion(rid, bo), g.prisonerMood(rid)))
	g.prison.Chat(bo, rid, uint64(g.clock.Now()))
	if got := 20000 - st.Resistance; got != want || want <= 0 {
		t.Fatalf("a visit takes %d, want %d", got, want)
	}
	if st.NextVisit != uint64(g.clock.Now())+prisoner.VisitInterval {
		t.Fatal("the next visit waits an interval")
	}
	if g.raiders[rid].Warden != 0 {
		t.Fatal("Chat alone does not name the warden (Do does)")
	}
}

func TestWardenVisitsOncePerIntervalNoFlap(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	capture(t, g, ada, bo, rid)
	g.setPriority(bo, work.Warden, 1)
	st := g.prison.State(rid)
	st.Mode, st.Resistance, st.NextVisit = prisoner.Reduce, prisoner.MaxInitialResistance, uint64(g.clock.Now())
	visits, last := 0, st.Resistance
	var lastKind job.Kind
	starts := 0
	for i := 0; i < int(2*prisoner.VisitInterval)+500; i++ {
		g.Step(1)
		fed(g, ada, bo)
		g.SetNeeds(rid, needs.Full, needs.Full)
		if st.Resistance != last {
			visits++
			last = st.Resistance
		}
		j, _ := g.jobs.Get(bo)
		if j.Kind == job.Warden && lastKind != job.Warden {
			starts++
		}
		lastKind = j.Kind
	}
	if visits < 2 || visits > 3 {
		t.Fatalf("visits in two intervals = %d, want 2-3", visits)
	}
	if starts > 4 {
		t.Fatalf("warden job started %d times: flapping", starts)
	}
	if v, _ := g.Prisoner(rid); v.Warden != "Bo" {
		t.Fatalf("warden of record %q", v.Warden)
	}
}

func TestPrisonerFoodBandHysteresis(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	capture(t, g, ada, bo, rid)
	band := func(h int) bool {
		g.prison.FoodBand(rid, needs.Pair{Hunger: h, Rest: needs.Full})
		return g.raiders[rid].Hungry
	}
	if band(prison.FoodLow + 1) {
		t.Fatal("above the low line: no meal yet")
	}
	if !band(prison.FoodLow) {
		t.Fatal("at the low line: wants a meal")
	}
	if !band(prison.FoodFed - 1) {
		t.Fatal("inside the band it keeps wanting until fed")
	}
	if band(prison.FoodFed) {
		t.Fatal("fed: no more")
	}
	if band(prison.FoodFed - 1) {
		t.Fatal("inside the band after a meal: no new request")
	}
}

func TestWardenBringsFoodOnceNoFlap(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	capture(t, g, ada, bo, rid)
	g.setPriority(bo, work.Warden, 1)
	if _, err := g.SpawnStack(item.Meal, 5, 10, 10); err != nil {
		t.Fatal(err)
	}
	g.prison.State(rid).NextVisit = 1 << 62 // food only
	g.SetNeeds(rid, prison.FoodLow-100, needs.Full)
	deliveries := 0
	var last job.Kind
	for i := 0; i < 3000; i++ {
		g.Step(1)
		fed(g, ada, bo)
		j, _ := g.jobs.Get(bo)
		if j.Kind == job.DeliverFood && last != job.DeliverFood {
			deliveries++
		}
		last = j.Kind
	}
	if n, _ := g.need.Get(rid); n.Hunger < prison.FoodFed {
		t.Fatalf("the prisoner should be fed: hunger %d", n.Hunger)
	}
	if deliveries != 1 || g.raiders[rid].Hungry {
		t.Fatalf("deliveries %d, hungry %v", deliveries, g.raiders[rid].Hungry)
	}
}

func TestRecruitKeepsFamilyAndRemembersWarden(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	g.ensurePerson(ada)
	g.setParents(rid, [2]entity.EntityID{ada, 0}) // Ada is the raider's mother
	capture(t, g, ada, bo, rid)
	g.sk.skills[bo][skill.Social].Level = 20 // a sure recruit
	g.setPriority(bo, work.Warden, 1)
	st := g.prison.State(rid)
	st.Resistance, st.NextVisit = 0, uint64(g.clock.Now())
	for i := 0; i < 2000 && g.prison.Is(rid); i++ {
		g.Step(1)
		fed(g, ada, bo)
	}
	if g.prison.Is(rid) || !slices.Contains(g.pawnIDs, rid) || slices.Contains(g.raiderIDs, rid) {
		v, _ := g.vital.Get(rid)
		t.Fatalf("the recruit should be a colonist: prisoner %v pawn %v raider %v vital %+v", g.prison.Is(rid), slices.Contains(g.pawnIDs, rid), slices.Contains(g.raiderIDs, rid), v)
	}
	if !slices.IsSorted(g.pawnIDs) {
		t.Fatal("pawnIDs stay ascending")
	}
	p := g.personOf(rid)
	if p.Where != whereColony || p.Faction != 0 || p.HasBio {
		t.Fatalf("registry %+v", p)
	}
	if _, ok := g.sk.bios[rid]; !ok {
		t.Fatal("a recruit has a colonist bio")
	}
	if !slices.Contains(g.kinOf(ada, rid), social.Child) {
		t.Fatalf("Ada keeps her child: %v", g.kinOf(ada, rid))
	}
	e := g.edgeOf(rid, bo)
	if e == nil || !social.HasMemory(e.mem[e.dir(rid)], social.MemRecruitedMe, social.ID(bo), uint64(g.clock.Now())) {
		t.Fatal("the recruit remembers who won it over")
	}
	if g.isRaider(rid) || !g.colonist(rid) {
		t.Fatal("the recruit is a colonist everywhere")
	}
	letter := false
	for _, m := range g.World().Messages {
		letter = letter || m.Letter && m.Title == "Recruited" && m.Focus == rid
	}
	if !letter || len(g.history) == 0 || g.history[len(g.history)-1].Kind != "recruit" {
		t.Fatal("recruiting posts a letter and a history line")
	}
}

func TestReleaseWalksOffAndPleasesFaction(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	g.ng.factions = []faction.Faction{{Type: faction.Settlers, Name: "Valley", Goodwill: -20}}
	g.personOf(rid).Faction = 1
	capture(t, g, ada, bo, rid)
	g.Enqueue(command.Command{Kind: command.ReleasePrisoner, Target: uint64(rid)})
	for i := 0; i < 4000 && slices.Contains(g.raiderIDs, rid); i++ {
		g.Step(1)
		fed(g, ada, bo)
		g.SetNeeds(rid, needs.Full, needs.Full)
		if v, _ := g.vital.Get(rid); v.Status == health.Downed {
			v.Status, v.Cause, v.Injury = health.Alive, health.CauseNone, 0 // let it walk
			g.vital.Set(rid, v)
		}
	}
	if slices.Contains(g.raiderIDs, rid) {
		t.Fatalf("the released prisoner should leave; at %+v job %+v", g.positionOf(rid), g.jobsOf(rid))
	}
	f := g.ng.factions[0]
	if f.Goodwill != -20+releaseGoodwill || !slices.Contains(f.Events, "prisoner released") {
		t.Fatalf("faction %+v", f)
	}
	if p := g.personOf(rid); p == nil || p.Where != whereOffMap {
		t.Fatalf("registry %+v", p)
	}
}

func (g *Game) jobsOf(id entity.EntityID) job.Record {
	j, _ := g.jobs.Get(id)
	return j
}

func TestExecutionGivesRelativesKillerOpinion(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	g.ensurePerson(ada)
	g.setParents(rid, [2]entity.EntityID{ada, 0})
	capture(t, g, ada, bo, rid)
	g.setPriority(bo, work.Warden, 1)
	g.Enqueue(command.Command{Kind: command.SetPrisonerMode, Target: uint64(rid), Mode: uint8(prisoner.Execute)})
	for i := 0; i < 2000 && g.prison.Is(rid); i++ {
		g.Step(1)
		fed(g, ada, bo)
	}
	if v, _ := g.vital.Get(rid); v.Status != health.Dead || v.Cause != health.CauseExecution {
		t.Fatalf("the prisoner should be executed: %+v", v)
	}
	e := g.edgeOf(ada, bo)
	if e == nil || !social.HasMemory(e.mem[e.dir(ada)], social.MemKilledMyParentChild, social.ID(bo), uint64(g.clock.Now())) {
		t.Fatal("the mother holds the execution against the warden")
	}
	if g.personOf(rid).KilledBy != bo {
		t.Fatal("the registry names the executioner")
	}
}

func TestArrestSuccessAndFailure(t *testing.T) {
	g, ada, bo, _, _ := prisonGame(t)
	// asleep: the arrest is sure
	g.jobs.Set(bo, job.Record{Kind: job.Sleep, State: job.Do})
	if c := g.arrestChance(ada, bo); c != 1000 {
		t.Fatalf("arresting a sleeper: %d", c)
	}
	g.jobs.Set(bo, job.Record{})
	if c := g.arrestChance(ada, bo); c != arrestBase+arrestPerSocial*int(g.skillOf(ada, skSocial).Level) {
		t.Fatalf("awake: %d", c)
	}
	g.sk.stats[ada].sheet[stat.ArrestSuccess] = 0 // Ada cannot hold anyone
	g.Enqueue(command.Command{Kind: command.Arrest, Pawn: uint64(ada), Target: uint64(bo)})
	failed := false
	for i := 0; i < 600 && !failed; i++ {
		g.Step(1)
		fed(g, ada, bo)
		failed = g.jobsOf(ada).Kind == job.Arrest && g.jobsOf(ada).State == job.Fail || g.mind.InFight(ada)
	}
	if !failed || g.prison.Is(bo) || !slices.Contains(g.pawnIDs, bo) {
		t.Fatalf("a failed arrest leaves Bo a colonist (failed %v)", failed)
	}
	if !g.mind.InFight(ada) || !g.mind.InFight(bo) {
		t.Fatal("Bo fights off the failed arrest")
	}
	for i := 0; i < 20000 && (g.mind.InFight(ada) || g.mind.InFight(bo)); i++ {
		g.Step(1)
		fed(g, ada, bo)
	}
	if g.mind.InFight(ada) || g.mind.InFight(bo) {
		t.Fatal("the resisted-arrest fight never ended")
	}
	for _, id := range []entity.EntityID{ada, bo} { // patch them up for the second try
		if v, ok := g.vital.Get(id); ok && v.Status == health.Downed {
			v.Status, v.Injury = health.Alive, 0
			g.vital.Set(id, v)
			delete(g.bodies, id)
		}
	}
	g.sk.stats[ada].sheet[stat.ArrestSuccess] = 1000
	g.sk.skills[ada][skill.Social].Level = 20 // 980 permille
	for try := 0; try < 6 && !g.prison.Is(bo); try++ {
		g.Enqueue(command.Command{Kind: command.Arrest, Pawn: uint64(ada), Target: uint64(bo)})
		for i := 0; i < 600 && !g.prison.Is(bo); i++ {
			g.Step(1)
			fed(g, ada, bo)
		}
	}
	if !g.prison.Is(bo) || slices.Contains(g.pawnIDs, bo) || !slices.Contains(g.raiderIDs, bo) || !slices.IsSorted(g.raiderIDs) {
		t.Fatal("an arrested colonist is the colony's prisoner, off the roster")
	}
	for i := 0; i < 2000 && !g.inBed(bo); i++ {
		g.Step(1)
		fed(g, ada)
	}
	if !g.inBed(bo) || g.personOf(bo).Where != wherePrisoner {
		t.Fatal("the arrested colonist is carried to the prisoner bed")
	}
	if g.colonist(bo) {
		t.Fatal("a prisoner is not a colonist")
	}
}

func TestSaveLoadKeepsPrisoners(t *testing.T) {
	g, ada, bo, rid, _ := prisonGame(t)
	capture(t, g, ada, bo, rid)
	g.SetNeeds(rid, prison.FoodLow-1, needs.Full)
	g.Step(1)
	st := *g.prison.State(rid)
	var buf bytes.Buffer
	if err := g.Save(&buf, "prison"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !h.prison.Is(rid) || *h.prison.State(rid) != st || !h.raiders[rid].Hungry {
		t.Fatalf("loaded prisoner %+v / %+v", h.prison.State(rid), h.raiders[rid])
	}
	if p := h.personOf(rid); p == nil || p.Where != wherePrisoner {
		t.Fatal("the registry keeps the prisoner")
	}
	for i := 0; i < 3; i++ {
		g.Step(300)
		h.Step(300)
		sameState(t, "prison", g, h)
	}
}
