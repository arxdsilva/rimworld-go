package game

// Factions in play (NG-1 §5.5). Owned by SETUP.

// Factions is the colony's Factions window.
func (g *Game) Factions() []FactionView { return nil } // NG-1 stub (SETUP)

// Story is the colony's storyteller, difficulty and save mode.
func (g *Game) Story() StoryView { return StoryView{} } // NG-1 stub (SETUP)
