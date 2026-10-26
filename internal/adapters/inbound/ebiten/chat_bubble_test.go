package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// PLAY-020: a chat bubble is just its round icon, with nothing behind it
// (it used to sit in a white box), and a plain chat shows the chit-chat
// icon too. Without art it is a small dark disc with dots, never a box.

func TestChatBubbleIsJustItsIcon(t *testing.T) {
	sc := wlScales[0]
	wlSetScale(t, sc)
	got := wlTrace(t)
	social := game.PawnView{ID: 7, X: 6, Y: 5, Interaction: "chitchat", InteractTick: 90}
	chat := game.PawnView{ID: 8, X: 12, Y: 5, Job: "chat do"} // even id: its turn at frame 0
	r := wlRenderer(sc, 1, 0, 0, social, chat)
	r.drawChatBubbles(ebiten.NewImage(64, 64), r.tilePx())

	imgs := wlOfKind(*got, "image")
	if len(imgs) != 2 {
		t.Fatalf("%d images drawn, want the two bubble icons: %+v", len(imgs), *got)
	}
	for _, e := range imgs {
		if e.Via != "worldIcon" {
			t.Errorf("bubble icon drawn via %q, want worldIcon", e.Via)
		}
	}
	for _, k := range []string{"fill", "stroke", "circle"} {
		if n := len(wlOfKind(*got, k)); n != 0 {
			t.Errorf("%d %s draws behind the icons, want none (no box, no tail)", n, k)
		}
	}
}

func TestChatBubbleFallbackIsADiscNotABox(t *testing.T) {
	sc := wlScales[0]
	wlSetScale(t, sc)
	got := wlTrace(t)
	r := wlRenderer(sc, 1, 0, 0, game.PawnView{ID: 8, X: 6, Y: 5, Job: "chat do"})
	r.art = figureArt() // no social icons
	r.drawChatBubbles(ebiten.NewImage(64, 64), r.tilePx())

	if n := len(wlOfKind(*got, "fill")) + len(wlOfKind(*got, "stroke")); n != 0 {
		t.Fatalf("%d box draws, want none", n)
	}
	if n := len(wlOfKind(*got, "circle")); n != 4 {
		t.Fatalf("%d circles, want the disc and its three dots", n)
	}
}
