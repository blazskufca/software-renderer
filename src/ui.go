package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type renderModeButton struct {
	label string
	mode  renderMode
	x     int
	y     int
	w     int
	h     int
}

var renderModeButtonDefs = []struct {
	label string
	mode  renderMode
}{
	{"1 Vert", RENDER_WIRE_VERTEX},
	{"2 Wire", RENDER_WIRE},
	{"3 Fill", RENDER_FILL_TRIANGLE},
	{"4 Fill+Wire", RENDER_FILL_TRIANGLE_WIRE},
	{"5 Tex", RENDER_TEXTURED},
	{"6 Tex+Wire", RENDER_TEXTURED_WIRE},
}

func (r *Renderer[T]) uiScale() float64 {
	sx := float64(r.Config.WindowWidth) / 1280.0
	sy := float64(r.Config.WindowHeight) / 720.0
	if sx < sy {
		return sx
	}
	return sy
}

func (r *Renderer[T]) scaledPixels(value float64) int {
	scaled := int(value * r.uiScale())
	if scaled < 1 {
		return 1
	}
	return scaled
}

func (r *Renderer[T]) renderModeButtons() []renderModeButton {
	margin := r.scaledPixels(10)
	gap := r.scaledPixels(8)
	buttonW := r.scaledPixels(122)
	buttonH := r.scaledPixels(32)
	y := margin
	x := margin

	buttons := make([]renderModeButton, len(renderModeButtonDefs))
	for i, def := range renderModeButtonDefs {
		buttons[i] = renderModeButton{
			label: def.label,
			mode:  def.mode,
			x:     x,
			y:     y,
			w:     buttonW,
			h:     buttonH,
		}
		x += buttonW + gap
	}
	return buttons
}

func (r *Renderer[T]) renderModeToolbarHeight() int {
	return r.scaledPixels(10) + r.scaledPixels(32) + r.scaledPixels(10)
}

func (r *Renderer[T]) handleRenderModeButtonClick(x, y int) bool {
	for _, button := range r.renderModeButtons() {
		if x >= button.x && x < button.x+button.w && y >= button.y && y < button.y+button.h {
			r.renderMethod = button.mode
			return true
		}
	}
	return false
}

func (r *Renderer[T]) drawRenderModeToolbar(screen *ebiten.Image) {
	buttons := r.renderModeButtons()
	barH := r.renderModeToolbarHeight()

	vector.FillRect(screen, 0, 0, float32(r.Config.WindowWidth), float32(barH), color.RGBA{0, 0, 0, 170}, false)

	textOffsetX := r.scaledPixels(9)
	textOffsetY := r.scaledPixels(9)
	for _, button := range buttons {
		bg := color.RGBA{35, 38, 45, 230}
		if r.renderMethod == button.mode {
			vector.FillRect(screen, float32(button.x-2), float32(button.y-2), float32(button.w+4), float32(button.h+4), color.RGBA{135, 190, 255, 255}, false)
			bg = color.RGBA{43, 120, 210, 245}
		}

		vector.FillRect(screen, float32(button.x), float32(button.y), float32(button.w), float32(button.h), bg, false)
		ebitenutil.DebugPrintAt(screen, button.label, button.x+textOffsetX, button.y+textOffsetY)
	}
}
