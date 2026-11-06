package hud

import (
	"slices"
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
)

// Electricity and fuel alerts (PLAY-021: POWERPANE owns this file). They
// read WorldView.PowerNets and the buildings' power fields; a click on one
// centres the camera on the devices it is about.

// The power roles a building view carries (game.BuildingView.PowerRole).
const (
	powerProducer = "producer"
	powerConsumer = "consumer"
)

// brownoutAlerts raises "Brownout" (a danger) for each net that sheds
// devices: it names the device drawing the most and targets the shed ones.
// With more than one grid the label says which (by order in the list).
func brownoutAlerts(tr lang.Tr, w game.WorldView) []Alert {
	var out []Alert
	for idx, n := range w.PowerNets {
		if !n.Brownout {
			continue
		}
		var shed, all [][2]int
		biggest, most := "", 0
		for i := range w.Buildings {
			b := &w.Buildings[i]
			if b.PowerRole != powerConsumer || b.PowerNet != n.ID {
				continue
			}
			all = append(all, [2]int{b.X, b.Y})
			switch {
			case b.Powered && b.PowerUse > most:
				biggest, most = b.Kind, b.PowerUse
			case !b.Powered && b.NoPowerReason == game.NoPowerBrownout:
				shed = append(shed, [2]int{b.X, b.Y})
			}
		}
		label := tr.T("Brownout")
		if len(w.PowerNets) > 1 {
			label = lang.Tf(tr, "Brownout on grid %d", idx+1)
		}
		if biggest != "" {
			label += lang.Tf(tr, ": %s draws most", biggest)
		} else {
			label += tr.T(": devices shed")
		}
		if len(shed) == 0 {
			shed = all // nothing marked as shed: the grid's consumers are the ones to look at
		}
		out = append(out, Alert{Key: "brownout." + strconv.Itoa(n.ID), Label: label, Danger: true, Targets: shed})
	}
	return out
}

// notConnectedAlert raises "Not connected" (a warning) for devices (of any
// role) no grid reaches, targeting them.
func notConnectedAlert(tr lang.Tr, w game.WorldView) (Alert, bool) {
	var at [][2]int
	kind := ""
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if b.Hostile || b.PowerRole == "" || b.NoPowerReason != game.NoPowerNotConnected { // MAP-13: a settlement's turret is not ours to wire
			continue
		}
		at = append(at, [2]int{b.X, b.Y})
		kind = b.Kind
	}
	switch len(at) {
	case 0:
		return Alert{}, false
	case 1:
		return Alert{Key: "notconnected", Label: lang.Tf(tr, "%s not connected", titleCase(kind)), Targets: at}, true
	}
	return Alert{Key: "notconnected", Label: lang.Tf(tr, "%d devices not connected", len(at)), Targets: at}, true
}

// noFuelAlert raises "out of wood" for every fire that burned out: a
// campfire, a torch lamp, a wood-fired generator. The label names the one
// kind, or counts a mixed lot.
func noFuelAlert(tr lang.Tr, w game.WorldView) (Alert, bool) {
	var at [][2]int
	var kinds []string
	for i := range w.Buildings {
		b := &w.Buildings[i]
		out := b.Fuel && b.FuelPct == 0 ||
			b.PowerRole == powerProducer && b.NoPowerReason == game.NoPowerNoFuel // a generator's view may lack the Fuel flag
		if !out {
			continue
		}
		at = append(at, [2]int{b.X, b.Y})
		if !slices.Contains(kinds, b.Kind) {
			kinds = append(kinds, b.Kind)
		}
	}
	switch {
	case len(at) == 0:
		return Alert{}, false
	case len(kinds) > 1:
		return Alert{Key: "nofuel", Label: lang.Tf(tr, "Out of wood (%d)", len(at)), Targets: at}, true
	case len(at) == 1:
		return Alert{Key: "nofuel", Label: lang.Tf(tr, "%s out of wood", titleCase(kinds[0])), Targets: at}, true
	}
	return Alert{Key: "nofuel", Label: lang.Tf(tr, "%d %ss out of wood", len(at), kinds[0]), Targets: at}, true
}
