// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package ebiten

import (
	"image/color"
	"sync"
	"sync/atomic"
	"testing"
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
//
// The queue is drained only by a test's own goroutine, never in the
// background: every testApp drains it first (forgetFontsNow), and a test
// that counts the queue or measures allocations drains it where it needs
// to. A periodic drainer used to race those tests (TestAtlasQueueDrains
// retried three times for it).

//go:linkname atlasDeferred github.com/hajimehoshi/ebiten/v2/internal/atlas.deferred
var atlasDeferred []func()

//go:linkname atlasDeferredM github.com/hajimehoshi/ebiten/v2/internal/atlas.deferredM
var atlasDeferredM sync.Mutex

// drainAtlasQueue drops the queued draws and reports how many there were.
// It keeps the queue's array, cleared so the dropped draws and the images
// they hold can go: Ebitengine's next append after a drain does not regrow
// it, so a drain never shows up in an allocation test's count.
func drainAtlasQueue() int {
	atlasDeferredM.Lock()
	defer atlasDeferredM.Unlock()
	n := len(atlasDeferred)
	clear(atlasDeferred)
	atlasDeferred = atlasDeferred[:0]
	return n
}

// atlasQueueCap is the queue array's capacity: a window whose draws stay
// under it appends without allocating the array again.
func atlasQueueCap() int {
	atlasDeferredM.Lock()
	defer atlasDeferredM.Unlock()
	return cap(atlasDeferred)
}

// appsBuilt counts testApp calls, for forgetFontsNow.
var appsBuilt atomic.Int64

// forgetFontsNow drops the atlas queue (each app a test builds frees what
// the tests before it queued), and lets the shaped-text cache go every so
// many apps. By count, not by the clock: an allocation test's loop must not
// meet an empty cache.
func forgetFontsNow() {
	drainAtlasQueue()
	if appsBuilt.Add(1)%256 == 0 {
		gfx.ForgetFonts()
	}
}

// TestAtlasQueueDrains is serial on purpose: every drain runs on a test's
// goroutine, and a top-level parallel test never runs beside a serial one,
// so nothing else can empty the queue between the draw and the count.
func TestAtlasQueueDrains(t *testing.T) {
	drainAtlasQueue()
	ebiten.NewImage(4, 4).Fill(color.White)
	c := atlasQueueCap()
	if drainAtlasQueue() == 0 {
		t.Fatal("a draw did not reach the queue: did Ebitengine rename atlas.deferred?")
	}
	if got := atlasQueueCap(); got < c || c == 0 {
		t.Fatalf("the drain dropped the queue's array: cap %d, was %d", got, c)
	}
}
