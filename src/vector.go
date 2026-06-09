package main

import "math"

type vec2 struct {
	X int
	Y int
}

type vec3 struct {
	X float64
	Y float64
	Z float64
}

func (v *vec3) rotateX(angle float64) {
	// Calculate both using original values, then assign simultaneously
	v.Y, v.Z = v.Y*math.Cos(angle)-v.Z*math.Sin(angle), v.Y*math.Sin(angle)+v.Z*math.Cos(angle)
}

func (v *vec3) rotateY(angle float64) {
	v.X, v.Z = v.X*math.Cos(angle)-v.Z*math.Sin(angle), v.X*math.Sin(angle)+v.Z*math.Cos(angle)
}

func (v *vec3) rotateZ(angle float64) {
	v.X, v.Y = v.X*math.Cos(angle)-v.Y*math.Sin(angle), v.X*math.Sin(angle)+v.Y*math.Cos(angle)
}
