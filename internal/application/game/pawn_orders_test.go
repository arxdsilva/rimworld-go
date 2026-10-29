package game

// pawn_orders_test.go: the pawn-targeted right-click orders (SIM-062)
// through the whole game: Rescue, Capture, Arrest, Prioritize tending and
// the drafted Tend, their menu rows, reasons and queueing.

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// orderRow is the menu row at (x, y) for pawn whose label starts with
// prefix, or nil.
func orderRow(g *Game, pawn entity.EntityID, x, y int, prefix string) *OrderOption {
	opts := g.OrdersAt(pawn, x, y)
	for i := range opts {
		if strings.HasPrefix(opts[i].Label, prefix) {
			return &opts[i]
		}
	}
	return nil
}

// tileOf is where id stands now (pawns wander off their spawn tile).
func tileOf(g *Game, id entity.EntityID) (int, int) {
	p, _ := g.positions.Get(id)
	return p.X, p.Y
}

func labelsAt(g *Game, pawn entity.EntityID, x, y int) []string {
	var out []string
	for _, o := range g.OrdersAt(pawn, x, y) {
		out = append(out, o.Label)
	}
	return out
}

func prioritize(o command.Order, doer, target entity.EntityID, x, y int, queue bool) command.Command {
	return command.Command{Kind: command.Prioritize, Order: o, Pawn: uint64(doer), Target: uint64(target), X: x, Y: y, Queue: queue}
}

// rescueScene is Doc beside a downed Bo, with a hospital bed and a plain
// bed (the plain one nearer to Bo).
func rescueScene(t *testing.T) (g *Game, doc, bo, hosp, plain entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 3, Width: 40, Height: 14})
	doc, _ = g.SpawnNamedPawn(6, 6, "Doc")
	bo, _ = g.SpawnNamedPawn(12, 6, "Bo")
	hosp = buildNow(t, g, construction.HospitalBed, item.Wood, construction.South, 24, 4)
	plain = buildNow(t, g, construction.Bed, item.Wood, construction.South, 14, 3)
	g.hurt(bo, health.DownedInjury+10, 0)
	g.Step(1)
	if v, _ := g.vital.Get(bo); v.Status != health.Downed {
		t.Fatalf("Bo should be downed: %+v", v)
	}
	return g, doc, bo, hosp, plain
}

func TestPawnOrdersRescueRow(t *testing.T) {
	g, doc, bo, _, _ := rescueScene(t)
	r := orderRow(g, doc, 12, 6, "Rescue Bo")
	if r == nil || r.Disabled || r.Cmd.Kind != command.Prioritize || r.Cmd.Order != command.OrderRescue || r.Cmd.Target != uint64(bo) || r.Cmd.Pawn != uint64(doc) {
		t.Fatalf("Rescue Bo row: %+v in %q", r, labelsAt(g, doc, 12, 6))
	}
	// a standing colonist is not rescued, a downed one is not arrested or captured
	if orderRow(g, bo, 6, 6, "Rescue Doc") != nil {
		t.Fatal("no Rescue for a standing colonist")
	}
	for _, p := range []string{"Arrest", "Capture"} {
		if orderRow(g, doc, 12, 6, p) != nil {
			t.Fatalf("%s offered on a downed colonist: %q", p, labelsAt(g, doc, 12, 6))
		}
	}
	if r := orderRow(g, doc, 6, 6, "Rescue"); r != nil {
		t.Fatalf("Rescue offered on oneself: %+v", r)
	}
}

func TestPawnOrdersRescueNoFreeBed(t *testing.T) {
	g := New(Config{Seed: 3, Width: 40, Height: 14})
	doc, _ := g.SpawnNamedPawn(6, 6, "Doc")
	g.SpawnNamedPawn(12, 6, "Bo")
	bo := g.pawnIDs[1]
	g.hurt(bo, health.DownedInjury+10, 0)
	g.Step(1)
	r := orderRow(g, doc, 12, 6, "Rescue Bo")
	if r == nil || !r.Disabled || r.Reason != care.ReasonNoBed {
		t.Fatalf("with no bed Rescue is disabled %q: %+v", care.ReasonNoBed, r)
	}
}

