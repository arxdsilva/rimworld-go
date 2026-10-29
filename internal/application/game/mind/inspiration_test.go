package mind

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeMuses is the fake host's Muses role: skill levels by pawn (0 when
// unset) and the inspiration news.
type fakeMuses struct {
	skill    map[entity.EntityID]uint8
	inspired []entity.EntityID
	faded    []entity.EntityID
}

func (f *fakeHost) SkillLevel(id entity.EntityID, _ skill.ID) uint8 { return f.skill[id] }
func (f *fakeHost) Inspired(id entity.EntityID, _ string)           { f.inspired = append(f.inspired, id) }
func (f *fakeHost) InspirationEnded(id entity.EntityID, _ string) {
	f.faded = append(f.faded, id)
}

// creativeFake is a fake with colonist 1 at Crafting CreativityMinSkill
// and every roll 0 (an inspiration whenever one may start).
func creativeFake() (*fakeHost, *Mind) {
	f := newFake(1)
	f.skill = map[entity.EntityID]uint8{1: CreativityMinSkill}
	return f, New(f)
}

func TestMTBHours(t *testing.T) {
	t.Parallel()
	cases := map[int]int{49: 0, 0: 0, 50: 210 * 24, 70: 130 * 24, 100: 10 * 24, 120: 10 * 24}
	for mood, want := range cases {
		if got := MTBHours(mood); got != want {
			t.Errorf("MTBHours(%d) = %d, want %d", mood, got, want)
		}
	}
}

func TestInspirationStartsUsesAndCalms(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	f.mood[1] = 49
	m.Inspire.Check(1)
	if _, ok := m.Inspire.Of(1); ok {
		t.Fatal("inspired below the minimum mood")
	}
	f.mood[1] = 90
	m.Inspire.Check(1)
	in, ok := m.Inspire.Of(1)
	if !ok || in.Kind != Creativity || in.Until != InspireDuration || len(f.inspired) != 1 {
		t.Fatalf("inspiration %+v %v, news %v", in, ok, f.inspired)
	}
	if in.Kind.String() != "inspired creativity" || NoInspiration.String() != "" {
		t.Fatal("labels")
	}
	if !m.Inspire.Use(1, Creativity) || m.Inspire.Use(1, Creativity) {
		t.Fatal("an inspiration is spent exactly once")
	}
	f.now = InspireCalm - 1
	m.Inspire.Check(1)
	if _, ok := m.Inspire.Of(1); ok {
		t.Fatal("inspired again during the calm")
	}
	f.now = InspireCalm
	m.Inspire.Check(1)
	if _, ok := m.Inspire.Of(1); !ok {
		t.Fatal("not inspired after the calm")
	}
}

func TestInspirationNeedsSkill(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	f.skill[1] = CreativityMinSkill - 1
	f.mood[1] = 100
	for h := range 100 {
		f.now = uint64(h) * clock.TicksPerHour
		m.Inspire.Check(1)
	}
	if _, ok := m.Inspire.Of(1); ok || len(f.inspired) != 0 {
		t.Fatal("an unskilled colonist got inspired creativity")
	}
}

func TestInspirationFades(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	m.Inspire.Start(1, Creativity)
	f.now = InspireDuration
	if _, ok := m.Inspire.Of(1); ok {
		t.Fatal("a faded inspiration still counts")
	}
	if m.Inspire.Use(1, Creativity) {
		t.Fatal("a faded inspiration was spent")
	}
	m.Inspire.Check(1)
	if len(f.faded) != 1 || m.Inspire.calm[1] != InspireDuration+InspireCalm {
		t.Fatalf("faded %v, calm %d", f.faded, m.Inspire.calm[1])
	}
}

// TestInspirationNoFlap: a mood swinging across the threshold every hour
// starts an inspiration once and never ends it early; after it fades, the
// calm keeps a new one off. Over 30 days of always-winning rolls there
// are at most one start per duration-plus-calm.
func TestInspirationNoFlap(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	starts, ends, was := 0, 0, false
	const hours = 30 * clock.HoursPerDay
	for h := range hours {
		f.now = uint64(h) * clock.TicksPerHour
		f.mood[1] = InspireMinMood - 1 + 2*(h%2) // 49, 51, 49, ...
		m.Inspire.Check(1)
		_, on := m.Inspire.Of(1)
		if on && !was {
			starts++
		}
		if !on && was {
			ends++
		}
		was = on
	}
	period := (InspireDuration + InspireCalm) / clock.TicksPerHour
	if max := hours/period + 1; starts == 0 || starts > max || ends > starts {
		t.Fatalf("%d starts, %d ends in %d hours (at most %d)", starts, ends, hours, max)
	}
}

// TestInspirationCheckFromTickAndSave: Mind.Tick's hourly check inspires a
// calm colonist, and the save keeps the inspiration and the calm.
func TestInspirationCheckFromTickAndSave(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	f.mood[1] = 100
	for h := range 2 * clock.TicksPerHour {
		f.now = uint64(h)
		m.Tick(1, job.Record{})
	}
	if _, ok := m.Inspire.Of(1); !ok {
		t.Fatal("the hourly check never ran")
	}
	m.Inspire.calm[2] = 99
	s := m.Export()
	n := New(f)
	n.Import(s)
	if _, ok := n.Inspire.Of(1); !ok || n.Inspire.calm[2] != 99 {
		t.Fatalf("loaded %+v", s)
	}
	n.Import(Save{}) // an older save: none
	if _, ok := n.Inspire.Of(1); ok {
		t.Fatal("an older save kept an inspiration")
	}
}
