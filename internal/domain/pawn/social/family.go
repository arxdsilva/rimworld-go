package social

// Graph is the family edges KinOf walks.
type Graph interface {
	Parents(ID) [2]ID
	Spouses(ID) []ID
}

// KinOf lists every relation b has to a.
func KinOf(a, b ID, g Graph) []Kind {
	return nil // TODO(M3)
}