func TestPawnOrdersRescueCarriedBy(t *testing.T) {
	g, doc, bo, _, _ := rescueScene(t)
	al, _ := g.SpawnNamedPawn(11, 7, "Al")
	g.Enqueue(prioritize(command.OrderRescue, doc, bo, 12, 6, false))
	if !runCare(g, 300, func() bool { return g.care.Carry.Carrier(bo) == doc }, doc, al) {
		t.Fatal("Doc never picks Bo up")
	}
	bp, _ := g.positions.Get(bo)
	r := orderRow(g, al, bp.X, bp.Y, "Rescue Bo")
	if r == nil || !r.Disabled || !strings.HasPrefix(r.Reason, care.ReasonCarried) {
		t.Fatalf("a carried patient shows a Rescue row disabled %q...: %+v", care.ReasonCarried, r)
	}
	if !strings.Contains(r.Reason, "Doc") {
		t.Fatalf("the reason names the carrier: %q", r.Reason)
	}
}

func TestPawnOrdersForcedRescueUndraftedReachesMedicalBed(t *testing.T) {
	g, doc, bo, hosp, plain := rescueScene(t)
	g.setPriority(doc, work.Doctor, 0) // it would never rescue on its own
	g.Enqueue(prioritize(command.OrderRescue, doc, bo, 12, 6, false))
	g.Step(1)
	if j, _ := g.jobs.Get(doc); j.Kind != job.Rescue || j.Target != bo {
		t.Fatalf("the ordered rescue starts: %+v", j)
	}
	seen := false
	ok := runCare(g, 600, func() bool {
		var by, carrying entity.EntityID
		for _, p := range g.Snapshot() {
			if p.ID == bo {
				by = p.CarriedBy
			}
			if p.ID == doc {
				carrying = p.CarryingPawn
			}
		}
		seen = seen || by == doc && carrying == bo
		return g.bedUnder(bo) == hosp
	}, doc)
	if !ok {
		t.Fatalf("Bo ends in the medical bed (in bed %v, plain %v)", g.bedUnder(bo), plain)
	}
	if !seen {
		t.Fatal("the read model showed CarriedBy during the carry")
	}
	if g.care.Carry.Carrier(bo) != 0 {
		t.Fatal("Bo is put down")
	}
}

func TestPawnOrdersForcedRescueDraftedStaysDrafted(t *testing.T) {
	g, doc, bo, hosp, _ := rescueScene(t)
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(doc), Enabled: true})
	g.Step(1)
	r := orderRow(g, doc, 12, 6, "Rescue Bo")
	if r == nil || r.Disabled {
		t.Fatalf("a drafted doer is offered Rescue: %q", labelsAt(g, doc, 12, 6))
	}
	g.Enqueue(r.Cmd)
	if !runCare(g, 600, func() bool { return g.bedUnder(bo) == hosp }, doc) {
		j, _ := g.jobs.Get(doc)
		t.Fatalf("drafted rescue reaches the medical bed (doc job %+v)", j)
	}
	if !g.drafted(doc) {
		t.Fatal("the doer stays drafted")
	}
}

func TestPawnOrdersRescueFallsBackToAnyFreeBed(t *testing.T) {
	g := New(Config{Seed: 3, Width: 40, Height: 14})
	doc, _ := g.SpawnNamedPawn(6, 6, "Doc")
	bo, _ := g.SpawnNamedPawn(12, 6, "Bo")
	plain := buildNow(t, g, construction.Bed, item.Wood, construction.South, 14, 3)
	g.hurt(bo, health.DownedInjury+10, 0)
	g.Step(1)
	g.Enqueue(prioritize(command.OrderRescue, doc, bo, 12, 6, false))
	if !runCare(g, 600, func() bool { return g.bedUnder(bo) == plain }, doc) {
		t.Fatal("with no medical bed the patient goes to a plain free bed")
	}
}

