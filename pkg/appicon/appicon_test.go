package appicon

import (
	"bytes"
	"encoding/binary"
	"image"
	"testing"
)

func sizes(ns ...int) []*image.RGBA {
	var out []*image.RGBA
	for _, n := range ns {
		out = append(out, Render(n))
	}
	return out
}

func TestRenderCornersTransparentCentreOpaque(t *testing.T) {
	im := Render(64)
	if _, _, _, a := im.At(0, 0).RGBA(); a != 0 {
		t.Error("corner should be transparent")
	}
	if _, _, _, a := im.At(32, 32).RGBA(); a != 0xffff {
		t.Error("centre should be opaque")
	}
}

func TestEncodeICO(t *testing.T) {
	b, err := EncodeICO(sizes(16, 256))
	if err != nil {
		t.Fatal(err)
	}
	var h [3]uint16
	binary.Read(bytes.NewReader(b), binary.LittleEndian, &h)
	if h != [3]uint16{0, 1, 2} || b[6] != 16 || b[6+16] != 0 {
		t.Errorf("header %v w=%d,%d", h, b[6], b[22])
	}
}
