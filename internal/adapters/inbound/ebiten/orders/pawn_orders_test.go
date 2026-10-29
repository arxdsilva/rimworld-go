package orders

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// pawnOpts is the contract's option order for a downed pawn 7 (Bo).
func pawnOpts() []game.OrderOption {
	mk := func(label string, o command.Order) game.OrderOption {
		return game.OrderOption{Label: label, Cmd: command.Command{Kind: command.Prioritize, Order: o, Pawn: 1, Target: 7, X: 4, Y: 5}}
	}
	rescue := mk("Rescue Bo", command.OrderRescue)
	rescue.Disabled, rescue.Reason = true, "no free bed"
	return []game.OrderOption{rescue, mk("Capture Bo", command.OrderCapture), mk("Arrest Bo", command.OrderArrest), mk("Prioritize tending Bo", command.OrderTendPatient)}
}

func TestUndraftedMenuListsPawnOrdersInOrderWithDisabledReason(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.selected, f.pawns = 1, []game.PawnView{{ID: 1}}
	f.opts = pawnOpts()
	c := New(f)
	c.RightClickTile(4, 5, 10, 10)
	if !c.Float.Open || len(f.cmds) != 0 {
		t.Fatalf("menu should open: %+v", f.cmds)
	}
	btns := c.FloatButtons()
	want := []string{"Rescue Bo  - no free bed", "Capture Bo", "Arrest Bo", "Prioritize tending Bo"}
	for i, w := range want {
		if btns[i].Label != w {
			t.Errorf("row %d: %q want %q", i, btns[i].Label, w)
		}
	}
	if !btns[0].Disabled || btns[1].Disabled {
		t.Fatalf("disabled flags wrong: %+v", btns)
	}
	c.ChooseFloat("float.0")
	if len(f.cmds) != 0 || f.hint != "no free bed" || !c.Float.Open {
		t.Fatalf("a disabled row only hints: cmds=%+v hint=%q open=%v", f.cmds, f.hint, c.Float.Open)
	}
}

func TestDraftedRightClickOnDownedPawnOpensMenuNoFormation(t *testing.T) {
	t.Parallel()
	f := newFake()
	f.selected, f.pawns = 1, []game.PawnView{{ID: 1, Drafted: true}}
	f.tx, f.ty = 4, 5
	f.opts = []game.OrderOption{{Label: "Tend Bo", Cmd: command.Command{Kind: command.EmergencyTend, Pawn: 1, Target: 7}}}
	c := New(f)
	f.in.Raw.Press[inputframe.Right], f.in.Raw.Down[inputframe.Right] = true, true
	c.HandleOrders()
	if c.Form.On || !c.Float.Open || len(f.cmds) != 0 {
		t.Fatalf("menu, no formation, no move: form=%+v open=%v cmds=%+v", c.Form, c.Float.Open, f.cmds)
	}
}

func TestChooseFloatPawnOrderEnqueuesCmdAndShiftQueues(t *testing.T) {
	t.Parallel()
	for _, shift := range []bool{false, true} {
		f := newFake()
		if shift {
			f.in = inputframe.NewFrame(inputframe.Raw{Mods: input.ModShift})
		}
		f.opts = pawnOpts()
		c := New(f)
		c.Float = Float{Open: true, Opts: f.opts}
		c.ChooseFloat("float.2") // Arrest Bo
		want := f.opts[2].Cmd
		want.Queue = shift
		if len(f.cmds) != 1 || f.cmds[0] != want || c.Float.Open {
			t.Fatalf("shift=%v: cmds=%+v want %+v", shift, f.cmds, want)
		}
	}
}
