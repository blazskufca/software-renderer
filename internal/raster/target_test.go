package raster

import (
	"bytes"
	"testing"
)

func TestNewTargetRejectsInvalidSizes(t *testing.T) {
	for _, sz := range [][2]int{{0, 480}, {960, 0}, {-16, 16}, {16, -1}, {970, 480}} {
		if _, err := NewTarget(sz[0], sz[1]); err == nil {
			t.Fatalf("NewTarget(%d, %d) succeeded, want error", sz[0], sz[1])
		}
	}
	tg, err := NewTarget(16, 1)
	if err != nil {
		t.Fatalf("NewTarget(16, 1): %v", err)
	}
	if len(tg.Color) != 16 || len(tg.Depth) != 16 || len(tg.Bytes()) != 64 {
		t.Fatalf("NewTarget(16, 1): len Color=%d Depth=%d Bytes=%d, want 16, 16, 64", len(tg.Color), len(tg.Depth), len(tg.Bytes()))
	}
}

func TestBytesAreRGBAView(t *testing.T) {
	tg, err := NewTarget(16, 1)
	if err != nil {
		t.Fatal(err)
	}
	tg.Clear(0x44332211)
	b := tg.Bytes()
	for i := range 16 {
		if got := b[i*4 : i*4+4]; !bytes.Equal(got, []byte{0x11, 0x22, 0x33, 0x44}) {
			t.Fatalf("pixel %d bytes = % x, want 11 22 33 44", i, got)
		}
		if tg.Depth[i] != 1 {
			t.Fatalf("Depth[%d] = %v after Clear, want 1", i, tg.Depth[i])
		}
	}
	tg.Color[3] = 0xDDCCBBAA
	if got := tg.Bytes()[12:16]; !bytes.Equal(got, []byte{0xAA, 0xBB, 0xCC, 0xDD}) {
		t.Fatalf("Bytes()[12:16] = % x after Color write, want aa bb cc dd", got)
	}
}

// TestClearResetsEverythingDrawn checks that Clear, which only resets the area
// drawn since the previous Clear, still leaves the whole target cleared.
func TestClearResetsEverythingDrawn(t *testing.T) {
	tg, err := NewTarget(64, 48)
	if err != nil {
		t.Fatal(err)
	}
	cleared := func(step string, color uint32) {
		t.Helper()
		for i := range tg.Color {
			if tg.Color[i] != color || tg.Depth[i] != 1 {
				t.Fatalf("%s: pixel (%d,%d) = %#08x/%v, want %#08x/1", step, i%64, i/64, tg.Color[i], tg.Depth[i], color)
			}
		}
	}
	tg.Clear(0xFF000000)
	cleared("first clear", 0xFF000000)

	// two triangles far apart, so the dirty area spans both
	for _, off := range []float32{2, 40} {
		tg.FillTriangle(&Triangle{Color: 0xFFFFFFFF, V: [3]Vertex{
			{X: off, Y: off - 2, W: 2, Light: 1}, {X: off + 20, Y: off, W: 2, Light: 1}, {X: off, Y: off + 6, W: 2, Light: 1},
		}})
	}
	tg.Clear(0xFF000000)
	cleared("after triangles", 0xFF000000)

	tg.Color[5*64+60], tg.Depth[5*64+60] = 0xFF00FF00, 0.5
	tg.MarkDirty(60, 5, 60, 5)
	tg.Clear(0xFF000000)
	cleared("after MarkDirty", 0xFF000000)

	tg.Clear(0xFF123456) // a new color must reach every pixel, drawn or not
	cleared("new color", 0xFF123456)
}
