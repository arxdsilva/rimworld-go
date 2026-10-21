//go:build audiodevice

package ebitenaudio

import (
	"testing"
	"time"
)

// TestDeviceSmoke plays a click and a second of the campfire loop on the
// real device: go test -tags audiodevice ./internal/adapters/outbound/ebitenaudio/
func TestDeviceSmoke(t *testing.T) {
	root, err := LocateAudio()
	if err != nil {
		t.Skip(err)
	}
	e, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	e.focused = func() bool { return true }
	e.Play("ui.click", 1, -0.5)
	for range tps {
		e.Loop("fire", "amb.campfire", 1, 0.5)
		e.Update()
		time.Sleep(time.Second / tps)
	}
}
