//go:build simd

package pipeline

// SetScalarOnly turns the SIMD paths off (true) or back on.
func SetScalarOnly(b bool) { scalarOnly = b }
