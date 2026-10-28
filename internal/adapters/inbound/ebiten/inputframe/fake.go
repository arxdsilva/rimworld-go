package inputframe

// FakeSource is a scripted keyboard and mouse: each Poll returns the
// current state, then clears what only lasts one frame (presses,
// releases, typed characters, the wheel). Tests set Cur between frames.
type FakeSource struct{ Cur Raw }

// Poll returns the scripted state and clears the one-frame parts.
func (f *FakeSource) Poll() Raw {
	r := f.Cur
	f.Cur.Pressed, f.Cur.Chars = nil, nil
	f.Cur.Press, f.Cur.Release = [NumMouse]bool{}, [NumMouse]bool{}
	f.Cur.WheelX, f.Cur.WheelY = 0, 0
	r.Held = append(append([]string(nil), r.Held...), r.Pressed...)
	return r
}
