package ebiten

import (
	"fmt"
	"slices"
	"strconv"

	"rimworld-go/internal/application/game"
)

// Electricity and fuel alerts (PLAY-021: POWERPANE owns this file). They
// read WorldView.PowerNets and the buildings' power fields; a click on one
// centres the camera on the devices it is about.

// brownoutAlerts raises "Brownout" (a danger) for each net that sheds
// devices: it names the device drawing the most and targets the shed ones.
// With more than one grid the label says which (by order in the list).
func brownoutAlerts(w game.WorldView) []alert {
	var out []alert
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
		label := "Brownout"
		if len(w.PowerNets) > 1 {
			label = fmt.Sprintf("Brownout on grid %d", idx+1)
		}
		if biggest != "" {
			label += ": " + biggest + " draws most"
		} else {
			label += ": devices shed"
		}
		if len(shed) == 0 {
			shed = all // nothing marked as shed: the grid's consumers are the ones to look at
		}
		out = append(out, alert{key: "brownout." + strconv.Itoa(n.ID), label: label, danger: true, targets: shed})
	}
	return out
}

// notConnectedAlert raises "Not connected" (a warning) for devices (of any
// role) no grid reaches, targeting them.
func notConnectedAlert(w game.WorldView) (alert, bool) {
	var at [][2]int
	kind := ""
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if b.PowerRole == "" || b.NoPowerReason != game.NoPowerNotConnected {
			continue
		}
		at = append(at, [2]int{b.X, b.Y})
		kind = b.Kind
	}
	switch len(at) {
	case 0:
		return alert{}, false
	case 1:
		return alert{key: "notconnected", label: titleCase(kind) + " not connected", targets: at}, true
	}
	return alert{key: "notconnected", label: fmt.Sprintf("%d devices not connected", len(at)), targets: at}, true
}

// noFuelAlert raises "out of wood" for every fire that burned out: a
// campfire, a torch lamp, a wood-fired generator. The label names the one
// kind, or counts a mixed lot.
func noFuelAlert(w game.WorldView) (alert, bool) {
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
		return alert{}, false
	case len(kinds) > 1:
		return alert{key: "nofuel", label: fmt.Sprintf("Out of wood (%d)", len(at)), targets: at}, true
	case len(at) == 1:
		return alert{key: "nofuel", label: titleCase(kinds[0]) + " out of wood", targets: at}, true
	}
	return alert{key: "nofuel", label: fmt.Sprintf("%d %ss out of wood", len(at), kinds[0]), targets: at}, true
}
