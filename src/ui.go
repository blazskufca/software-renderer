package main

import (
	"image/color"
	"math"

	"github.com/blazskufca/software-renderer/internal/config"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The debug bitmap font drawn by ebitenutil.DebugPrintAt is a fixed size: each
// glyph occupies a 6x16 px cell and cannot be scaled directly. We render labels
// to native-size images and scale those to fit, so text tracks the button size.
const (
	debugCharWidth  = 6
	debugLineHeight = 16
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

// uiButtons builds the toolbar: row one selects the render mode, row two selects
// the fill and texture rasterizer backends.
func (r *Renderer[T]) uiButtons() []uiButton {
	margin := r.scaledPixels(10)
	gap := r.scaledPixels(8)
	rowGap := r.scaledPixels(8)
	buttonW := r.scaledPixels(122)
	buttonH := r.scaledPixels(32)

	var buttons []uiButton

	// Row 1: render modes
	x := margin
	y := margin
	for _, def := range renderModeButtonDefs {
		mode := def.mode
		buttons = append(buttons, uiButton{
			label:  def.label,
			x:      x,
			y:      y,
			w:      buttonW,
			h:      buttonH,
			active: r.renderMethod == mode,
			action: func() { r.renderMethod = mode },
		})
		x += buttonW + gap
	}

	// Row 2: rasterizer backends
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
		row2[i].x = x
		row2[i].y = y
		row2[i].w = buttonW
		row2[i].h = buttonH
		buttons = append(buttons, row2[i])
		x += buttonW + gap
	}

	return buttons
}

func (r *Renderer[T]) renderModeToolbarHeight() int {
	return r.scaledPixels(10) + r.scaledPixels(32)*2 + r.scaledPixels(8) + r.scaledPixels(10)
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

	vector.FillRect(screen, 0, 0, float32(r.Config.WindowWidth), float32(barH), color.RGBA{0, 0, 0, 170}, false)

	scale := r.labelScale(buttons)
	for _, button := range buttons {
		bg := color.RGBA{35, 38, 45, 230}
		if button.active {
			vector.FillRect(screen, float32(button.x-2), float32(button.y-2), float32(button.w+4), float32(button.h+4), color.RGBA{135, 190, 255, 255}, false)
			bg = color.RGBA{43, 120, 210, 245}
		}

		vector.FillRect(screen, float32(button.x), float32(button.y), float32(button.w), float32(button.h), bg, false)
		r.drawButtonLabel(screen, button, scale)
	}
}

// labelScale returns one text scale shared by every button, sized so the longest
// label fits within the button (with padding) at any resolution. Using a single
// scale keeps all labels the same size rather than each filling its own box.
func (r *Renderer[T]) labelScale(buttons []uiButton) float64 {
	if len(buttons) == 0 {
		return 1
	}
	maxChars := 0
	for _, b := range buttons {
		if len(b.label) > maxChars {
			maxChars = len(b.label)
		}
	}
	if maxChars == 0 {
		return 1
	}

	padX := r.scaledPixels(6)
	padY := r.scaledPixels(4)
	availW := float64(buttons[0].w - 2*padX)
	availH := float64(buttons[0].h - 2*padY)
	textW := float64(maxChars * debugCharWidth)

	scale := math.Min(availW/textW, availH/float64(debugLineHeight))
	// Never upscale: the bitmap font has a fixed native resolution and scaling it
	// up looks blurry. The text only needs to fit, so cap at 1:1 and shrink only
	// when the button is genuinely too small (very low window resolutions).
	if scale > 1 {
		return 1
	}
	if scale <= 0 {
		return 0.1
	}
	return scale
}

// labelImage renders a label to a native-size offscreen image once and caches
// it. The image is resolution-independent; only the draw-time scale changes.
func (r *Renderer[T]) labelImage(label string) *ebiten.Image {
	if r.labelImages == nil {
		r.labelImages = make(map[string]*ebiten.Image)
	}
	if img, ok := r.labelImages[label]; ok {
		return img
	}
	w := len(label) * debugCharWidth
	if w < 1 {
		w = 1
	}
	img := ebiten.NewImage(w, debugLineHeight)
	ebitenutil.DebugPrintAt(img, label, 0, 0)
	r.labelImages[label] = img
	return img
}

// drawButtonLabel draws a button's label scaled by the shared factor and centered
// within the button.
func (r *Renderer[T]) drawButtonLabel(screen *ebiten.Image, b uiButton, scale float64) {
	img := r.labelImage(b.label)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	drawW := float64(w) * scale
	drawH := float64(h) * scale

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	// Snap to integer pixels so the common 1:1 (unscaled) case is pixel-perfect
	// crisp rather than blurred across half-pixel boundaries.
	op.GeoM.Translate(
		math.Floor(float64(b.x)+(float64(b.w)-drawW)/2),
		math.Floor(float64(b.y)+(float64(b.h)-drawH)/2),
	)
	screen.DrawImage(img, op)
}
