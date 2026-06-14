package main

// Light is a global illumination light source
type light struct {
	direction vec3
}

func lightApplyIntensity(originalColor uint32, percentageFactor float64) uint32 {
	if percentageFactor < 0 {
		percentageFactor = 0
	}
	if percentageFactor > 1 {
		percentageFactor = 1
	}
	a := originalColor & 0xFF000000
	r := uint32(float64(originalColor&0x00FF0000) * percentageFactor)
	g := uint32(float64(originalColor&0x0000FF00) * percentageFactor)
	b := uint32(float64(originalColor&0x000000FF) * percentageFactor)

	newColor := a | (r & 0x00FF0000) | (g & 0x0000FF00) | (b & 0x000000FF)

	return newColor
}
