package utils

type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// SwapNumericPtr swaps the pointers to A,B so that B becomes A and A becomes B
// If either is nil function does nothing
func SwapNumericPtr[T Numeric](a, b *T) {
	if a == nil || b == nil {
		return
	}
	*a, *b = *b, *a
}

// Lerp calculates linear interpolation between U and V with factor T according to formula I = u + factor(v-u)
func Lerp[T, U Numeric](u, v, factor T) U {
	return U(u + factor*(v-u))
}
