package gfx

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestCanvasSnapsAtFractionalScale (UIS-011): at 1.25 and 1.75 an unblended
// rectangle starts on a whole device pixel and equal rectangles get equal
// whole widths; at a whole scale nothing moves.
func TestCanvasSnapsAtFractionalScale(t *testing.T) {
	old := CurrentScale()
	t.Cleanup(func() { SetScale(old); SetCanvasTrace(nil) })
	var got []string
	SetCanvasTrace(func(kind string, x, y, w, h float32) {
		got = append(got, fmt.Sprintf("%s %g,%g %gx%g", kind, x, y, w, h))
	})
	img := ebiten.NewImage(64, 64)
	for _, c := range []struct {
		s    float64
		want []string
	}{
		{1.25, []string{"fill 1,1 2x2", "fill 3,1 2x2", "fill 4,1 2x2", "stroke 1,1 13x2"}},
		{1.75, []string{"fill 2,2 2x2", "fill 4,2 2x2", "fill 5,2 2x2", "stroke 2,2 18x2"}},
		{2, []string{"fill 2,2 2x2", "fill 4,2 2x2", "fill 6,2 2x2", "stroke 2,2 20x2"}},
	} {
		got = nil
		SetScale(Scale{Dev: 1, UI: c.s, S: c.s})
		for i := range 3 {
			FillRect(img, float32(1+i), 1, 1, 1, color.White, false)
		}
		StrokeRect(img, 1, 1, 10, 1, 1, color.White, false)
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("s=%v drew %q, want %q", c.s, got, c.want)
		}
	}
	got = nil
	SetScale(Scale{Dev: 1, UI: 1.25, S: 1.25})
	FillRect(img, 1, 1, 1, 1, color.White, true) // anti-aliased: unsnapped
	if want := "[fill 1.25,1.25 1.25x1.25]"; fmt.Sprint(got) != want {
		t.Errorf("anti-aliased fill at 1.25 = %q, want %q", got, want)
	}
}

// TestReferenceTypeScale (UIS-012, UIS-013): Body and Label are 14 px with a
// 16 px pitch, Small 11 with 15, Title 21 with 24, at any UI scale.
func TestReferenceTypeScale(t *testing.T) {
	old := CurrentScale()
	t.Cleanup(func() { SetScale(old) })
	for _, s := range []float64{1, 1.25, 3} {
		SetScale(Scale{Dev: 1, UI: s, S: s})
		for _, c := range []struct {
			st    TextStyle
			size  float64
			pitch float64
		}{{Small, 11, 15}, {Body, 14, 16}, {Label, 14, 16}, {Title, 21, 24}} {
			if c.st.Size != c.size || LineH(c.st) != c.pitch {
				t.Errorf("s=%v: size %v pitch %v, want %v and %v", s, c.st.Size, LineH(c.st), c.size, c.pitch)
			}
		}
		if _, h := Measure("a\nb\nc", Body); h < 2*BodyPitch || h > 2*BodyPitch+20 {
			t.Errorf("s=%v: three Body lines are %v tall, want two pitches of %v plus one line", s, h, BodyPitch)
		}
	}
}
