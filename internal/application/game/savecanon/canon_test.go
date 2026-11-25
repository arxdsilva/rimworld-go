// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savecanon

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io"
	"reflect"
	"strconv"
	"testing"
	"time"
)

type cell struct{ X, Y int }

type item struct {
	Name  string
	Qty   int8
	Tags  map[string][]int
	Where [2]uint16
}

type world struct {
	At      time.Time
	Ratio   float64
	C       complex128
	Raw     []byte
	Items   map[uint64]*item
	ByCell  map[cell]map[uint8]bool
	Grid    map[[2]int]string
	List    []item
	Nested  map[string]map[string]map[int]float32
	Skipped map[int]int
	Neg     int64
	On      bool
}

type header struct {
	Name string
	Ver  int
	Tags map[string]string
}

func sample(n int) (header, world) {
	h := header{Name: "slot", Ver: 7, Tags: map[string]string{}}
	w := world{At: time.Unix(1700000000, 5).UTC(), Ratio: 1.5, C: 2 + 3i, Raw: []byte{9, 8}, Neg: -1 << 40, On: true,
		Items: map[uint64]*item{}, ByCell: map[cell]map[uint8]bool{}, Grid: map[[2]int]string{},
		Nested: map[string]map[string]map[int]float32{}}
	for i := range n {
		id := uint64(i*37%101 + 1)
		h.Tags[strconv.Itoa(i)] = "t"
		w.Items[id] = &item{Name: "i" + strconv.Itoa(i), Qty: int8(i), Tags: map[string][]int{"a": {i}, "b": nil, strconv.Itoa(i): {1, 2}}, Where: [2]uint16{uint16(i), 300}}
		w.ByCell[cell{i, -i}] = map[uint8]bool{uint8(i): true, uint8(i + 1): false}
		w.Grid[[2]int{i, i * 2}] = "g" + strconv.Itoa(i)
		w.List = append(w.List, item{Name: "l" + strconv.Itoa(i)})
		w.Nested[strconv.Itoa(i)] = map[string]map[int]float32{"x": {i: float32(i) / 3, -i: 1}}
	}
	return h, w
}

func encode(t *testing.T, h header, w world) []byte {
	t.Helper()
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	if err := enc.Encode(h); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(&w); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestCanonicalIsOneByteStringAndDecodesToTheSameValue(t *testing.T) {
	h, w := sample(40)
	var want []byte
	plain := map[string]bool{}
	for range 30 {
		raw := encode(t, h, w)
		plain[string(raw)] = true
		got, err := canonicalize(raw)
		if err != nil {
			t.Fatal(err)
		}
		if want == nil {
			want = got
		}
		if !bytes.Equal(got, want) {
			t.Fatal("two encodings of one value have different canonical bytes")
		}
	}
	if len(plain) < 2 {
		t.Fatal("test is vacuous: encoding/gob produced one byte string")
	}
	dec := gob.NewDecoder(bytes.NewReader(want))
	var h2 header
	var w2 world
	if err := dec.Decode(&h2); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&w2); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h, h2) || !reflect.DeepEqual(w.Items, w2.Items) || !reflect.DeepEqual(w.ByCell, w2.ByCell) ||
		!reflect.DeepEqual(w.Grid, w2.Grid) || !reflect.DeepEqual(w.Nested, w2.Nested) || !w.At.Equal(w2.At) || w.C != w2.C || w.Neg != w2.Neg {
		t.Fatal("canonical stream decodes to a different value")
	}
	again, err := canonicalize(want)
	if err != nil || !bytes.Equal(again, want) {
		t.Fatalf("canonical form is not a fixed point: %v", err)
	}
}

func TestCorruptStreamIsAnErrorNotAPanic(t *testing.T) {
	h, w := sample(5)
	raw := encode(t, h, w)
	for n := range len(raw) {
		_, _ = canonicalize(raw[:n])
	}
	if _, err := canonicalize([]byte{0x05, 0x01}); err == nil {
		t.Fatal("short message accepted")
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestEncodeReportsShortWrite(t *testing.T) {
	if err := Encode(shortWriter{}, header{Name: "test"}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
}

func TestOversizedFieldDelta(t *testing.T) {
	v := values{}
	r := &reader{b: appendUint(nil, ^uint64(0))}
	d := &typeDef{name: "test", fields: []field{{name: "x", id: idInt}}}
	if _, err := v.structure(r, d, nil); err == nil {
		t.Fatal("overflowing field delta accepted")
	}
}

func TestCaptureOwnsValuesAndReusesBuffers(t *testing.T) {
	h, w := sample(40)
	want, err := canonicalize(encode(t, h, w))
	if err != nil {
		t.Fatal(err)
	}
	c, err := Capture(nil, h, w)
	if err != nil {
		t.Fatal(err)
	}
	h.Tags["later"] = "mutation"
	w.Items[1].Name = "later"
	got, err := c.CanonicalBytes(nil)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("capture not frozen: %v", err)
	}
	h, w = sample(40)
	c, err = Capture(c.Recycle(), h, w)
	if err != nil {
		t.Fatal(err)
	}
	again, err := c.CanonicalBytes(got[:0])
	if err != nil || !bytes.Equal(again, want) {
		t.Fatalf("recycled capture changed bytes: %v", err)
	}
	b := bytes.NewBufferString("prefix")
	if err := Encode(b, h, w); err != nil || !bytes.Equal(b.Bytes()[6:], want) || b.String()[:6] != "prefix" {
		t.Fatalf("append to destination buffer: %v", err)
	}
}
