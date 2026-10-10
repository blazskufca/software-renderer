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
