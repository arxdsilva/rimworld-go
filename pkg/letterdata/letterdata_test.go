// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package letterdata

import (
	"reflect"
	"testing"
)

func TestDefaultsAndTypedArgs(t *testing.T) {
	t.Parallel()
	const title, format = "Raid", "Raid! %d people from %s."
	args := []string{"3", "Heron-Eye Clan"}
	code, fields := Defaults("", nil, title, format, args)
	if code == "" || code != Code(title, format) || code == Code(title, "another event") {
		t.Fatalf("unstable source event code %q", code)
	}
	want := []Field{{Name: "arg1", Kind: "number", Value: "3"}, {Name: "arg2", Kind: "text", Value: "Heron-Eye Clan"}}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("generic fields = %+v, want %+v", fields, want)
	}
	named := []Field{{Name: "who", Kind: "faction", Value: "Heron-Eye Clan"}}
	code, fields = Defaults("colony.raid", &named, title, format, args)
	if code != "colony.raid" || !reflect.DeepEqual(fields, named) {
		t.Fatalf("explicit code and fields = %q %+v", code, fields)
	}
	fields[0].Value = "modified"
	if named[0].Value != "Heron-Eye Clan" {
		t.Fatal("Defaults shared caller memory")
	}
}
