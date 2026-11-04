package windows

// painter is a held left button dragging one value over a window's cells
// (CTL-015). The Schedule paints its brush; Work and Assign paint what the
// first click set, so they Arm it on that click. The Set has one, because
// one window is open at a time.
type painter struct {
	last  string // the cell the drag painted last
	Value int    // what the first click set
	Armed bool   // a press set Value and is still held
}

// Arm remembers the pressed cell and the value it took.
func (p *painter) Arm(id string, value int) { p.last, p.Value, p.Armed = id, value, true }

// Entered is the cell id the held button just moved onto. id is the cell
// under the cursor, "" for none. Leaving the cells lets the same one be
// entered again; the caller skips a cell that already has the value.
func (p *painter) Entered(down bool, id string) (string, bool) {
	if !down {
		p.Armed = false
		return "", false
	}
	if id == "" {
		p.last = ""
		return "", false
	}
	if id == p.last {
		return "", false
	}
	p.last = id
	return id, true
}
