package ebiten

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// PLAY-018: the messages sit in the layout's Messages rect, right-aligned
// against the right column under the colonist bar, up to five rows, and
// fade out instead of vanishing.

func messageStub(n int) *stubSim {
	s := &stubSim{w: 80, h: 80, tick: 100, pawns: []game.PawnView{{ID: 1, Name: "Ada", Status: "alive", X: 20, Y: 20}}}
	for i := 1; i <= n; i++ {
		m := game.MessageView{Tick: uint64(i), Text: fmt.Sprintf("Message number %d about something that happened", i), X: 40 + i, Y: 40}
		if i%2 == 0 {
			m.Danger = true
		}
		s.world.Messages = append(s.world.Messages, m)
	}
	return s
}

func TestMessagesFillTheLayoutsRectRightAligned(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, messageStub(7))
	l := a.hud()
	if got := a.chrome.Right().MessageCount(); got != hudlayout.MaxMsgs {
		t.Fatalf("messageCount() = %d, want the %d newest", got, hudlayout.MaxMsgs)
	}
	rows := a.messageRows(l)
	if len(rows) != hudlayout.MaxMsgs {
		t.Fatalf("rows = %d, want %d", len(rows), hudlayout.MaxMsgs)
	}
	for i, r := range rows {
		if r.r.H != hudlayout.MsgRowH || r.r.Y != l.Messages.Y+float32(i)*hudlayout.MsgStride {
			t.Errorf("row %d at %+v, Messages %+v", i, r.r, l.Messages)
		}
		if r.r.Right() != l.Messages.Right() || r.r.X < l.Messages.X {
			t.Errorf("row %d %+v is not right-aligned inside %+v", i, r.r, l.Messages)
		}
	}
	if got := rows[hudlayout.MaxMsgs-1].msg.Text; !strings.Contains(got, "number 7") {
		t.Errorf("the newest message is last, got %q", got)
	}
	if l.Messages.Right() >= l.Right.X {
		t.Errorf("messages %+v reach the right column %+v", l.Messages, l.Right)
	}
	// each row with a place is a click-to-focus button, ids msg.<i>
	ids := map[string]uikit.Button{}
	for _, b := range a.chrome.Right().Buttons(a.sw) {
		ids[b.ID] = b
	}
	for i, r := range rows {
		b, ok := ids[fmt.Sprintf("msg.%d", i)]
		if !ok || b.X != r.r.X || b.Y != r.r.Y || b.W != r.r.W || b.H != r.r.H {
			t.Errorf("msg.%d button %+v (found %v), want %+v", i, b, ok, r.r)
		}
	}
	before := a.renderer.cam
	a.applyClick("msg.4")
	if a.renderer.cam == before {
		t.Error("clicking a message does not jump to its place")
	}
}

// Everything the messages draw lies inside the Messages rect, at every UI scale.
func TestMessagesDrawInsideTheirRectAtScale(t *testing.T) {
	for _, s := range []float64{1, 1.5, 2} {
		t.Run(fmt.Sprintf("x%v", s), func(t *testing.T) {
			sim := messageStub(5)
			sim.world.Messages[1].Text = strings.Repeat("A very long notice that has to be shortened to fit. ", 8)
			a, f := colonyApp(t, sim)
			pw, ph := rightAtScale(t, a, f, s)
			l := a.hud()
			if l.Messages.Empty() {
				t.Fatal("no Messages rect")
			}
			var evs []gfx.TraceEvent
			gfx.SetDrawTrace(func(e gfx.TraceEvent) { evs = append(evs, e) })
			t.Cleanup(func() { gfx.SetDrawTrace(nil) })
			a.chrome.Right().DrawMessagesIn(ebiten.NewImage(pw, ph), l)
			gfx.SetDrawTrace(nil)
			if len(evs) == 0 {
				t.Fatal("nothing drawn")
			}
			texts := 0
			for _, e := range evs {
				x, y, w, h := e.X/float32(s), e.Y/float32(s), e.W/float32(s), e.H/float32(s)
				const eps = 0.6
				if x < l.Messages.X-eps || y < l.Messages.Y-eps || x+w > l.Messages.Right()+eps || y+h > l.Messages.Bottom()+eps {
					t.Errorf("%s %q at %g,%g %gx%g (logical) leaves Messages %+v", e.Kind, e.Text, x, y, w, h, l.Messages)
				}
				if e.Kind == "text" {
					texts++
				}
			}
			if texts != 5 {
				t.Errorf("%d text runs, want 5", texts)
			}
		})
	}
}

// A message fades over its last frames instead of vanishing at once.
func TestMessageFadesOut(t *testing.T) {
	t.Parallel()
	if hud.MessageAlpha(0) != 1 || hud.MessageAlpha(hud.MessageFrames-hud.MessageFadeFrames) != 1 {
		t.Fatalf("alpha early: %v / %v", hud.MessageAlpha(0), hud.MessageAlpha(hud.MessageFrames-hud.MessageFadeFrames))
	}
	mid := hud.MessageAlpha(hud.MessageFrames - hud.MessageFadeFrames/2)
	if mid <= 0.3 || mid >= 0.7 {
		t.Fatalf("alpha half way through the fade = %v", mid)
	}
	if hud.MessageAlpha(hud.MessageFrames-1) <= 0 || hud.MessageAlpha(hud.MessageFrames-1) >= 0.2 {
		t.Fatalf("alpha on the last frame = %v", hud.MessageAlpha(hud.MessageFrames-1))
	}

	a, _ := colonyApp(t, messageStub(1))
	if a.chrome.Right().MessageCount() != 1 {
		t.Fatal("the message is not live")
	}
	rows := a.messageRows(a.hud())
	if len(rows) != 1 || rows[0].alpha != 1 {
		t.Fatalf("fresh row %+v", rows)
	}
	a.renderer.present += hud.MessageFrames - hud.MessageFadeFrames/2
	rows = a.messageRows(a.hud())
	if len(rows) != 1 || rows[0].alpha >= 0.7 || rows[0].alpha <= 0.3 {
		t.Fatalf("half-faded row %+v", rows)
	}
	a.chrome.Right().Draw(ebiten.NewImage(a.sw, a.sh), a.sw, a.sh)
	a.renderer.present += hud.MessageFadeFrames
	if a.chrome.Right().MessageCount() != 0 || len(a.messageRows(a.hud())) != 0 {
		t.Fatal("a faded message is gone")
	}
}

// Danger messages keep their red text, now with a kind-coloured edge.
func TestMessageEdgeFollowsItsKind(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		m    game.MessageView
		want any
	}{
		{game.MessageView{Danger: true}, uikit.LetterThreat},
		{game.MessageView{Kind: "good"}, uikit.LetterGood},
		{game.MessageView{Kind: "caution"}, uikit.LetterCaution},
		{game.MessageView{}, nil},
	} {
		got := hud.MessageEdge(c.m)
		if c.want == nil {
			if got.A != 0 {
				t.Errorf("%+v: edge %v, want none", c.m, got)
			}
			continue
		}
		if got != c.want {
			t.Errorf("%+v: edge %v, want %v", c.m, got, c.want)
		}
	}
}
