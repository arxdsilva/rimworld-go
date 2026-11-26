// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import (
	"reflect"
	"testing"
)

func TestSnapshotGroupsCoverEveryStateField(t *testing.T) {
	state := reflect.TypeOf(State{})
	snap := reflect.TypeOf(Snapshot{})
	owner := make(map[string]string, state.NumField())
	for i := 0; i < snap.NumField(); i++ {
		group := snap.Field(i)
		for j := 0; j < group.Type.NumField(); j++ {
			field := group.Type.Field(j)
			existing, ok := state.FieldByName(field.Name)
			if !ok {
				t.Errorf("%s.%s has no legacy field", group.Name, field.Name)
			} else if existing.Type != field.Type {
				t.Errorf("%s.%s: type %v, legacy %v", group.Name, field.Name, field.Type, existing.Type)
			}
			if prev := owner[field.Name]; prev != "" {
				t.Errorf("%s has two owners: %s and %s", field.Name, prev, group.Name)
			}
			owner[field.Name] = group.Name
		}
	}
	for i := 0; i < state.NumField(); i++ {
		if field := state.Field(i); owner[field.Name] == "" {
			t.Errorf("live field %s has no component owner", field.Name)
		}
	}
	if len(owner) != state.NumField() {
		t.Fatalf("component map has %d fields, live State has %d", len(owner), state.NumField())
	}
}

// The test fills all current fields, including the body pointer, training
// array and route, so a forgotten conversion cannot pass with zero values.
func TestSnapshotStateRoundTripEveryField(t *testing.T) {
	var a State
	v := reflect.ValueOf(&a).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.Bool:
			field.SetBool(true)
		case reflect.String:
			field.SetString("animal")
		case reflect.Int, reflect.Int32, reflect.Int64, reflect.Int8, reflect.Int16:
			field.SetInt(3)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetUint(3)
		case reflect.Ptr:
			field.Set(reflect.New(field.Type().Elem()))
		case reflect.Slice:
			field.Set(reflect.MakeSlice(field.Type(), 1, 1))
		case reflect.Array:
			for j := 0; j < field.Len(); j++ {
				field.Index(j).SetUint(uint64(j + 1))
			}
		case reflect.Struct:
			field.Set(reflect.New(field.Type()).Elem())
		default:
			t.Fatalf("unexpected field kind %s: %s", v.Type().Field(i).Name, field.Kind())
		}
	}
	if got := SnapshotFromState(a).State(); !reflect.DeepEqual(a, got) {
		t.Fatalf("group roundtrip failed: input %+v, result %+v", a, got)
	}
	if got := SnapshotFromState(State{}).State(); !reflect.DeepEqual(State{}, got) {
		t.Fatalf("zero-value roundtrip: %+v", got)
	}
	snap := SnapshotFromState(a)
	live := &Live{
		Identity: &snap.Identity, Vitals: &snap.Vitals,
		Intent: &snap.Intent, Motion: &snap.Motion, Threat: &snap.Threat,
		Feeding: &snap.Feeding, Handling: &snap.Handling,
		Reproduction: &snap.Reproduction, Medical: &snap.Medical,
	}
	if got := live.State(); !reflect.DeepEqual(a, got) {
		t.Fatalf("live assembly lost a nonzero field: %+v", got)
	}
	live.AssignState(State{})
	if got := live.State(); !reflect.DeepEqual(State{}, got) {
		t.Fatalf("live clearing left residual state: %+v", got)
	}
	live.AssignState(a)
	if got := live.State(); !reflect.DeepEqual(a, got) {
		t.Fatalf("live assignment lost a field: %+v", got)
	}
}
