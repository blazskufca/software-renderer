package light

import (
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

type Light[T utils.Numeric] struct {
	Direction vector.Vec3[T]
}

// LightApplyIntensity applies light intensity based on original color and percentageFactor
func LightApplyIntensity(originalColor uint32, percentageFactor float32) uint32 {
	if percentageFactor < 0 {
		percentageFactor = 0
	}
	if percentageFactor > 1 {
		percentageFactor = 1
	}
	a := originalColor & 0xFF000000
	r := uint32(float32(originalColor&0x00FF0000) * percentageFactor)
	g := uint32(float32(originalColor&0x0000FF00) * percentageFactor)
	b := uint32(float32(originalColor&0x000000FF) * percentageFactor)

	newColor := a | (r & 0x00FF0000) | (g & 0x0000FF00) | (b & 0x000000FF)

	return newColor
}
