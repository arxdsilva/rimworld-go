package game

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"reflect"
	"slices"

	"rimworld-go/internal/domain/sim/statehash"
)

// StateHash fingerprints the complete authoritative state: everything Save
// writes, decoded and hashed in a normalised, format-independent form
// (statehash.OfFields: fields by name, zero-valued ones skipped, map
// entries ordered by key — Go's random map order makes raw gob bytes vary
// run to run). Research is hashed as one progress list without trailing
// zeros, whichever save fields carry it. So an additive save field (at its
// zero value) or a re-encoding does not move a golden pin; a change of
// behaviour does. It reads the state only.
func (g *Game) StateHash() uint64 {
	var buf bytes.Buffer
	if err := g.Save(&buf, "statehash"); err != nil {
		panic(fmt.Errorf("state hash: %w", err))
	}
	zr, err := gzip.NewReader(&buf)
	if err != nil {
		panic(fmt.Errorf("state hash: %w", err))
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	var s saveState
	if err := dec.Decode(&info); err != nil {
		panic(fmt.Errorf("state hash: %w", err))
	}
	if err := dec.Decode(&s); err != nil {
		panic(fmt.Errorf("state hash: %w", err))
	}
	return statehash.OfFields(normalisedFields(info, &s))
}

// normalisedFields is the save as named fields, with research folded into
// one "Research" progress list (the newest field that holds it).
func normalisedFields(info SaveInfo, s *saveState) map[string]any {
	f := map[string]any{"Info": info}
	v, t := reflect.ValueOf(s).Elem(), reflect.TypeOf(s).Elem()
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() {
			f[t.Field(i).Name] = v.Field(i).Interface()
		}
	}
	research := ints(f["ResearchAll"])
	if research == nil {
		research = ints(f["Research"])
	}
	delete(f, "ResearchAll")
	for len(research) > 0 && research[len(research)-1] == 0 {
		research = research[:len(research)-1]
	}
	f["Research"] = research
	return f
}

// ints is an int array or slice as a fresh []int (nil for anything else).
func ints(x any) []int {
	v := reflect.ValueOf(x)
	if !v.IsValid() || (v.Kind() != reflect.Array && v.Kind() != reflect.Slice) || v.Type().Elem().Kind() != reflect.Int {
		return nil
	}
	out := make([]int, v.Len())
	for i := range out {
		out[i] = int(v.Index(i).Int())
	}
	return slices.Clip(out)
}
