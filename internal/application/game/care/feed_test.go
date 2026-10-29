package care

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// arrive puts feeder id where its job walks to and lets the job go on.
func arrive(h *fakeHost, id entity.EntityID) {
	j := h.jobs[id]
	h.pos[id] = [2]int{j.TX, j.TY}
	j.State = job.Do
	h.jobs[id] = j
}

func TestDoctorFeedsAHungryDownedPatient(t *testing.T) {
	c, h, m := feedScene()
	if !c.Doctor.Assign(doc) {
		t.Fatal("a doctor should feed the hungry downed patient")
	}
	j := h.jobs[doc]
	if j.Kind != job.FeedPatient || j.Target != m.food || j.Dest != patient || j.TX != 1 || j.Forced {
		t.Fatalf("a run to the food for the patient: %+v", j)
	}
	if h.owners[patient] != doc {
		t.Fatal("the patient is held for its feeder")
	}
	arrive(h, doc)
	c.Feed.Do(doc, h.jobs[doc]) // takes the food, heads for the patient
	if j = h.jobs[doc]; j.Carry != m.food || j.State != job.Goto || j.TX != 6 || j.TY != 5 {
		t.Fatalf("carrying the meal to the patient: %+v", j)
	}
	arrive(h, doc)
	c.Feed.Do(doc, h.jobs[doc])
	if j = h.jobs[doc]; j.State != job.Complete || j.Carry != 0 {
		t.Fatalf("fed and done: %+v", j)
	}
	if !slices.Contains(h.log, "feed") || !slices.Equal(m.log, []string{"take", "fed by"}) {
		t.Fatalf("host %v meals %v", h.log, m.log)
	}
}

func TestFeedGiverSkips(t *testing.T) {
	cases := []struct {
		name string
		set  func(c *Care, h *fakeHost, m *fakeMeals)
	}{
		{"above Hungry", func(c *Care, h *fakeHost, m *fakeMeals) { m.hunger[patient] = FeedHungry + 1 }},
		{"standing", func(c *Care, h *fakeHost, m *fakeMeals) { h.vital[patient] = health.Record{Status: health.Alive} }},
		{"dead", func(c *Care, h *fakeHost, m *fakeMeals) { h.vital[patient] = health.Record{Status: health.Dead} }},
		{"a prisoner (the warden's)", func(c *Care, h *fakeHost, m *fakeMeals) { h.prisoners[patient] = true }},
		{"carried", func(c *Care, h *fakeHost, m *fakeMeals) { c.Carry.Pick(other, patient) }},
		{"held by another", func(c *Care, h *fakeHost, m *fakeMeals) { h.owners[patient] = other }},
		{"no way there", func(c *Care, h *fakeHost, m *fakeMeals) { h.noPath = true }},
		{"itself", func(c *Care, h *fakeHost, m *fakeMeals) { m.hunger[doc] = 0; h.pawns = []entity.EntityID{doc} }},
	}
	for _, tc := range cases {
		c, h, m := feedScene()
		tc.set(c, h, m)
		if c.Feed.assign(doc) {
			t.Errorf("%s: no feeding expected, got %+v", tc.name, h.jobs[doc])
		}
	}
	c, h, m := feedScene()
	if New(h, Roles{}).Feed.assign(doc) || New(h, Roles{}).Feed.Start(doc, patient, job.FeedPatient, false) {
		t.Fatal("no Meals role: nobody is fed")
	}
	if !c.Feed.assign(doc) || m.looks != 1 {
		t.Fatal("the scene itself feeds")
	}
}

// TestFeedRetriesAfterNoFood: a run that finds no food leaves the
// patient alone for FeedRetry, then a doctor looks again.
func TestFeedRetriesAfterNoFood(t *testing.T) {
	c, h, m := feedScene()
	m.food = 0
	h.now = 100
	if c.Feed.assign(doc) || c.Feed.RetryAt(patient) != 100+FeedRetry {
		t.Fatalf("no food: retry at %d", c.Feed.RetryAt(patient))
	}
	m.food = 50
	h.now = 100 + FeedRetry - 1
	if c.Feed.assign(doc) {
		t.Fatal("food back, but the patient waits out its retry")
	}
	h.now = 100 + FeedRetry
	if !c.Feed.assign(doc) || c.Feed.RetryAt(patient) != 0 {
		t.Fatal("after the retry a doctor feeds it")
	}
	// a doctor that can't start the run (blocked) counts as a failed run
	c2, h2, _ := feedScene()
	h2.blocked[doc] = true
	if c2.Feed.assign(doc) || c2.Feed.RetryAt(patient) == 0 {
		t.Fatal("a run that could not start holds the patient back")
	}
}

