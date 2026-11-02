package options

import (
	"testing"
)

// A step row writes its value next to its own + button, whichever column
// the row is in: Max colonies once wrote its number in the first column.
func TestStepValueFollowsItsRow(t *testing.T) {
	t.Parallel()
	for _, tab := range []string{"gameplay", "graphics", "audio"} {
		rows := OptionRows(tab)
		for i, r := range rows {
			if r.kind != rowStep {
				continue
			}
			_, cx, _ := OptPos(rows, i)
			plusEnd := cx + OptBtnH + 8 + optBarW + 8 + OptBtnH
			if x := stepValueX(cx); x < plusEnd || x > plusEnd+16 {
				t.Errorf("%s/%s: value at %.0f, the + button ends at %.0f", tab, r.Key, x, plusEnd)
			}
		}
	}
}
