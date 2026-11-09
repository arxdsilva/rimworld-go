// Package memo holds the read models' copy-on-write memos: a list handed
// out again while it reads the same (shared, read only: never written once
// handed out), so a renderer cache keyed on the slice holds.
package memo

// List is a small World() list's memo (the zero value is empty).
type List[T comparable] struct {
	views []T
	ok    bool
}

// Share is the list of n elements at(i): the last one while every element
// is equal (memo on), else a fresh copy; nil when n is 0.
func (m *List[T]) Share(on bool, n int, at func(int) T) []T {
	if n == 0 {
		return nil
	}
	if on && m.ok && len(m.views) == n {
		same := true
		for i := range n {
			if m.views[i] != at(i) {
				same = false
				break
			}
		}
		if same {
			return m.views
		}
	}
	out := make([]T, n)
	for i := range n {
		out[i] = at(i)
	}
	if on {
		m.views, m.ok = out, true
	}
	return out
}
