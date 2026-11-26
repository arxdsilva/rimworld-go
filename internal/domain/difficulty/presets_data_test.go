// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package difficulty

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"testing"
)

//go:embed presets.json
var presetJSON []byte

func TestPresetDataContract(t *testing.T) {
	got, err := parsePresets(presetJSON)
	if err != nil || got != presets {
		t.Fatalf("embedded presets differ from loaded presets: %v", err)
	}

	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(presetJSON, &rows); err != nil {
		t.Fatal(err)
	}
	checkRejected := func(name string, mutate func()) {
		t.Helper()
		var copyRows []map[string]json.RawMessage
		if err := json.Unmarshal(presetJSON, &copyRows); err != nil {
			t.Fatal(err)
		}
		rows = copyRows
		mutate()
		data, err := json.Marshal(rows)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parsePresets(data); err == nil {
			t.Errorf("%s: malformed preset data was accepted", name)
		}
	}
	checkRejected("wrong preset key", func() {
		rows[0]["key"] = json.RawMessage(`"seasoned"`)
	})
	checkRejected("missing numeric field", func() {
		var settings map[string]json.RawMessage
		if err := json.Unmarshal(rows[0]["settings"], &settings); err != nil {
			t.Fatal(err)
		}
		delete(settings, "MoodOffset")
		rows[0]["settings"], _ = json.Marshal(settings)
	})
	checkRejected("unknown setting", func() {
		var settings map[string]json.RawMessage
		if err := json.Unmarshal(rows[0]["settings"], &settings); err != nil {
			t.Fatal(err)
		}
		delete(settings, "MoodOffset")
		settings["AnotherFactor"] = json.RawMessage("40")
		rows[0]["settings"], _ = json.Marshal(settings)
	})
	checkRejected("out of range", func() {
		var settings map[string]json.RawMessage
		if err := json.Unmarshal(rows[0]["settings"], &settings); err != nil {
			t.Fatal(err)
		}
		settings["MoodOffset"] = json.RawMessage("200")
		rows[0]["settings"], _ = json.Marshal(settings)
	})
}

func parsePresets(data []byte) ([Custom]Named, error) {
	var out [Custom]Named
	var rows []struct {
		Key         string          `json:"key"`
		Description string          `json:"description"`
		Source      string          `json:"source"`
		Settings    json.RawMessage `json:"settings"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rows); err != nil {
		return out, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return out, fmt.Errorf("trailing data: %v", err)
	}
	if len(rows) != len(out) {
		return out, fmt.Errorf("got %d presets, want %d", len(rows), len(out))
	}
	t := reflect.TypeFor[Settings]()
	var dataFields int
	for i := range t.NumField() {
		if t.Field(i).Tag.Get("json") != "-" {
			dataFields++
		}
	}
	for i, row := range rows {
		if row.Key != presetKeys[i] || row.Description == "" || row.Source != "wiki: AI Storytellers; original tuning" {
			return out, fmt.Errorf("preset %d: invalid key or description", i)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(row.Settings, &raw); err != nil {
			return out, fmt.Errorf("%s: %w", row.Key, err)
		}
		if len(raw) != dataFields {
			return out, fmt.Errorf("%s: got %d settings, want %d", row.Key, len(raw), dataFields)
		}
		for f := range t.NumField() {
			if t.Field(f).Tag.Get("json") == "-" {
				continue
			}
			if _, ok := raw[t.Field(f).Name]; !ok {
				return out, fmt.Errorf("%s: missing %s", row.Key, t.Field(f).Name)
			}
		}
		var settings Settings
		entry := json.NewDecoder(bytes.NewReader(row.Settings))
		entry.DisallowUnknownFields()
		if err := entry.Decode(&settings); err != nil {
			return out, fmt.Errorf("%s: %w", row.Key, err)
		}
		settings.BalanceVersion = 3
		if invalid := Valid(settings); len(invalid) != 0 {
			return out, fmt.Errorf("%s: out of range: %v", row.Key, invalid)
		}
		out[i] = Named{Preset: Preset(i), Key: row.Key, Label: Preset(i).String(), Desc: row.Description, Settings: settings}
	}
	return out, nil
}
