package savefmt

import (
	"strings"
	"testing"
)

// TestCheckNameRules: the SAV-004 rules, one row each, with the names
// next to every edge that stay allowed.
func TestCheckNameRules(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, want string
	}{
		{"Base camp", ""},
		{"  Base camp  ", ""},
		{"river_2-north", ""},
		{"", NameEmpty},
		{"   ", NameEmpty},
		{strings.Repeat("a", NameMax), ""},
		{strings.Repeat("a", NameMax+1), NameTooLong},
		{strings.Repeat("é", NameMax+1), NameTooLong},
		{"café", NameChars},
		{"a/b", NameChars},
		{"a.b", NameChars},
		{"tab\there", NameChars},
		{"colony-autosave", NameAutosave},
		{"colony-autosave-3", NameAutosave},
		{"colony autosave", NameAutosave},
		{"Colony-AUTOSAVE-12", NameAutosave},
		{"-autosave", NameAutosave},
		{"autosave", ""},
		{"colony-autosave-x", ""},
		{"autosaved colony", ""},
		{"CON", NameDevice},
		{"con", NameDevice},
		{"Nul", NameDevice},
		{"prn", NameDevice},
		{"aux", NameDevice},
		{"COM1", NameDevice},
		{"lpt9", NameDevice},
		{"COM0", ""},
		{"COM10", ""},
		{"console", ""},
		{"CON 2", ""},
	} {
		if got := CheckName(c.name); got != c.want {
			t.Errorf("CheckName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