func TestFeedDriverEnds(t *testing.T) {
	run := func(set func(h *fakeHost, m *fakeMeals)) (job.Record, *fakeHost) {
		c, h, m := feedScene()
		c.Feed.assign(doc)
		arrive(h, doc)
		set(h, m)
		c.Feed.Do(doc, h.jobs[doc])
		return h.jobs[doc], h
	}
	if j, h := run(func(h *fakeHost, m *fakeMeals) { h.vital[patient] = health.Record{Status: health.Alive} }); j.State != job.Complete || slices.Contains(h.log, "feed") {
		t.Fatalf("a patient that got up ends the run: %+v", j)
	}
	if j, _ := run(func(h *fakeHost, m *fakeMeals) { m.takeFail = true }); j.State != job.Fail || j.Fail != job.TargetGone {
		t.Fatalf("the food gone fails the run: %+v", j)
	}
	if j, _ := run(func(h *fakeHost, m *fakeMeals) { h.noPath = true }); j.State != job.Fail || j.Fail != job.Unreachable {
		t.Fatalf("no way to the patient fails the run: %+v", j)
	}
	// the patient moved after the meal was picked up: chase, then give up
	c, h, _ := feedScene()
	c.Feed.assign(doc)
	arrive(h, doc)
	c.Feed.Do(doc, h.jobs[doc])
	for i := 0; i <= feedChase; i++ {
		arrive(h, doc)
		h.pos[patient] = [2]int{20 + 3*i, 5}
		c.Feed.Do(doc, h.jobs[doc])
		if j := h.jobs[doc]; i < feedChase && j.State != job.Goto {
			t.Fatalf("chase %d should keep walking: %+v", i, j)
		}
	}
	if j := h.jobs[doc]; j.State != job.Fail || j.Fail != job.TargetGone {
		t.Fatalf("after feedChase chases the run fails: %+v", j)
	}
}

// TestWardenFoodRunUsesTheDriver: a DeliverFood run feeds a prisoner and
// ends once its target is no prisoner.
func TestWardenFoodRunUsesTheDriver(t *testing.T) {
	c, h, m := feedScene()
	h.prisoners[patient] = true
	h.vital[patient] = health.Record{Status: health.Alive}
	if !c.Feed.Start(doc, patient, job.DeliverFood, false) || h.jobs[doc].Kind != job.DeliverFood {
		t.Fatal("the warden's run starts")
	}
	arrive(h, doc)
	c.Feed.Do(doc, h.jobs[doc])
	arrive(h, doc)
	c.Feed.Do(doc, h.jobs[doc])
	if h.jobs[doc].State != job.Complete || !slices.Contains(h.log, "feed") || !slices.Contains(m.log, "fed by") {
		t.Fatalf("the prisoner is fed: %+v %v", h.jobs[doc], h.log)
	}
	c.Feed.Start(doc, patient, job.DeliverFood, false)
	h.prisoners[patient] = false
	arrive(h, doc)
	c.Feed.Do(doc, h.jobs[doc])
	if h.jobs[doc].State != job.Complete || h.jobs[doc].Carry != 0 {
		t.Fatalf("no prisoner any more: the run ends: %+v", h.jobs[doc])
	}
}

func TestFeedOrderMenu(t *testing.T) {
	label := func(set func(c *Care, h *fakeHost, m *fakeMeals)) (Option, bool) {
		c, h, m := feedScene()
		set(c, h, m)
		return optionFor(c.PawnOrders(doc, 5, 5), command.OrderFeedPatient, patient)
	}
	if o, ok := label(func(*Care, *fakeHost, *fakeMeals) {}); !ok || o.Disabled || o.Label != "Feed P2" {
		t.Fatalf("Feed offered: %+v %v", o, ok)
	}
	if o, ok := label(func(c *Care, h *fakeHost, m *fakeMeals) { m.hunger[patient] = FeedOffer - 1 }); !ok || o.Disabled {
		t.Fatalf("below FeedOffer the order is offered (not only at Hungry): %+v", o)
	}
	if o, ok := label(func(c *Care, h *fakeHost, m *fakeMeals) { m.food = 0 }); !ok || !o.Disabled || o.Reason != ReasonNoFood {
		t.Fatalf("no food disables it: %+v", o)
	}
	if o, ok := label(func(c *Care, h *fakeHost, m *fakeMeals) { h.owners[patient] = other }); !ok || !o.Disabled || o.Reason != ReasonBusyTarget+feedScenePerson(other) {
		t.Fatalf("held by another disables it: %+v", o)
	}
	if o, ok := label(func(c *Care, h *fakeHost, m *fakeMeals) { h.blocked[doc] = true }); !ok || !o.Disabled || o.Reason != ReasonIncapable {
		t.Fatalf("a doer who can't doctor: %+v", o)
	}
	if _, ok := label(func(c *Care, h *fakeHost, m *fakeMeals) { m.hunger[patient] = FeedOffer }); ok {
		t.Fatal("a fed patient: hidden")
	}
	if _, ok := label(func(c *Care, h *fakeHost, m *fakeMeals) { h.vital[patient] = health.Record{Status: health.Alive} }); ok {
		t.Fatal("a patient that can feed itself: hidden")
	}
	c, _, _ := feedScene()
	if offer, why := c.Feed.judge(doc, patient); !offer || why != "" {
		t.Fatalf("judge %v %q", offer, why)
	}
}

