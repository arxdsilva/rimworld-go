package ebiten

import (
	"fmt"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/input"
)

// Electricity on the building inspector (PLAY-021: POWERPANE owns this
// file). A device's pane says what it draws or makes, why it has no power,
// and what its grid is doing; its command buttons are the Power toggle
// (SetPowerWanted: a colonist flicks it later, "flick pending" until then)
// and, for a heater or cooler, the thermostat (SetThermostat). Everything
// is read from BuildingView and WorldView.PowerNets; nothing here knows a
// device's watts or the sim's rules.

// BuildingView.PowerRole values (power.Role's names), the thermostat's
// step and the command ids. The building kinds (heaterKind, coolerKind,
// torchLampKind) are power_draw.go's.
const (
	powerProducer    = "producer"
	powerConsumer    = "consumer"
	powerStorage     = "storage"
	powerSwitch      = "switch"
	powerThermoStep  = 10 // a thermostat step: 1 °C, in tenths
	powerToggleCmdID = "cmd.power.toggle"
	powerDownCmdID   = "cmd.power.temp_down"
	powerUpCmdID     = "cmd.power.temp_up"
)

// hasPowerToggle reports b has the player's on / off: a switch, a generator
// or a consumer. A battery and a conduit have none.
func hasPowerToggle(b game.BuildingView) bool {
	switch b.PowerRole {
	case powerProducer, powerConsumer, powerSwitch:
		return true
	}
	return false
}

// isThermostatBldg reports b is a heater or a cooler.
func isThermostatBldg(b game.BuildingView) bool {
	return b.Kind == heaterKind || b.Kind == coolerKind
}

// powerNetOf is the world's net with id (ok false: not listed, or id 0).
func powerNetOf(w game.WorldView, id int) (game.PowerNetView, bool) {
	if id == 0 {
		return game.PowerNetView{}, false
	}
	for _, n := range w.PowerNets {
		if n.ID == id {
			return n, true
		}
	}
	return game.PowerNetView{}, false
}

// fmtTarget is a thermostat target in tenths of a degree C: "21 °C", or
// "21.5 °C" when it is not whole.
func fmtTarget(tenths int) string {
	if tenths%10 == 0 {
		return fmt.Sprintf("%d °C", tenths/10)
	}
	sign, t := "", tenths
	if t < 0 {
		sign, t = "-", -t
	}
	return fmt.Sprintf("%s%d.%d °C", sign, t/10, t%10)
}

// Warning lines start with one of these; the pane draws them in the
// warning colour (paneLineWarns).
var powerWarnPrefixes = []string{"No power", "Not producing", "Not connected", "Brownout"}

