package main

import (
	"image/color"

	"github.com/blazskufca/software-renderer/internal/config"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// glyphWidth / glyphHeight are the native cell size of one glyph in the fixed
// ebitenutil debug font. Text is drawn at this native size (cheap, no offscreen
// scaling); button dimensions are derived from it so the box hugs the label with
// no empty space on any side.
const (
	glyphWidth  = 6
	glyphHeight = 16
)

// uiButton is a single clickable toolbar button. active drives the highlighted
// state and action is invoked on click. Buttons are rebuilt every frame so both
// fields can reflect the renderer's current state.
type uiButton struct {
	label  string
	x      int
	y      int
	w      int
	h      int
	active bool
	action func()
}

var renderModeButtonDefs = []struct {
	label string
	mode  config.RenderMode
}{
	{"1 Vert", config.RENDER_WIRE_VERTEX},
	{"2 Wire", config.RENDER_WIRE},
	{"3 Fill", config.RENDER_FILL_TRIANGLE},
	{"4 Fill+Wire", config.RENDER_FILL_TRIANGLE_WIRE},
	{"5 Tex", config.RENDER_TEXTURED},
	{"6 Tex+Wire", config.RENDER_TEXTURED_WIRE},
}

func (r *Renderer[T]) uiScale() float64 {
	sx := float64(r.config.WindowWidth) / 1280.0
	sy := float64(r.config.WindowHeight) / 720.0
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

// textPadX / textPadY are the padding between the label and the button edge.
func (r *Renderer[T]) textPadX() int {
	return r.scaledPixels(9)
}

func (r *Renderer[T]) textPadY() int {
	return r.scaledPixels(5)
}

// buttonWidth / buttonHeight size a button to fit its label exactly (native text
// size plus padding), so there is no empty space regardless of window resolution.
func (r *Renderer[T]) buttonWidth(label string) int {
	return len(label)*glyphWidth + 2*r.textPadX()
}

func (r *Renderer[T]) buttonHeight() int {
	return glyphHeight + 2*r.textPadY()
}

// uiButtons builds the toolbar: row one selects the render mode, row two selects
// the fill/texture rasterizer backends and the cull mode.
func (r *Renderer[T]) uiButtons() []uiButton {
	margin := r.scaledPixels(10)
	gap := r.scaledPixels(8)
	rowGap := r.scaledPixels(8)
	buttonH := r.buttonHeight()

	var buttons []uiButton

	// Row 1: render modes
	x := margin
	y := margin
	for _, def := range renderModeButtonDefs {
		mode := def.mode
		w := r.buttonWidth(def.label)
		buttons = append(buttons, uiButton{
			label:  def.label,
			x:      x,
			y:      y,
			w:      w,
			h:      buttonH,
			active: r.renderMethod == mode,
			action: func() { r.renderMethod = mode },
		})
		x += w + gap
	}

	// Row 2: rasterizer backends and culling
	x = margin
	y = margin + buttonH + rowGap
	row2 := []uiButton{
		{
			label:  "7 Fill SL",
			active: r.fillRasterMode == config.FILL_SCANLINE,
			action: func() { r.setFillRaster(config.FILL_SCANLINE) },
		},
		{
			label:  "8 Fill Pineda",
			active: r.fillRasterMode == config.FILL_PINEDA,
			action: func() { r.setFillRaster(config.FILL_PINEDA) },
		},
		{
			label:  "9 Tex SL",
			active: r.textureRasterMode == config.TEXTURE_SCANLINE,
			action: func() { r.setTextureRaster(config.TEXTURE_SCANLINE) },
		},
		{
			label:  "0 Tex Pineda",
			active: r.textureRasterMode == config.TEXTURE_PINEDA,
			action: func() { r.setTextureRaster(config.TEXTURE_PINEDA) },
		},
		{
			label:  "C Cull",
			active: r.cullMode == config.CULL_BACKFACE,
			action: func() { r.cullMode = config.CULL_BACKFACE },
		},
		{
			label:  "X No Cull",
			active: r.cullMode == config.CULL_NONE,
			action: func() { r.cullMode = config.CULL_NONE },
		},
	}
	for i := range row2 {
		w := r.buttonWidth(row2[i].label)
		row2[i].x = x
		row2[i].y = y
		row2[i].w = w
		row2[i].h = buttonH
		buttons = append(buttons, row2[i])
		x += w + gap
	}

	// Row 3: shading mode
	x = margin
	y = margin + (buttonH+rowGap)*2
	row3 := []uiButton{
		{
			label:  "F Flat",
			active: r.shadingMode == config.SHADING_FLAT,
			action: func() { r.shadingMode = config.SHADING_FLAT },
		},
		{
			label:  "G Gouraud",
			active: r.shadingMode == config.SHADING_GOURAUD,
			action: func() { r.shadingMode = config.SHADING_GOURAUD },
		},
	}
	for i := range row3 {
		w := r.buttonWidth(row3[i].label)
		row3[i].x = x
		row3[i].y = y
		row3[i].w = w
		row3[i].h = buttonH
		buttons = append(buttons, row3[i])
		x += w + gap
	}

	return buttons
}

func (r *Renderer[T]) renderModeToolbarHeight() int {
	return r.scaledPixels(10) + r.buttonHeight()*3 + r.scaledPixels(8)*2 + r.scaledPixels(10)
}

func (r *Renderer[T]) handleRenderModeButtonClick(x, y int) bool {
	for _, button := range r.uiButtons() {
		if x >= button.x && x < button.x+button.w && y >= button.y && y < button.y+button.h {
			button.action()
			return true
		}
	}
	return false
}

func (r *Renderer[T]) drawRenderModeToolbar(screen *ebiten.Image) {
	buttons := r.uiButtons()
	barH := r.renderModeToolbarHeight()

	vector.FillRect(screen, 0, 0, float32(r.config.WindowWidth), float32(barH), color.RGBA{0, 0, 0, 170}, false)

	padX := r.textPadX()
	textOffsetY := r.textPadY()
	for _, button := range buttons {
		bg := color.RGBA{35, 38, 45, 230}
		if button.active {
			vector.FillRect(screen, float32(button.x-2), float32(button.y-2), float32(button.w+4), float32(button.h+4), color.RGBA{135, 190, 255, 255}, false)
			bg = color.RGBA{43, 120, 210, 245}
		}

		vector.FillRect(screen, float32(button.x), float32(button.y), float32(button.w), float32(button.h), bg, false)
		ebitenutil.DebugPrintAt(screen, button.label, button.x+padX, button.y+textOffsetY)
	}
}
