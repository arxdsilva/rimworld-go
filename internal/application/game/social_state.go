package game

// socialState is the colony's social web (ENG-003, M3-M7): opinion edges,
// couples, gatherings and the social log. nil until the first social
// event; saved as Social.
type socialState struct{}

// socialSave is socialState as saved (filled by M3).
type socialSave struct {
	Version uint8 // format of this block; 0 = empty
}

// exportSocial is the Social save field (nil: nothing yet).
func (g *Game) exportSocial() *socialSave {
	if g.soc == nil {
		return nil
	}
	return nil // TODO(M3)
}

// importSocial restores exportSocial's state (nil: fresh).
func (g *Game) importSocial(s *socialSave) {
	if s == nil {
		return
	}
	// TODO(M3)
}
