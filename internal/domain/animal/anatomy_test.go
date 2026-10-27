package animal

import (
	"testing"

	"rimworld-go/internal/domain/pawn/body"
)

// MED-13: each archetype has its body plan; no animal is humanlike.
func TestArchetypeAnatomy(t *testing.T) {
	t.Parallel()
	want := map[Species]body.Anatomy{
		TimberWolf: body.Quadruped, Monkey: body.Quadruped, HiveBrute: body.Quadruped,
		Chicken: body.Bird, Cobra: body.Snake, Iguana: body.Lizard, Tortoise: body.Lizard,
		HiveBeetle: body.Insect, HiveCrawler: body.Insect,
	}
	for s, a := range want {
		if got := AnatomyOf(s); got != a {
			t.Fatalf("%v: %v, want %v", s, got, a)
		}
	}
	for _, s := range All() {
		if AnatomyOf(s) == body.Humanlike {
			t.Fatalf("%v has the humanlike body", s)
		}
	}
}

// legsGone is a whole quadruped body (a man's scale) with legs ls (and
// their paws) lost.
func legsGone(ls ...body.Part) *body.Body {
	paw := map[body.Part]body.Part{body.QLegFL: body.QPawFL, body.QLegFR: body.QPawFR, body.QLegHL: body.QPawHL, body.QLegHR: body.QPawHR}
	b := &body.Body{Anatomy: body.Quadruped, Scale: 1000}
	for _, l := range ls {
		b.Missing |= 1<<l | 1<<paw[l]
	}
	return b
}

// MED-13: a quadruped that lost a leg moves at three quarters: its
// capacity reads 75 % and each step takes a third longer.
func TestQuadrupedLosingLegMovesSlower(t *testing.T) {
	t.Parallel()
	whole := State{Species: TimberWolf, Body: legsGone()}
	lame := State{Species: TimberWolf, Body: legsGone(body.QLegFL)}
	if whole.Moving() != 1000 || lame.Moving() != 750 {
		t.Fatalf("moving: whole %d, on three legs %d", whole.Moving(), lame.Moving())
	}
	st := DefOf(TimberWolf).StepTicks
	if whole.Step() != st || lame.Step() != (st*4+2)/3 {
		t.Fatalf("step: whole %d (want %d), lame %d (want %d)", whole.Step(), st, lame.Step(), (st*4+2)/3)
	}
	lame.Mode = Flee
	if f := DefOf(TimberWolf).FleeStep; lame.Step() <= f {
		t.Fatalf("a lame wolf flees slower too: %d vs %d", lame.Step(), f)
	}
	if lame.DownsNow() {
		t.Fatal("three legs still carry it")
	}
	gone := State{Species: TimberWolf, Body: legsGone(body.QLegFL, body.QLegFR, body.QLegHL, body.QLegHR)}
	if gone.Moving() != 0 || !gone.DownsNow() {
		t.Fatalf("no legs: moving %d, down %v", gone.Moving(), gone.DownsNow())
	}
	if gone.Step() != (st*1000+DownMoving-1)/DownMoving {
		t.Fatalf("the step slows no further than DownMoving: %d", gone.Step())
	}
}

// MED-13: moving downs an animal below DownMoving and stands it only at
// UpMoving, so a crippled animal hovering at the down line does not flap
// between lying and standing.
func TestAnimalMovingDownNoFlap(t *testing.T) {
	t.Parallel()
	s := State{Species: TimberWolf, Body: legsGone(body.QLegFL, body.QLegFR, body.QLegHL)}
	// the last leg's wound swings moving between 141 and 250 (three legs
	// gone: a quarter of the last one)
	hurt := []body.Wound{{Part: body.QLegHR, Dmg: 13, Kind: body.Bruise}}
	changes := 0
	for i := 0; i < 40; i++ {
		s.Body.Wounds = nil
		if i%2 == 0 {
			s.Body.Wounds = hurt
		}
		m := s.Moving()
		if (i%2 == 0) != (m < DownMoving) || m >= UpMoving {
			t.Fatalf("step %d: moving %d", i, m)
		}
		switch {
		case s.DownsNow():
			s.Downed = true
			changes++
		case s.StandsUp():
			s.Downed = false
			changes++
		}
	}
	if changes != 1 || !s.Downed {
		t.Fatalf("%d changes (downed %v), want one: down and staying down", changes, s.Downed)
	}
	// standing between the lines, it stays up
	up := State{Species: TimberWolf, Body: legsGone(body.QLegFL, body.QLegFR, body.QLegHL)}
	if m := up.Moving(); m < DownMoving || m >= UpMoving || up.DownsNow() {
		t.Fatalf("between the lines (%d) a standing animal stays up", m)
	}
}
