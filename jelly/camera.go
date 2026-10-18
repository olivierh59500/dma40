package jelly

import (
	"math"

	"github.com/olivierh59500/democonstructionkit/geometry"
)

const sphereRadius = 250.0
const orbitPeriod, orbitEntrance = 10.0, 2.0
const OrbitSeconds = orbitPeriod + orbitEntrance/2

// sceneView applies one world-to-view transform to the floor, sphere, logo and
// shadow. The camera starts head-on, pulls back, then orbits the shared Y axis.
type sceneView struct {
	camera     geometry.Camera
	target     geometry.Vec3
	yaw, pitch geometry.Rotation
	distance   float64
	angle      float64
}

func orbitAngle(seconds float64) float64 {
	t := max(0, seconds-OrbitAt)
	const entrance = orbitEntrance
	travel := t - entrance/2
	if t < entrance {
		// Integrate an eased angular velocity, avoiding a speed discontinuity
		// when the first orbit joins its indefinitely repeating revolutions.
		travel = t/2 - entrance/(2*math.Pi)*math.Sin(math.Pi*t/entrance)
	}
	return travel * 2 * math.Pi / orbitPeriod
}

func viewAt(seconds float64) sceneView {
	open := ramp(PullbackAt, SphereAt, seconds)
	angle := orbitAngle(seconds)
	// The shared camera descends and rises once during each revolution. The
	// height and angle meet their initial pose again at the full-turn boundary.
	pitch := (.29 + .10*math.Sin(angle)) * open
	radius := 660 + 740*open
	target := geometry.Vec3{Y: -sphereRadius - initialHeight + initialHeight*.5*open}
	v := sceneView{
		camera: geometry.Camera{Center: geometry.Vec2{X: Width / 2, Y: 150 + 20*open}, Focal: 650, Near: 32},
		target: target, distance: radius / math.Cos(pitch), angle: angle,
		yaw: geometry.RotateXYZ(geometry.Vec3{Y: angle}), pitch: geometry.RotateXYZ(geometry.Vec3{X: pitch}),
	}
	// Strafe along camera-right, keeping the floor, shadow and enclosure in
	// the same world. Both lateral sweeps return to zero after the full turn.
	v.target = v.target.Add(v.worldVector(geometry.Vec3{X: 1}).Scale(125 * math.Sin(angle*2)))
	return v
}

func (v sceneView) point(world geometry.Vec3) geometry.Vec3 {
	p := v.pitch.Apply(v.yaw.Apply(world.Sub(v.target)))
	p.Z += v.distance
	return p
}

func (v sceneView) worldVector(camera geometry.Vec3) geometry.Vec3 {
	inverse := func(m geometry.Rotation, p geometry.Vec3) geometry.Vec3 {
		return geometry.Vec3{X: m[0]*p.X + m[3]*p.Y + m[6]*p.Z, Y: m[1]*p.X + m[4]*p.Y + m[7]*p.Z, Z: m[2]*p.X + m[5]*p.Y + m[8]*p.Z}
	}
	return inverse(v.yaw, inverse(v.pitch, camera))
}
