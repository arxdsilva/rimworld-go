package inspector

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// AX-9 travel in the building pane (MAIN_UI_SCREENS §35 travel slice):
// the stasis casket's sleeper and Eject, a transport pod group's load,
// fuel and range with Load pods, Launch and Unload, a pod launcher's
// chemfuel, the long-range mineral scanner's scan and target, and the
// hitching spot's note. Everything is read from BuildingView.AX; Load
// pods and Launch hand the shell an intent (the load dialog, the World
// tab's target picker), the rest are typed commands.

// Travel button ids (journeys press them).
const (
	EjectCasketBtnID = "cmd.bldg.eject_casket"
	LoadPodsBtnID    = "cmd.bldg.load_pods"
	LaunchPodsBtnID  = "cmd.bldg.launch_pods"
	UnloadPodsBtnID  = "cmd.bldg.unload_pods"
	ScanTargetBtnID  = "cmd.bldg.scan_target"
)

// Travel kinds (construction.go names).
const (
	stasisCasketKind   = "stasis casket"
	transportPodKind   = "transport pod"
	podLauncherKind    = "pod launcher"
	mineralScannerKind = "long-range mineral scanner"
	hitchingSpotKind   = "caravan hitching spot"
)

// ScanMinerals are the scanner's targets in its menu's order (game/travel
// Minerals).
func ScanMinerals() []item.Kind {
	return []item.Kind{item.Steel, item.Plasteel, item.Components, item.Silver, item.Gold, item.Uranium, item.Jade}
}

// TravelLines are a travel building's pane lines.
func TravelLines(b game.BuildingView) []string {
	switch b.Kind {
	case stasisCasketKind:
		if b.AX.Sleeper != "" {
			return []string{"Sleeper: " + b.AX.Sleeper, "Needs, aging and illness stand still while asleep"}
		}
		return []string{"Empty: select a colonist and right-click the casket to climb in,", "or right-click a downed colonist to carry them in"}
	case transportPodKind:
		out := []string{fmt.Sprintf("Load  %d / %d kg", b.AX.LoadKg, b.AX.CapKg)}
		if b.AX.Range <= 0 {
			return append(out, "Can't fly: every pod needs a fuelled pod launcher beside it")
		}
		return append(out, fmt.Sprintf("Range  %d tiles (least chemfuel %d)", b.AX.Range, b.AX.FuelUnits))
	case podLauncherKind:
		out := []string{fmt.Sprintf("Chemfuel  %d / 150", b.AX.FuelUnits)}
		if b.AX.Group == 0 {
			return append(out, "Feeds no transport pod: build one beside it")
		}
		return append(out, fmt.Sprintf("Pods reach %d tiles", b.AX.Range))
	case mineralScannerKind:
		target := "any mineral"
		if b.AX.ScanFor != "" {
			target = b.AX.ScanFor
		}
		return []string{fmt.Sprintf("Scanning  %d%%", b.AX.Cycle/10), "Looking for: " + target, "A researcher works it to find world mining sites"}
	case hitchingSpotKind:
		return []string{"Forming caravans gather here and wait before they leave"}
	}
	return nil
}

// TravelCommands are a travel building's buttons; do hands the shell an
// intent (a free function: Building is at its method budget).
func TravelCommands(tr lang.Tr, do func(ports.Intent), b game.BuildingView) []Button {
	switch b.Kind {
	case stasisCasketKind:
		e := Button{ID: EjectCasketBtnID, Label: tr.T("Eject"), Icon: "stasis_casket", Desc: tr.T("Open the casket: the sleeper climbs out at once."),
			Cmds: []command.Command{{Kind: command.EjectCasket, Target: uint64(b.ID)}}}
		if b.AX.Sleeper == "" {
			e.Disabled, e.Reason, e.Cmds = true, "the casket is empty", nil
		}
		return []Button{e}
	case transportPodKind:
		pod := b.ID
		load := Button{ID: LoadPodsBtnID, Label: tr.T("Load pods"), Icon: "transport_pod",
			Desc: tr.T("Pick colonists, animals and goods for this pod group (150 kg a pod)."), Run: func() { do(ports.LoadPods{Pod: pod}) }}
		launch := Button{ID: LaunchPodsBtnID, Label: tr.T("Launch"), Icon: "pod_launcher",
			Desc: tr.T("Pick a world tile in range on the World tab and how the pods land there."), Run: func() { do(ports.LaunchPods{Pod: pod}) }}
		switch {
		case b.AX.LoadKg <= 0:
			launch.Disabled, launch.Reason, launch.Run = true, "the pods are empty", nil
		case b.AX.Range <= 0:
			launch.Disabled, launch.Reason, launch.Run = true, "no fuelled launcher beside every pod", nil
		}
		unload := Button{ID: UnloadPodsBtnID, Label: tr.T("Unload"), Icon: "order_cancel", Desc: tr.T("Set everything aboard down beside the pods and drop the load list."),
			Cmds: []command.Command{{Kind: command.LoadPods, Target: uint64(b.ID)}}}
		return []Button{load, launch, unload}
	case mineralScannerKind:
		label := "Target: any"
		if b.AX.ScanFor != "" {
			label = "Target: " + b.AX.ScanFor
		}
		return []Button{{ID: ScanTargetBtnID, Label: label, Icon: "mineral_scanner",
			Desc:    tr.T("Choose the mineral the scanner looks for (any finds the common ones most often)."),
			Options: func() []Option { return scanOptions(tr, uint64(b.ID)) }}}
	}
	return nil
}

// scanOptions are the scanner target menu's rows: any, then each mineral.
func scanOptions(tr lang.Tr, id uint64) []Option {
	out := []Option{{Label: tr.T("Any mineral"), Cmd: &command.Command{Kind: command.SetScannerTarget, Target: id}}}
	for _, k := range ScanMinerals() {
		out = append(out, Option{Label: Capitalize(k.String()), Cmd: &command.Command{Kind: command.SetScannerTarget, Target: id, Stuff: uint8(k)}})
	}
	return out
}