func TestPawnOrdersAutonomousDoctorLeavesOrderedTarget(t *testing.T) {
	g, doc, bo, hosp, _ := rescueScene(t)
	rival, _ := g.SpawnNamedPawn(13, 7, "Rival") // nearer to Bo, a doctor
	g.setPriority(rival, work.Doctor, 1)
	g.setPriority(doc, work.Doctor, 0)
	g.Enqueue(prioritize(command.OrderRescue, doc, bo, 12, 6, false))
	stolen := false
	ok := runCare(g, 600, func() bool {
		if j, _ := g.jobs.Get(rival); j.Kind == job.Rescue && j.Target == bo {
			stolen = true
		}
		return g.bedUnder(bo) == hosp
	}, doc, rival)
	if stolen {
		t.Fatal("the autonomous doctor took a patient another colonist was ordered to rescue")
	}
	if !ok {
		t.Fatal("the ordered rescue still completes")
	}
}

func TestPawnOrdersRescuedByAnotherEndsCleanly(t *testing.T) {
	g, doc, bo, _, _ := rescueScene(t)
	other, _ := g.SpawnNamedPawn(13, 7, "Eve")
	g.Enqueue(prioritize(command.OrderRescue, doc, bo, 12, 6, false))
	g.Enqueue(prioritize(command.OrderRescue, other, bo, 12, 6, false))
	g.Step(1)
	ok := runCare(g, 800, func() bool {
		jd, _ := g.jobs.Get(doc)
		je, _ := g.jobs.Get(other)
		return g.inBed(bo) && g.care.Carry.Carrier(bo) == 0 && jd.Kind != job.Rescue && je.Kind != job.Rescue
	}, doc, other)
	if !ok {
		jd, _ := g.jobs.Get(doc)
		je, _ := g.jobs.Get(other)
		t.Fatalf("both ordered, one carried: patient in bed, no one stuck (doc %+v, eve %+v, carrier %d)", jd, je, g.care.Carry.Carrier(bo))
	}
	if len(g.orderQueue[doc]) != 0 || len(g.orderQueue[other]) != 0 {
		t.Fatal("no leftover queue")
	}
}

func TestPawnOrdersRescueOfPatientWhoDiesEndsCleanly(t *testing.T) {
	g, doc, bo, _, _ := rescueScene(t)
	g.Enqueue(prioritize(command.OrderRescue, doc, bo, 12, 6, false))
	g.Step(1)
	g.hurt(bo, 100000, 0)
	g.Step(1)
	ok := runCare(g, 300, func() bool {
		j, _ := g.jobs.Get(doc)
		return !(j.Kind == job.Rescue && j.Active())
	}, doc)
	if !ok {
		j, _ := g.jobs.Get(doc)
		t.Fatalf("the rescuer drops a dead patient: %+v", j)
	}
	if g.care.Carry.Carrier(bo) == doc {
		t.Fatal("still carrying a dead patient")
	}
	if r := orderRow(g, doc, 12, 6, "Rescue"); r != nil && !r.Disabled {
		t.Fatalf("no live Rescue row for a dead patient: %+v", r)
	}
}

func TestPawnOrdersCaptureRows(t *testing.T) {
	g := New(Config{Seed: 5, Width: 24, Height: 14})
	ada, _ := g.SpawnNamedPawn(8, 6, "Ada")
	rid := spawnRaiderAt(t, g, 12, 6, 0)
	g.hurt(rid, health.DownedInjury+10, 0)
	g.Step(1)
	r := orderRow(g, ada, 12, 6, "Capture Krell")
	if r == nil || !r.Disabled || r.Reason != care.ReasonNoPrisonBed {
		t.Fatalf("no prisoner bed: Capture disabled %q: %+v", care.ReasonNoPrisonBed, r)
	}
	g2, ada2, _, rid2, _ := prisonGame(t)
	r = orderRow(g2, ada2, 12, 6, "Capture Krell")
	if r == nil || r.Disabled || r.Cmd.Kind != command.Prioritize || r.Cmd.Order != command.OrderCapture || r.Cmd.Target != uint64(rid2) {
		t.Fatalf("with a prisoner bed Capture is offered: %+v in %q", r, labelsAt(g2, ada2, 12, 6))
	}
	if orderRow(g2, ada2, 12, 6, "Rescue") != nil || orderRow(g2, ada2, 12, 6, "Arrest") != nil {
		t.Fatalf("a downed raider is captured, not rescued or arrested: %q", labelsAt(g2, ada2, 12, 6))
	}
	g2.Enqueue(r.Cmd)
	for i := 0; i < 2000 && !g2.isPrisoner(rid2); i++ {
		g2.Step(1)
		fed(g2, ada2)
	}
	if !g2.isPrisoner(rid2) {
		t.Fatal("the Capture row's command captures the raider")
	}
}

