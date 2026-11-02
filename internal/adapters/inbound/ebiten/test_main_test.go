package ebiten

import (
	"image/color"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	_ "unsafe" // go:linkname, below

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"

	"github.com/hajimehoshi/ebiten/v2"
)

// Ebitengine's atlas queues a draw it gets outside a frame, vertices and
// pixels copied, until the next frame. No test runs a game loop, so nothing
// ever reads the queue and every draw of the whole binary stays in memory
// (with -race, 15 GB of RSS on CI's 16 GB runner). There is no public way to
// empty it, so the tests reach the two variables. TestAtlasQueueDrains fails
// if an Ebitengine upgrade renames them.

//go:linkname atlasDeferred github.com/hajimehoshi/ebiten/v2/internal/atlas.deferred
var atlasDeferred []func()

//go:linkname atlasDeferredM github.com/hajimehoshi/ebiten/v2/internal/atlas.deferredM
var atlasDeferredM sync.Mutex

// drainAtlasQueue drops the queued draws and reports how many there were.
func drainAtlasQueue() int {
	atlasDeferredM.Lock()
	defer atlasDeferredM.Unlock()
	n := len(atlasDeferred)
	atlasDeferred = nil
	return n
}

// appsBuilt counts testApp calls, for forgetFontsNow.
var appsBuilt atomic.Int64

// forgetFontsNow lets the shaped-text cache go every so many apps. By count,
// not by the clock: an allocation test's loop must not meet an empty cache.
func forgetFontsNow() {
	if appsBuilt.Add(1)%256 == 0 {
		gfx.ForgetFonts()
	}
}

func TestMain(m *testing.M) {
	go func() {
		for range time.Tick(time.Second) {
			drainAtlasQueue()
		}
	}()
	os.Exit(m.Run())
}

func TestAtlasQueueDrains(t *testing.T) {
	t.Parallel()
	for try := 0; try < 3; try++ { // the ticker can take a draw before we look
		ebiten.NewImage(4, 4).Fill(color.White)
		if drainAtlasQueue() > 0 {
			return
		}
	}
	t.Fatal("a draw did not reach the queue: did Ebitengine rename atlas.deferred?")
}
