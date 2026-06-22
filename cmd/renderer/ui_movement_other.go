//go:build !js

package main

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// No-op navigation-control stubs for non-wasm builds. Desktop uses the keyboard
// and mouse-look, so the on-screen buttons (defined in ui_movement_js.go) are
// omitted; these keep the call sites in main.go building unchanged.

func (r *Renderer[T]) applyMovementButtons() {}

func (r *Renderer[T]) pointInMovementButtons(x, y int) bool { return false }

func (r *Renderer[T]) drawMovementButtons(screen *ebiten.Image) {}
