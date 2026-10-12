package main

import (
	"math"
	"testing"
)

func TestStartIsClearOfColliders(t *testing.T) {
	r := newTestRenderer(t)
	before := r.camera.Position
	r.stepPlayer()
	if r.camera.Position != before {
		t.Fatalf("stepPlayer moved the resting camera from %+v to %+v", before, r.camera.Position)
	}
}

func TestPlayerCannotWalkThroughTheCar(t *testing.T) {
	r := newTestRenderer(t)
	r.deltaTime = 1.0 / 60
	r.camera.Position.X, r.camera.Position.Z = float32(carX), float32(carZ)-4
	r.camera.YawAngle, r.camera.PitchAngle = 0, 0
	for range 240 {
		r.update() // computes camera.Direction
		r.moveForward()
		r.stepPlayer()
	}
	if z := float64(r.camera.Position.Z); z > carZ-1.3 {
		t.Fatalf("walked into the car: z = %v, car at %v", z, carZ)
	}
}

func TestJumpLandsBackOnTheGround(t *testing.T) {
	r := newTestRenderer(t)
	r.deltaTime = 1.0 / 60
	r.jump()
	peak := 0.0
	for range 120 {
		r.stepPlayer()
		peak = math.Max(peak, float64(r.camera.Position.Y))
	}
	if peak <= eyeHeight+0.5 || float64(r.camera.Position.Y) != eyeHeight {
		t.Fatalf("peak %v, landed at %v, want a jump that returns to %v", peak, r.camera.Position.Y, eyeHeight)
	}
}

func TestPlayerStaysOnTheGroundPlane(t *testing.T) {
	r := newTestRenderer(t)
	r.camera.Position.X = 1000
	r.stepPlayer()
	if x := float64(r.camera.Position.X); x > groundX+groundSize/2 {
		t.Fatalf("left the ground: x = %v", x)
	}
}
