package architect

import "rimworld-go/internal/adapters/inbound/ebiten/ports"

// The catalog's cards and categories in the player's language. Tools and
// Categories stay the English tables the rest of the code and the tests
// key on; toolDefs and categoryDefs build the same tables through the
// catalog, and a Tool keeps the last build until the language changes.

// toolTexts is the translated tables for one language.
type toolTexts struct {
	probe string
	byID  map[string]ToolDef
	cats  []Category
}

// of returns the tables in sh's current language. A language change
// shows as a different translation of the probe strings.
func (x *toolTexts) of(sh ports.Texts) *toolTexts {
	probe := sh.T("Harvest") + "|" + sh.T("Orders") + "|" + sh.T("Mark ripe berry bushes for harvesting.")
	if x.byID == nil || x.probe != probe {
		defs := toolDefs(sh)
		*x = toolTexts{probe: probe, byID: make(map[string]ToolDef, len(defs)), cats: categoryDefs(sh)}
		for _, d := range defs {
			x.byID[d.ID] = d
		}
	}
	return x
}

// localize is d with its label and description in sh's language.
func (x *toolTexts) localize(sh ports.Texts, d ToolDef) ToolDef {
	if l, ok := x.of(sh).byID[d.ID]; ok {
		d.Label, d.Desc = l.Label, l.Desc
	}
	return d
}
