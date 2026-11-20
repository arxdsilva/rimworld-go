// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite

import (
	"bytes"
	"math"
	"testing"
)

func TestLEBUnsigned(t *testing.T) {
	cases := []struct {
		in   []byte
		want uint64
		n    int
		ok   bool
	}{
		{[]byte{0}, 0, 1, true},
		{[]byte{0x7f}, 127, 1, true},
		{[]byte{0x80, 0x01}, 128, 2, true},
		{[]byte{0x80, 0x80, 0x80, 0x80, 0x00}, 0, 5, true}, // overlong
		{[]byte{0xff, 0xff, 0xff, 0xff, 0x0f}, math.MaxUint32, 5, true},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0x1f}, 0, 0, false}, // bit 32 set
		{[]byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x00}, 0, 0, false},
		{[]byte{0x80}, 0, 0, false},
		{nil, 0, 0, false},
		{[]byte{0x01, 0xff}, 1, 1, true},
	}
	for _, c := range cases {
		v, n, err := readU32(c.in)
		if (err == nil) != c.ok || (c.ok && (uint64(v) != c.want || n != c.n)) {
			t.Errorf("readU32(%x) = %d,%d,%v", c.in, v, n, err)
		}
	}
}

func TestLEBU64(t *testing.T) {
	max := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}
	if v, n, err := readU64(max); err != nil || v != math.MaxUint64 || n != 10 {
		t.Fatalf("max: %d %d %v", v, n, err)
	}
	max[9] = 0x02
	if _, _, err := readU64(max); err == nil {
		t.Fatal("bit 64 accepted")
	}
	if _, _, err := readU64(max[:5]); err == nil {
		t.Fatal("truncated accepted")
	}
}

func TestLEBSigned(t *testing.T) {
	s32 := []struct {
		in   []byte
		want int32
		ok   bool
	}{
		{[]byte{0x00}, 0, true},
		{[]byte{0x7f}, -1, true},
		{[]byte{0x3f}, 63, true},
		{[]byte{0x40}, -64, true},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0x07}, math.MaxInt32, true},
		{[]byte{0x80, 0x80, 0x80, 0x80, 0x78}, math.MinInt32, true},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0x7f}, -1, true}, // overlong -1
		{[]byte{0xff, 0xff, 0xff, 0xff, 0x0f}, 0, false}, // not a sign extension
		{[]byte{0x80, 0x80, 0x80, 0x80, 0x70}, 0, false},
		{[]byte{0xff, 0xff}, 0, false},
	}
	for _, c := range s32 {
		v, _, err := readS32(c.in)
		if (err == nil) != c.ok || (c.ok && v != c.want) {
			t.Errorf("readS32(%x) = %d,%v", c.in, v, err)
		}
	}
	s33 := []struct {
		in   []byte
		want int64
		ok   bool
	}{
		{[]byte{0x40}, -64, true},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0x0f}, 1<<32 - 1, true},
		{[]byte{0x80, 0x80, 0x80, 0x80, 0x70}, -(1 << 32), true},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0x1f}, 0, false},
	}
	for _, c := range s33 {
		v, _, err := readS33(c.in)
		if (err == nil) != c.ok || (c.ok && v != c.want) {
			t.Errorf("readS33(%x) = %d,%v", c.in, v, err)
		}
	}
	maxS64 := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x00}
	minS64 := []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x7f}
	if v, _, err := readS64(maxS64); err != nil || v != math.MaxInt64 {
		t.Fatalf("max s64 %d %v", v, err)
	}
	if v, _, err := readS64(minS64); err != nil || v != math.MinInt64 {
		t.Fatalf("min s64 %d %v", v, err)
	}
	maxS64[9] = 0x01
	if _, _, err := readS64(maxS64); err == nil {
		t.Fatal("bad last byte accepted")
	}
	if _, _, err := readS64(minS64[:9]); err == nil {
		t.Fatal("truncated accepted")
	}
}

func TestLEBRoundTrip(t *testing.T) {
	us := []uint64{0, 1, 127, 128, 16383, 16384, 1<<32 - 1, 1 << 32, math.MaxUint64}
	for _, v := range us {
		b := appendU(nil, v)
		got, n, err := readU64(b)
		if err != nil || got != v || n != len(b) {
			t.Errorf("u %d: %x -> %d %v", v, b, got, err)
		}
	}
	ss := []int64{0, 1, -1, 63, 64, -64, -65, math.MaxInt32, math.MinInt32, math.MaxInt64, math.MinInt64}
	for _, v := range ss {
		b := appendS(nil, v)
		got, n, err := readS64(b)
		if err != nil || got != v || n != len(b) {
			t.Errorf("s %d: %x -> %d %v", v, b, got, err)
		}
	}
	if !bytes.Equal(appendU32(nil, 624485), []byte{0xe5, 0x8e, 0x26}) {
		t.Error("624485 encoding")
	}
}

func TestCursorSticky(t *testing.T) {
	c := &cursor{b: []byte{0x80}}
	c.u32()
	c.s32()
	c.s33()
	c.s64()
	c.u64()
	if c.byte() != 0 || c.take(1) != nil || c.count() != 0 || c.err == nil {
		t.Fatal("error did not stick")
	}
	c = &cursor{b: []byte{0x05, 'a'}}
	if c.name(); c.err == nil {
		t.Fatal("short name accepted")
	}
	c = &cursor{b: []byte{0x02, 0xff, 0xfe}}
	if c.name(); c.err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	c = &cursor{b: []byte{0x09}}
	if c.count(); c.err == nil {
		t.Fatal("oversize vector accepted")
	}
}