func TestForceFeed(t *testing.T) {
	c, h, m := feedScene()
	m.hunger[patient] = FeedOffer - 1 // ordered above Hungry
	h.jobs[doc] = job.Record{Kind: job.Move, State: job.Goto}
	if why := c.Force(doc, command.OrderFeedPatient, patient); why != "" {
		t.Fatalf("Feed order refused: %s", why)
	}
	if j := h.jobs[doc]; j.Kind != job.FeedPatient || !j.Forced || j.Dest != patient || h.log[0] != "interrupt" || h.owners[patient] != doc {
		t.Fatalf("a forced feeding: %+v log %v", j, h.log)
	}
	c2, h2, m2 := feedScene()
	m2.food = 0
	if why := c2.Force(doc, command.OrderFeedPatient, patient); why != ReasonNoFood {
		t.Fatalf("no food: %q", why)
	}
	if len(h2.log) != 0 {
		t.Fatal("a refused order leaves the doer's job alone")
	}
	c3, h3, _ := feedScene()
	h3.blocked[doc] = true // the host refuses the start and stores no failure
	if why := c3.Feed.force(doc, patient); why != ReasonNoFood {
		t.Fatalf("a refused start: %q", why)
	}
}

func TestFeedRetrySaveRoundTrip(t *testing.T) {
	c, h, m := feedScene()
	m.food = 0
	h.now = 7
	c.Feed.assign(doc)
	saved := c.Feed.Export()
	if len(saved) != 1 || saved[0].ID != patient || saved[0].V != 7+FeedRetry {
		t.Fatalf("export %+v", saved)
	}
	c2, _, _ := feedScene()
	c2.Feed.Import(saved)
	if c2.Feed.RetryAt(patient) != 7+FeedRetry {
		t.Fatal("import restores the retry")
	}
	c2.Feed.Import(nil)
	if c2.Feed.RetryAt(patient) != 0 || c2.Feed.Export() != nil {
		t.Fatal("an older save has no retries")
	}
}

// TestFeedDoesNotFlap is the no-flap test (AGENTS.md, after sleep_test.go):
// the patient grows hungry one point a tick; it is fed only at Hungry,
// the meal fills it, and it is not fed again until it is Hungry again.
// With no food a doctor looks once per FeedRetry, not every tick.
func TestFeedDoesNotFlap(t *testing.T) {
	c, h, m := feedScene()
	m.hunger[patient] = FeedHungry + 3000
	starts, lastStart := 0, -1
	for tick := 0; tick < 20000; tick++ {
		h.now = uint64(tick)
		m.hunger[patient] = max(0, m.hunger[patient]-1)
		j := h.jobs[doc]
		switch {
		case j.Kind != job.FeedPatient || j.State == job.Complete || j.State == job.Fail:
			delete(h.owners, patient) // the host releases a finished job's hold
			h.jobs[doc] = job.Record{}
			if c.Doctor.Assign(doc) {
				if m.hunger[patient] > FeedHungry {
					t.Fatalf("tick %d: fed at %d, above Hungry", tick, m.hunger[patient])
				}
				if lastStart >= 0 && tick-lastStart < 7000 {
					t.Fatalf("tick %d: fed again %d ticks after the last meal", tick, tick-lastStart)
				}
				starts, lastStart = starts+1, tick
			}
		case j.State == job.Goto:
			arrive(h, doc)
		default:
			n := len(h.log)
			c.Feed.Do(doc, j)
			if len(h.log) > n && h.log[len(h.log)-1] == "feed" {
				m.hunger[patient] = 9900 // one meal: nearly full
			}
		}
	}
	if starts != 3 {
		t.Fatalf("%d feedings in 20000 ticks, want 3 (at Hungry only)", starts)
	}
	c2, h2, m2 := feedScene()
	m2.food = 0
	for tick := 0; tick < 5000; tick++ {
		h2.now = uint64(tick)
		c2.Doctor.Assign(doc)
	}
	if want := 5000/FeedRetry + 1; m2.looks > want {
		t.Fatalf("no food: %d looks for it in 5000 ticks, want at most %d", m2.looks, want)
	}
}

// feedScenePerson is how the scene's menu names id.
func feedScenePerson(id entity.EntityID) string {
	c, _, _ := feedScene()
	return c.personName(id)
}
