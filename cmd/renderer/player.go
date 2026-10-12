package main

import (
	"math"

	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/utils"
)

const (
	eyeHeight    = 1.5
	playerRadius = 0.3
	stepHeight   = 0.25 // anything lower can be walked over
	gravity      = 12.0
	jumpSpeed    = 4.5
)

// collider is a capsule lying on the ground: every point within radius of the
// segment a-b is solid. A segment of zero length makes it a circle.
type collider struct {
	ax, az, bx, bz, radius float64
}

// meshCollider fits a capsule around the part of m that is in the player's
// way: vertices above head height are ignored, so a tree is as wide as its
// trunk and low branches, and a mesh lower than stepHeight has no collider.
func meshCollider[T utils.Numeric](m *mesh.Mesh[T]) (collider, bool) {
	minX, minZ, maxX, maxZ, top := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1), math.Inf(-1)
	for _, v := range m.Vertices {
		x, z := float64(v.X*m.Scale.X), float64(v.Z*m.Scale.Z)
		y := float64(v.Y*m.Scale.Y + m.Translation.Y)
		if y > eyeHeight {
			continue
		}
		minX, maxX, minZ, maxZ, top = min(minX, x), max(maxX, x), min(minZ, z), max(maxZ, z), max(top, y)
	}
	if top < stepHeight {
		return collider{}, false
	}

	// The capsule runs along the longer side of the bounding box.
	halfX, halfZ := (maxX-minX)/2, (maxZ-minZ)/2
	radius := min(halfX, halfZ)
	alongX, alongZ := halfX-radius, halfZ-radius

	sin, cos := math.Sincos(float64(m.Rotation.Y))
	world := func(x, z float64) (float64, float64) {
		return float64(m.Translation.X) + x*cos + z*sin, float64(m.Translation.Z) - x*sin + z*cos
	}
	centreX, centreZ := (minX+maxX)/2, (minZ+maxZ)/2
	c := collider{radius: radius}
	c.ax, c.az = world(centreX-alongX, centreZ-alongZ)
	c.bx, c.bz = world(centreX+alongX, centreZ+alongZ)
	return c, true
}

// pushOut moves the point x, z out of c if it is closer than margin to it.
func (c collider) pushOut(x, z, margin float64) (float64, float64) {
	abx, abz := c.bx-c.ax, c.bz-c.az
	t := 0.0
	if length2 := abx*abx + abz*abz; length2 > 0 {
		t = min(1, max(0, ((x-c.ax)*abx+(z-c.az)*abz)/length2))
	}
	nearX, nearZ := c.ax+abx*t, c.az+abz*t
	dist := math.Hypot(x-nearX, z-nearZ)
	if keep := c.radius + margin; dist < keep && dist > 0 {
		x, z = nearX+(x-nearX)/dist*keep, nearZ+(z-nearZ)/dist*keep
	}
	return x, z
}

func (r *Renderer[T]) onGround() bool { return float64(r.camera.Position.Y) <= eyeHeight }

func (r *Renderer[T]) jump() {
	if r.onGround() {
		r.fallSpeed = -jumpSpeed
	}
}

// stepPlayer applies gravity and keeps the camera on the ground and out of
// the colliders.
func (r *Renderer[T]) stepPlayer() {
	pos := &r.camera.Position
	x, y, z := float64(pos.X), float64(pos.Y), float64(pos.Z)

	r.fallSpeed += gravity * r.deltaTime
	y -= r.fallSpeed * r.deltaTime
	if y <= eyeHeight {
		y, r.fallSpeed = eyeHeight, 0
	}

	for _, c := range r.colliders {
		x, z = c.pushOut(x, z, playerRadius)
	}
	half := groundSize/2 - playerRadius
	x = min(groundX+half, max(groundX-half, x))
	z = min(groundZ+half, max(groundZ-half, z))

	pos.X, pos.Y, pos.Z = T(x), T(y), T(z)
}
