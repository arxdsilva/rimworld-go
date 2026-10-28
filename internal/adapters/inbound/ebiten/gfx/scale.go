package gfx

// Scale is one App's scale. world is the map's render scale (0 or
// 1: the map draws at every physical pixel; see worldK).
type Scale struct{ Dev, UI, S, World float64 }

var UnitScale = Scale{Dev: 1, UI: 1, S: 1, World: 1}

// WorldK is the map's render scale: world when it is a reduced one
// (0 < world < 1), else 1.
func (u Scale) WorldK() float64 {
	if u.World > 0 && u.World < 1 {
		return u.World
	}
	return 1
}

// UIScale is the total UI scale the canvas draws with. During a world
// pass into the render-scaled map image (world_scale.go) it is s x world,
// so a world pass's hand conversion (map pixels / UIScale()) still gives
// logical pixels.
func UIScale() float64 {
	if cur.passK != 0 {
		return cur.scale.S * cur.passK
	}
	return cur.scale.S
}

// HUDScale is the UI scale the HUD and every label draw at, also during a
// world pass: text is measured at the scale it is drawn at.
func HUDScale() float64 { return cur.scale.S }

// MapUIScale is map pixels per logical UI pixel (s x the render scale),
// in and out of a world pass: for map <-> logical conversions that also run
// in Update (hit tests).
func MapUIScale() float64 { return cur.scale.S * cur.scale.WorldK() }
