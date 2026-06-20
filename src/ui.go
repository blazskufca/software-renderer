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

func uiScale() float64 {
	sx := float64(windowWidth) / 1280.0
	sy := float64(windowHeight) / 720.0
	if sx < sy {
		return sx
	}
	return sy
}

func scaledPixels(value float64) int {
	scaled := int(value * uiScale())
	if scaled < 1 {
		return 1
	}
	return scaled
}

func renderModeButtons() []renderModeButton {
	margin := scaledPixels(10)
	gap := scaledPixels(8)
	buttonW := scaledPixels(122)
	buttonH := scaledPixels(32)
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

func renderModeToolbarHeight() int {
	return scaledPixels(10) + scaledPixels(32) + scaledPixels(10)
}

func handleRenderModeButtonClick(x, y int) bool {
	for _, button := range renderModeButtons() {
		if x >= button.x && x < button.x+button.w && y >= button.y && y < button.y+button.h {
			renderMethod = button.mode
			return true
		}
	}
	return false
}

func drawRenderModeToolbar(screen *ebiten.Image) {
	buttons := renderModeButtons()
	barH := renderModeToolbarHeight()

	vector.FillRect(screen, 0, 0, float32(windowWidth), float32(barH), color.RGBA{0, 0, 0, 170}, false)

	textOffsetX := scaledPixels(9)
	textOffsetY := scaledPixels(9)
	for _, button := range buttons {
		bg := color.RGBA{35, 38, 45, 230}
		if renderMethod == button.mode {
			vector.FillRect(screen, float32(button.x-2), float32(button.y-2), float32(button.w+4), float32(button.h+4), color.RGBA{135, 190, 255, 255}, false)
			bg = color.RGBA{43, 120, 210, 245}
		}

		vector.FillRect(screen, float32(button.x), float32(button.y), float32(button.w), float32(button.h), bg, false)
		ebitenutil.DebugPrintAt(screen, button.label, button.x+textOffsetX, button.y+textOffsetY)
	}
}
