package ebiten

import "strings"

// wrapText breaks s into rows of at most cols characters at spaces (a
// longer word is cut).
func wrapText(s string, cols int) []string {
	if cols < 4 {
		cols = 4
	}
	var rows []string
	cur := ""
	for _, w := range strings.Fields(s) {
		for len(w) > cols {
			if cur != "" {
				rows = append(rows, cur)
				cur = ""
			}
			rows = append(rows, w[:cols])
			w = w[cols:]
		}
		switch {
		case cur == "":
			cur = w
		case len(cur)+1+len(w) <= cols:
			cur += " " + w
		default:
			rows = append(rows, cur)
			cur = w
		}
	}
	if cur != "" {
		rows = append(rows, cur)
	}
	return rows
}
