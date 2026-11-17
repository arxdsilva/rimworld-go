package config

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
)

func TestPluginsNormalizeSortsAndDedups(t *testing.T) {
	got := Plugins{Enabled: []string{"zeta", " alpha ", "", "zeta", "beta"}}.Normalize()
	if want := []string{"alpha", "beta", "zeta"}; !slices.Equal(got.Enabled, want) {
		t.Fatalf("Enabled = %v, want %v", got.Enabled, want)
	}
	if got := (Plugins{Enabled: []string{" "}}).Normalize(); got.Enabled != nil {
		t.Fatalf("blank list = %v, want nil", got.Enabled)
	}
}

func TestNewInstallEnablesNoPlugin(t *testing.T) {
	if got := Defaults().Normalize().Plugins.Enabled; got != nil {
		t.Fatalf("defaults enable %v", got)
	}
	b, err := json.Marshal(Defaults().Normalize())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"plugins":{}`)) {
		t.Fatalf("an empty list is written: %s", b)
	}
}
