package orders

import (
	"testing"

	"rimworld-go/internal/domain/sim/command"
)

type sink struct{ got []command.Command }

func (s *sink) Enqueue(c command.Command) { s.got = append(s.got, c) }

func TestMortarAimClickSendsTarget(t *testing.T) {
	var m MortarAim
	s := &sink{}
	m.Start(7, 3, 4)
	if !m.Step(s, 5, 9, 9, true, true, false) || !m.On || len(s.got) != 0 {
		t.Fatal("the opening click must be swallowed")
	}
	m.Step(s, 5, 9, 9, false, true, false) // on the HUD: stays armed
	if !m.On || len(s.got) != 0 {
		t.Fatal("a click on the chrome must not fire")
	}
	m.Step(s, 5, 9, 8, true, true, false)
	if m.On || len(s.got) != 1 {
		t.Fatalf("on=%v cmds=%v", m.On, s.got)
	}
	c := s.got[0]
	if c.Kind != command.SetMortarTarget || c.X != 3 || c.Y != 4 || c.X2 != 9 || c.Y2 != 8 || c.Pawn != 5 {
		t.Errorf("command = %+v", c)
	}
}

func TestMortarAimRightClickCancels(t *testing.T) {
	var m MortarAim
	s := &sink{}
	m.Start(7, 3, 4)
	m.Fresh = false
	m.Step(s, 5, 1, 1, true, false, true)
	if m.On || len(s.got) != 0 {
		t.Fatalf("on=%v cmds=%v", m.On, s.got)
	}
}

func TestGrenadeAimClickThrows(t *testing.T) {
	var m MortarAim
	s := &sink{}
	m.StartGrenade(9)
	m.Step(s, 0, 5, 5, true, true, false) // the opening click
	m.Step(s, 0, 5, 5, false, true, false)
	if !m.On || len(s.got) != 0 {
		t.Fatal("the opening click and a chrome click must not throw")
	}
	m.Step(s, 0, 12, 7, true, true, false)
	if m.On || len(s.got) != 1 {
		t.Fatalf("on=%v cmds=%v", m.On, s.got)
	}
	if c := s.got[0]; c.Kind != command.ThrowGrenade || c.Pawn != 9 || c.X != 12 || c.Y != 7 {
		t.Errorf("command = %+v", c)
	}
	m.StartGrenade(9)
	m.Fresh = false
	m.Step(s, 0, 1, 1, true, false, true)
	if m.On || len(s.got) != 1 {
		t.Fatal("cancel must leave without throwing")
	}
}
