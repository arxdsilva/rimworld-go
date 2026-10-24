package game

// visitorsFit reports whether a friendly visitor group can arrive (M9).
func (g *Game) visitorsFit(now uint64) bool { return false }

// visitorsArrive brings a friendly visitor group in (M9).
func (g *Game) visitorsArrive(now uint64) bool { return false }

// wandererFits reports whether a wanderer can join (M9).
func (g *Game) wandererFits(now uint64) bool { return false }

// wandererJoins brings a wanderer who joins the colony (M9).
func (g *Game) wandererJoins(now uint64) bool { return false }
