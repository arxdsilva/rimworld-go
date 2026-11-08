package light

import (
	"sync"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Every Lit shares the one warmed lit.kage, and first uses on several
// goroutines at once (parallel tests) do not race on it (GAR E9).
func TestSharedLitIsOneWarmShader(t *testing.T) {
	t.Parallel()
	want, err := SharedLit()
	if err != nil || want == nil {
		t.Fatalf("lit.kage: %v", err)
	}
	var wg sync.WaitGroup
	got := make([]*ebiten.Shader, 4)
	for i := range got {
		wg.Go(func() {
			var l Lit
			got[i] = l.LitShader()
		})
	}
	wg.Wait()
	for i, sh := range got {
		if sh != want {
			t.Fatalf("Lit %d compiled its own shader", i)
		}
	}
}
