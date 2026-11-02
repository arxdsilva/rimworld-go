package scenario

// SplitKit cuts a starting kit into drop-pod loads (NG-1 §2.11): stacks
// of at most podValue worth, at most maxPods pieces each, in kit order,
// all landing near the colonists (moved out of package game by AX-9).
func SplitKit(kit []Stack, podValue, maxPods int) []Stack {
	var out []Stack
	for _, s := range kit {
		if s.Qty <= 0 {
			continue
		}
		v := s.Kind.Value() * s.Qty
		n := min(max((v+podValue-1)/podValue, 1), maxPods, s.Qty)
		for i := 0; i < n; i++ {
			q := s.Qty / n
			if i == 0 {
				q += s.Qty % n
			}
			out = append(out, Stack{Kind: s.Kind, Qty: q, Near: true, Piles: 1})
		}
	}
	return out
}