// paneLineWarns reports a pane line is a power warning.
func paneLineWarns(line string) bool {
	for _, p := range powerWarnPrefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

// powerLines are b's pane lines for electricity: its own draw or output,
// what is wrong, a pending flick, and its grid's balance. Nil for a
// building that is not electrical.
func powerLines(b game.BuildingView, w game.WorldView) []string {
	if b.PowerRole == "" {
		return nil
	}
	var out []string
	switch b.PowerRole {
	case powerConsumer:
		if b.Powered {
			out = append(out, fmt.Sprintf("Uses %d W", b.PowerUse))
		} else if b.NoPowerReason != "" {
			out = append(out, "No power: "+b.NoPowerReason)
		} else {
			out = append(out, "No power")
		}
		if isThermostatBldg(b) {
			out = append(out, "Target "+fmtTarget(b.ThermoTarget))
			switch {
			case !b.ThermoActive:
				out = append(out, "Idle (at target)")
			case b.Kind == coolerKind:
				out = append(out, "Cooling now")
			default:
				out = append(out, "Heating now")
			}
		}
	case powerProducer:
		switch {
		case b.PowerOut > 0:
			out = append(out, fmt.Sprintf("Produces %d W", b.PowerOut))
			if b.NoPowerReason == game.NoPowerNotConnected {
				out = append(out, "Not connected: its power goes nowhere")
			}
		case b.NoPowerReason != "":
			out = append(out, "Not producing: "+b.NoPowerReason)
		default:
			out = append(out, "Produces 0 W") // idle: a panel at night, a turbine in a calm
		}
		if b.PowerMax > 0 && b.PowerOut < b.PowerMax && (b.Kind == solarPanelKind || b.Kind == windTurbineKind) {
			out = append(out, fmt.Sprintf("Up to %d W", b.PowerMax)) // sun and wind vary
		}
	case powerStorage:
		if b.CapWh > 0 {
			out = append(out, fmt.Sprintf("Holds %d of %d Wh", b.StoredWh, b.CapWh))
		}
		if b.NoPowerReason == game.NoPowerNotConnected {
			out = append(out, "Not connected: it can't charge or give power")
		}
	case powerSwitch:
		if b.PowerOn {
			out = append(out, "Closed (carries power)")
		} else {
			out = append(out, "Open (cuts the grid)")
		}
	}
	if hasPowerToggle(b) && b.PowerWantOn != b.PowerOn {
		way := "on"
		if !b.PowerWantOn {
			way = "off"
		}
		out = append(out, "Flick pending: a colonist will switch it "+way)
	}
	if n, ok := powerNetOf(w, b.PowerNet); ok {
		out = append(out, gridLine(n))
		if n.Brownout {
			out = append(out, "Brownout: some devices are shed")
		}
	}
	return out
}

// gridLine is a net's balance: "Grid: +350 W (800 W made, 450 W used),
// stored 900 of 1200 Wh".
func gridLine(n game.PowerNetView) string {
	s := fmt.Sprintf("Grid: %+d W (%d W made, %d W used)", n.Gen-n.Load, n.Gen, n.Load)
	if n.CapWh > 0 {
		s += fmt.Sprintf(", stored %d of %d Wh", n.StoredWh, n.CapWh)
	}
	return s
}

// powerBars are b's electricity bars: a battery's charge. (A generator's
// and a torch's fuel is the Fuel bar every burner has.)
func powerBars(b game.BuildingView) []paneBar {
	if b.PowerRole == powerStorage && b.CapWh > 0 {
		return []paneBar{{Label: "Charge", Pct: bldgPct(b.StoredWh * 100 / b.CapWh), Color: uiAccent}}
	}
	return nil
}

// fuelLine is what a fuel burner's pane says about its fire: out of fuel,
// burning, a campfire banked down, or a torch the rain put out. "" says
// nothing (a generator holding fuel while switched off: its power lines
// say why).
func fuelLine(b game.BuildingView) string {
	switch {
	case !b.Fuel:
		return ""
	case b.FuelPct == 0:
		return "Out of fuel"
	case b.Burning:
		return "Burning"
	case b.Kind == "campfire":
		return "Banked (warm enough)"
	case b.Kind == torchLampKind:
		return "Not burning"
	}
	return ""
}

// powerCommands are b's power buttons: the Power toggle on every device
// with one, and a heater's or cooler's thermostat.
func (a *App) powerCommands(b game.BuildingView) []cmdButton {
	var out []cmdButton
	if hasPowerToggle(b) {
		desc := "Switch it on or off. A colonist walks over and flicks it, so it takes a moment."
		if b.PowerRole == powerSwitch {
			desc = "Close or open the switch to join or cut the grid. A colonist walks over and flicks it, so it takes a moment."
		}
		if b.PowerWantOn != b.PowerOn {
			desc += " A flick is pending."
		}
		out = append(out, cmdButton{ID: powerToggleCmdID, Label: "Power", Icon: "power_toggle", Hotkey: input.Power,
			Toggle: true, On: b.PowerWantOn, Desc: desc,
			Cmds: []command.Command{{Kind: command.SetPowerWanted, Target: uint64(b.ID), Enabled: !b.PowerWantOn}}})
	}
	if isThermostatBldg(b) {
		now := fmtTarget(b.ThermoTarget)
		out = append(out,
			cmdButton{ID: powerDownCmdID, Label: "-1 °C", Icon: "thermostat_down",
				Desc: "Lower the target temperature by 1 °C. It is " + now + " now.",
				Cmds: []command.Command{{Kind: command.SetThermostat, Target: uint64(b.ID), Count: b.ThermoTarget - powerThermoStep}}},
			cmdButton{ID: powerUpCmdID, Label: "+1 °C", Icon: "thermostat_up",
				Desc: "Raise the target temperature by 1 °C. It is " + now + " now.",
				Cmds: []command.Command{{Kind: command.SetThermostat, Target: uint64(b.ID), Count: b.ThermoTarget + powerThermoStep}}})
	}
	return out
}

// powerSelection is V: the selected device is switched on, or off when it
// is wished on already (SetPowerWanted; a colonist flicks it later). A
// building with no power toggle does nothing.
func (a *App) powerSelection() {
	b, ok := a.selectedBuilding()
	if !ok || !hasPowerToggle(b) {
		return
	}
	a.shell.Enqueue(command.Command{Kind: command.SetPowerWanted, Target: uint64(b.ID), Enabled: !b.PowerWantOn})
}
