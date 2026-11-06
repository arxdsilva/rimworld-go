package architect

// Remembered draw styles (OPT-025): with Camera.RememberStyles on, arming a
// drag tool again brings back the style the player last chose on it this
// session; otherwise every arming starts from the tool's own default.

// StyleMemory is what a shell may offer to switch the memory on. Without it
// (or while it answers false) styles reset, as before.
type StyleMemory interface{ RememberStyles() bool }

// styleMemory is the last style picked per tool.
type styleMemory struct{ byTool map[string]DrawStyle }

// start is the style tool id starts with when armed on shell sh.
func (m *styleMemory) start(sh any, id string) DrawStyle {
	if on, ok := sh.(StyleMemory); ok && on.RememberStyles() {
		if s, ok := m.byTool[id]; ok {
			return s
		}
	}
	return StyleDefault
}

// pick remembers style s for tool id.
func (m *styleMemory) pick(id string, s DrawStyle) {
	if id == "" {
		return
	}
	if m.byTool == nil {
		m.byTool = map[string]DrawStyle{}
	}
	m.byTool[id] = s
}
