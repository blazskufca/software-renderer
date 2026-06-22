//go:build js

package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// On-screen navigation controls. Only built for wasm (js): on desktop the
// keyboard and mouse-look cover navigation, so these buttons would just be
// clutter. The no-op stubs in ui_movement_other.go keep the call sites in
// main.go identical across builds.

// holdButton is an on-screen navigation button. Unlike the toolbar buttons it is
// continuous: action runs every frame the button is pressed (touch or mouse).
type holdButton struct {
	label  string
	x      int
	y      int
	w      int
	h      int
	action func()
}

// movementButtons lays out the on-screen navigation controls for touch devices:
// a translation cross plus fly up/down on the bottom-left, and a look cross on
// the bottom-right. Positions are recomputed each frame so they track resizes.
func (r *Renderer[T]) movementButtons() []holdButton {
	bw := r.scaledPixels(54)
	bh := r.scaledPixels(54)
	gap := r.scaledPixels(6)
	margin := r.scaledPixels(12)
	bottom := r.config.WindowHeight - margin

	row0 := bottom - 3*bh - 2*gap
	row1 := bottom - 2*bh - gap
	row2 := bottom - bh

	// Left translation cross + fly column.
	lc0 := margin
	lc1 := margin + bw + gap
	lc2 := margin + 2*(bw+gap)
	flyCol := margin + 3*(bw+gap)

	// Right look cross.
	rc2 := r.config.WindowWidth - margin - bw
	rc1 := r.config.WindowWidth - margin - 2*bw - gap
	rc0 := r.config.WindowWidth - margin - 3*bw - 2*gap

	return []holdButton{
		{label: "Fwd", x: lc1, y: row0, w: bw, h: bh, action: r.moveForward},
		{label: "Left", x: lc0, y: row1, w: bw, h: bh, action: r.strafeLeft},
		{label: "Right", x: lc2, y: row1, w: bw, h: bh, action: r.strafeRight},
		{label: "Back", x: lc1, y: row2, w: bw, h: bh, action: r.moveBackward},

		{label: "Up", x: flyCol, y: row1, w: bw, h: bh, action: r.moveUp},
		{label: "Down", x: flyCol, y: row2, w: bw, h: bh, action: r.moveDown},

		{label: "^", x: rc1, y: row0, w: bw, h: bh, action: r.lookUp},
		{label: "<", x: rc0, y: row1, w: bw, h: bh, action: r.lookLeft},
		{label: ">", x: rc2, y: row1, w: bw, h: bh, action: r.lookRight},
		{label: "v", x: rc1, y: row2, w: bw, h: bh, action: r.lookDown},
	}
}

// heldPointers returns the screen positions of every currently-pressed pointer:
// all active touches, plus the mouse when its left button is down and the cursor
// is not captured for mouse-look.
func (r *Renderer[T]) heldPointers() [][2]int {
	var pts [][2]int
	for _, id := range ebiten.TouchIDs() {
		x, y := ebiten.TouchPosition(id)
		pts = append(pts, [2]int{x, y})
	}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && ebiten.CursorMode() != ebiten.CursorModeCaptured {
		x, y := ebiten.CursorPosition()
		pts = append(pts, [2]int{x, y})
	}
	return pts
}

func pointInButton(b holdButton, x, y int) bool {
	return x >= b.x && x < b.x+b.w && y >= b.y && y < b.y+b.h
}

// applyMovementButtons runs the action of every navigation button currently held
// by a pointer. Called once per frame for continuous movement.
func (r *Renderer[T]) applyMovementButtons() {
	pts := r.heldPointers()
	if len(pts) == 0 {
		return
	}
	for _, b := range r.movementButtons() {
		for _, p := range pts {
			if pointInButton(b, p[0], p[1]) {
				b.action()
				break
			}
		}
	}
}

// pointInMovementButtons reports whether (x, y) falls on any navigation button.
func (r *Renderer[T]) pointInMovementButtons(x, y int) bool {
	for _, b := range r.movementButtons() {
		if pointInButton(b, x, y) {
			return true
		}
	}
	return false
}

func (r *Renderer[T]) drawMovementButtons(screen *ebiten.Image) {
	held := r.heldPointers()
	for _, b := range r.movementButtons() {
		pressed := false
		for _, p := range held {
			if pointInButton(b, p[0], p[1]) {
				pressed = true
				break
			}
		}
		bg := color.RGBA{35, 38, 45, 200}
		if pressed {
			bg = color.RGBA{43, 120, 210, 245}
		}
		vector.FillRect(screen, float32(b.x), float32(b.y), float32(b.w), float32(b.h), bg, false)
		// Center the label in the square button.
		tx := b.x + (b.w-len(b.label)*glyphWidth)/2
		ty := b.y + (b.h-glyphHeight)/2
		ebitenutil.DebugPrintAt(screen, b.label, tx, ty)
	}
}
