package ebiten

// The link grid lives in render/sprite (sprite.LinkGrid); the building
// kinds that link stay with the renderer's kind names (power_draw.go).

// linkKinds is the renderer's sprite.Kinds: buildingLink and powerLink.
type linkKinds struct{}

func (linkKinds) Building(kind string) linkCell { return buildingLink(kind) }
func (linkKinds) Power(kind string) linkCell    { return powerLink(kind) }

// buildingLink is the link cell of a building or frame kind.
func buildingLink(kind string) linkCell {
	switch kind {
	case "wall", "door", coolerKind: // a cooler fills a wall gap like a door
		return wallLink
	case fenceKind, gateKind:
		return fenceLink
	}
	return linkCell{}
}

// powerLink is the power-layer link cell of a building or frame kind:
// conduits link, a switch is a transmitter they join, a device is joined.
func powerLink(kind string) linkCell {
	switch {
	case isConduitKind(kind):
		return conduitLink
	case kind == powerSwitchKind:
		return linkCell{Flags: linkConduit}
	case powerDeviceKinds[kind]:
		return linkCell{Flags: linkDevice}
	}
	return linkCell{}
}
