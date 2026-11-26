// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wizard

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/difficulty"
)

// TestDifficultyFriendlyFireSlider (SHT-007): Custom Difficulty has a
// Friendly fire slider over 0..100 that starts at the preset's 100 and
// writes Settings.FriendlyFire; its ends are the limits.
func TestDifficultyFriendlyFireSlider(t *testing.T) {
	t.Parallel()
	sh, w := newFakeShell(t)
	w.Click(ids.MenuNewGame)
	sh.idle()
	sh.click(ids.WizardNext)
	sh.idle()
	sh.click("diff.custom")
	sh.idle()
	if sh.kit.Top() != "diff.custom" {
		t.Fatalf("Custom Difficulty is not open: %q", sh.kit.Top())
	}
	if lim, ok := difficulty.Limit("ff"); !ok || lim.Min != 0 || lim.Max != 100 {
		t.Fatalf("limit %+v %v", lim, ok)
	}
	if sp := DiffSpec("ff"); sp.Min != 0 || sp.Max != 100 {
		t.Fatalf("slider spec %+v", sp)
	}
	pressAt := func(frac float64) {
		t.Helper()
		wd, ok := sh.kit.Find("diff.f.ff")
		if !ok {
			t.Fatal("no Friendly fire slider in Custom Difficulty")
		}
		tr := uikit.SliderTrackRect(wd.R)
		raw := inputframe.Raw{Mx: int(tr.X + float32(frac)*tr.W), My: int(wd.R.Y + wd.R.H/2)}
		raw.Down[inputframe.Left], raw.Press[inputframe.Left] = true, true
		sh.frame(raw)
		raw.Press[inputframe.Left] = false
		sh.frame(raw)
		raw.Down[inputframe.Left], raw.Release[inputframe.Left] = false, true
		sh.frame(raw)
		sh.idle()
	}
	pressAt(0)
	sh.click("modal.diff.custom.accept")
	sh.idle()
	if got := w.Draft().Custom.FriendlyFire; got != 0 {
		t.Fatalf("dragged to the left end: Friendly fire %d, want 0", got)
	}
	sh.click("diff.customize")
	pressAt(1)
	sh.click("modal.diff.custom.accept")
	sh.idle()
	if got := w.Draft().Custom.FriendlyFire; got != 100 {
		t.Fatalf("dragged to the right end: Friendly fire %d, want 100", got)
	}
}
