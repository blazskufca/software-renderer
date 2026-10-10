package main

import (
	"testing"

	"github.com/blazskufca/software-renderer/internal/config"
)

func TestToolbarLayout(t *testing.T) {
	r := newTestRenderer(t)
	buttons := r.getUIButtons()
	if len(buttons) != 14 {
		t.Fatalf("got %d buttons, want 14", len(buttons))
	}
	barH := r.renderModeToolbarHeight()
	for i, b := range buttons {
		if b.x < 0 || b.y < 0 || b.x+b.w > r.config.WindowWidth || b.y+b.h > barH {
			t.Fatalf("button %d %q rect (%d,%d %dx%d) outside toolbar %dx%d", i, b.label, b.x, b.y, b.w, b.h, r.config.WindowWidth, barH)
		}
		for j, o := range buttons[:i] {
			if b.x < o.x+o.w && o.x < b.x+b.w && b.y < o.y+o.h && o.y < b.y+b.h {
				t.Fatalf("button %d %q overlaps button %d %q", i, b.label, j, o.label)
			}
		}
	}
}

func TestToolbarClickSelectsButton(t *testing.T) {
	n := len(newTestRenderer(t).getUIButtons())
	for i := range n {
		r := newTestRenderer(t)
		b := r.getUIButtons()[i]
		t.Run(b.label, func(t *testing.T) {
			if !r.handleRenderModeButtonClick(b.x+b.w/2, b.y+b.h/2) {
				t.Fatalf("click at centre of button %d %q not handled", i, b.label)
			}
			if !r.getUIButtons()[i].active {
				t.Fatalf("button %d %q not active after click", i, b.label)
			}
		})
	}

	t.Run("outside toolbar", func(t *testing.T) {
		r := newTestRenderer(t)
		before := [5]int{int(r.renderMethod), int(r.fillRasterMode), int(r.textureRasterMode), int(r.cullMode), int(r.shadingMode)}
		if r.handleRenderModeButtonClick(r.config.WindowWidth-1, r.config.WindowHeight-1) {
			t.Fatal("click outside the toolbar was handled")
		}
		after := [5]int{int(r.renderMethod), int(r.fillRasterMode), int(r.textureRasterMode), int(r.cullMode), int(r.shadingMode)}
		if before != after {
			t.Fatalf("state changed from %v to %v", before, after)
		}
	})
}

func TestToolbarHighlightFollowsState(t *testing.T) {
	r := newTestRenderer(t)
	r.getUIButtons() // build the cache before the state changes
	r.setFillRaster(config.FILL_SCANLINE)
	r.cullMode = config.CULL_NONE
	want := map[string]bool{"7 Fill SL": true, "X No Cull": true, "8 Fill Pineda": false, "C Cull": false}
	found := 0
	for _, b := range r.getUIButtons() {
		w, ok := want[b.label]
		if !ok {
			continue
		}
		found++
		if b.active != w {
			t.Fatalf("button %q active=%v, want %v", b.label, b.active, w)
		}
	}
	if found != len(want) {
		t.Fatalf("found %d of %d labelled buttons", found, len(want))
	}
}