func TestPawnOrdersArrestRow(t *testing.T) {
	g, ada, bo, _, _ := prisonGame(t)
	bx, by := tileOf(g, bo)
	r := orderRow(g, ada, bx, by, "Arrest Bo")
	if r == nil || r.Disabled || r.Cmd.Kind != command.Prioritize || r.Cmd.Order != command.OrderArrest || r.Cmd.Target != uint64(bo) {
		t.Fatalf("Arrest Bo row: %+v in %q", r, labelsAt(g, ada, bx, by))
	}
	ax, ay := tileOf(g, ada)
	if orderRow(g, ada, ax, ay, "Arrest") != nil {
		t.Fatal("no Arrest on oneself")
	}
	g.Enqueue(r.Cmd)
	started := false
	for i := 0; i < 300 && !started; i++ {
		g.Step(1)
		fed(g, ada, bo)
		started = g.jobsOf(ada).Kind == job.Arrest
	}
	if !started {
		j, _ := g.jobs.Get(ada)
		t.Fatalf("the arrest job starts: %+v", j)
	}
}

func TestPawnOrdersTendRowsByDraft(t *testing.T) {
	g := New(Config{Seed: 3, Width: 40, Height: 12})
	doc, _ := g.SpawnNamedPawn(6, 6, "Doc")
	bo, _ := g.SpawnNamedPawn(12, 6, "Bo")
	g.setPriority(doc, work.Doctor, 0)
	g.InjureCut(bo, 6)
	g.Step(1)
	bx, by := tileOf(g, bo)
	r := orderRow(g, doc, bx, by, "Prioritize tending Bo")
	if r == nil || r.Disabled || r.Cmd.Order != command.OrderTendPatient || r.Cmd.Kind != command.Prioritize || r.Cmd.Target != uint64(bo) {
		t.Fatalf("undrafted: Prioritize tending Bo: %+v in %q", r, labelsAt(g, doc, bx, by))
	}
	if orderRow(g, doc, bx, by, "Tend Bo") != nil {
		t.Fatal("the plain Tend row is for drafted doers")
	}
	g.Enqueue(r.Cmd)
	if !runCare(g, 400, func() bool { return g.tended(bo) }, doc, bo) {
		j, _ := g.jobs.Get(doc)
		t.Fatalf("the prioritized tend is done (doc job %+v)", j)
	}
	g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(doc), Enabled: true})
	g.Step(1)
	g.InjureCut(bo, 6)
	g.Step(1)
	bx, by = tileOf(g, bo)
	if orderRow(g, doc, bx, by, "Prioritize tending") != nil {
		t.Fatal("drafted: no Prioritize tending row")
	}
	if r := orderRow(g, doc, bx, by, "Tend Bo"); r == nil || r.Cmd.Kind != command.EmergencyTend {
		t.Fatalf("drafted: Tend Bo is an EmergencyTend: %+v in %q", r, labelsAt(g, doc, bx, by))
	}
}

func TestPawnOrdersQueuedRescueRunsAfterFirstOrder(t *testing.T) {
	g, doc, bo, hosp, _ := rescueScene(t)
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(doc), X: 4, Y: 9})
	g.Step(1)
	g.Enqueue(prioritize(command.OrderRescue, doc, bo, 12, 6, true))
	g.Step(1)
	q := g.orderQueue[doc]
	if len(q) != 1 || q[0].label != "Rescue Bo" {
		t.Fatalf("the rescue is queued as %q: %+v", "Rescue Bo", q)
	}
	if g.inBed(bo) || g.care.Carry.Carrier(bo) != 0 {
		t.Fatal("the rescue waits for the first order")
	}
	if !runCare(g, 800, func() bool { return g.bedUnder(bo) == hosp }, doc) {
		j, _ := g.jobs.Get(doc)
		t.Fatalf("the queued rescue runs after the move (doc job %+v, queue %+v)", j, g.orderQueue[doc])
	}
	if len(g.orderQueue[doc]) != 0 {
		t.Fatal("the queue empties")
	}
}
