// Package city stages the electric-roadster comic and its DMA rooftop reveal.
package city

import (
	"math"

	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/motion"
)

const Width, Height = 640, 400
const FormedAt, AskAt, ReplyAt, ByeAt, CloseAt = 4.0, 12.0, 15.5, 19.0, 23.0
const LaunchAt, OverheadAt, RooftopAt, PortalAt, Duration = 24.0, 32.0, 37.0, 39.0, 42.0

const Question = "ELON, DOES IT FLY?"
const Answer = "YES, OF COURSE,\nOLIVIER."
const Farewell = "OK, THANKS ELON.\nSEE YOU SOON. BYE."

func PhaseAt(seconds float64) string {
	switch {
	case seconds >= PortalAt:
		return "dma-city-portal"
	case seconds >= OverheadAt:
		return "dma-rooftops"
	case seconds >= LaunchAt:
		return "roadster-flight"
	case seconds >= CloseAt:
		return "roadster-countdown"
	case seconds >= ByeAt:
		return "roadster-farewell"
	case seconds >= ReplyAt:
		return "elon-reply"
	case seconds >= AskAt:
		return "roadster-question"
	case seconds >= FormedAt:
		return "roadster-drive"
	default:
		return "pixels-to-roadster"
	}
}

func ease(a, b, seconds float64) float64 { return motion.Smooth((seconds - a) / (b - a)) }
func mix(a, b, u float64) float64        { return motion.Lerp(a, b, u) }

type route struct{ path *motion.Path }

func newRoute() (route, error) {
	// The same path constructs the actual road corridor and moves the car.
	// Arc-length sampling keeps corners from changing its physical speed.
	path, err := motion.NewSpline([]motion.Point{
		{X: -570, Y: 280}, {X: -380, Y: 280}, {X: -280, Y: 280},
		{X: -245, Y: 240}, {X: -245, Y: 110}, {X: -205, Y: 70},
		{X: -60, Y: 70}, {X: 70, Y: 70}, {X: 110, Y: 25},
		{X: 110, Y: -150}, {X: 155, Y: -195}, {X: 330, Y: -195},
		{X: 390, Y: -250}, {X: 390, Y: -420}, {X: 330, Y: -475},
		{X: 120, Y: -475}, {X: -80, Y: -475}, {X: -145, Y: -415},
		{X: -145, Y: -270}, {X: -100, Y: -225}, {X: 20, Y: -225},
	}, false, 128)
	return route{path}, err
}

func driveDistance(seconds float64) float64 {
	t := max(0, min(LaunchAt, seconds)-FormedAt)
	// Acceleration, a fast first run, then continued driving during the comic.
	if t < 2 {
		return 18 * t * t
	}
	if t < 7 {
		return 72 + 72*(t-2)
	}
	if t < 9 {
		u := (t - 7) / 2
		return 432 + 72*(t-7) - 76*(u*u*u-.5*u*u*u*u)
	}
	return 538 + 34*(t-9)
}

type carPose struct {
	position geometry.Vec3
	rotation geometry.Vec3
	distance float64
}

func (r route) pose(seconds float64) carPose {
	d := driveDistance(seconds)
	p, _ := r.path.At(d)
	back, _ := r.path.At(max(0, d-2))
	ahead, _ := r.path.At(d + 2)
	tangent := motion.Point{X: ahead.X - back.X, Y: ahead.Y - back.Y}
	length := math.Hypot(tangent.X, tangent.Y)
	tangent.X, tangent.Y = tangent.X/length, tangent.Y/length
	_, before := r.path.At(max(0, d-3))
	_, after := r.path.At(d + 3)
	turn := math.Atan2(before.X*after.Y-before.Y*after.X, before.X*after.X+before.Y*after.Y)
	pose := carPose{position: geometry.Vec3{X: p.X, Z: p.Y}, rotation: geometry.Vec3{Y: math.Atan2(tangent.X, tangent.Y), Z: -.25 * turn}, distance: d}
	if seconds <= LaunchAt {
		return pose
	}
	// Lift above the neighboring roofs while still following the real road.
	// Only then leave the corridor for the fast aerial route.
	const liftSeconds = 2.5
	liftDistance := d + 34*min(liftSeconds, seconds-LaunchAt)
	p, _ = r.path.At(liftDistance)
	liftBack, _ := r.path.At(liftDistance - 2)
	liftAhead, _ := r.path.At(liftDistance + 2)
	length = math.Hypot(liftAhead.X-liftBack.X, liftAhead.Y-liftBack.Y)
	tangent = motion.Point{X: (liftAhead.X - liftBack.X) / length, Y: (liftAhead.Y - liftBack.Y) / length}
	altitude := -105*ease(LaunchAt, LaunchAt+3.5, seconds) - 115*ease(LaunchAt+3.5, OverheadAt+2, seconds)
	if seconds < LaunchAt+liftSeconds {
		pose.position = geometry.Vec3{X: p.X, Y: altitude, Z: p.Y}
		pose.rotation = geometry.Vec3{Y: math.Atan2(tangent.X, tangent.Y)}
		return pose
	}
	u := min(1, (seconds-LaunchAt-liftSeconds)/(RooftopAt-LaunchAt-liftSeconds))
	flight := motion.BezierCurve(p, motion.Point{X: p.X + tangent.X*260, Y: p.Y + tangent.Y*260}, motion.Point{X: -220, Y: 770}, motion.Point{X: -80, Y: 960})
	q := flight(u)
	a, b := flight(max(0, u-.002)), flight(min(1, u+.002))
	pose.position = geometry.Vec3{X: q.X, Y: altitude, Z: q.Y}
	pose.rotation = geometry.Vec3{X: .23 * ease(LaunchAt+liftSeconds, LaunchAt+liftSeconds+1, seconds) * (1 - ease(LaunchAt+5, OverheadAt, seconds)), Y: math.Atan2(b.X-a.X, b.Y-a.Y), Z: -.12 * math.Sin(u*math.Pi*2)}
	if seconds > RooftopAt-2 {
		escape := ease(RooftopAt-2, PortalAt, seconds)
		pose.position = pose.position.Add(geometry.Vec3{X: -450 * escape, Y: -110 * escape, Z: 800 * escape})
	}
	return pose
}

type view struct {
	eye, right, down, forward geometry.Vec3
	camera                    geometry.Camera
}

func lookAt(eye, target geometry.Vec3, centerX float64) view {
	f := target.Sub(eye).Unit()
	r := geometry.Vec3{X: f.Z, Z: -f.X}.Unit()
	if r.Dot(r) < .5 {
		r = geometry.Vec3{X: 1}
	}
	return view{eye: eye, right: r, down: f.Cross(r).Unit(), forward: f,
		camera: geometry.Camera{Center: geometry.Vec2{X: centerX, Y: 212}, Focal: 430, Near: .25}}
}

func (v view) point(p geometry.Vec3) geometry.Vec3 {
	d := p.Sub(v.eye)
	return geometry.Vec3{X: d.Dot(v.right), Y: d.Dot(v.down), Z: d.Dot(v.forward)}
}

func (r route) cameraAt(seconds float64) view {
	car := r.pose(seconds)
	rotation := geometry.RotateXYZ(car.rotation)
	forward := rotation.Apply(geometry.Vec3{Z: 1})
	right := rotation.Apply(geometry.Vec3{X: 1})
	eye := car.position.Sub(forward.Scale(10)).Add(right.Scale(4.5)).Add(geometry.Vec3{Y: -3.7})
	target := car.position.Add(forward.Scale(3.4)).Add(geometry.Vec3{Y: -.9})
	center := 320 + 74*ease(ReplyAt-.4, ReplyAt, seconds)*(1-ease(CloseAt, CloseAt+.5, seconds))
	if seconds > LaunchAt {
		flight := ease(LaunchAt, LaunchAt+1.5, seconds)
		eye = geometry.Lerp(eye, car.position.Sub(forward.Scale(15)).Add(right.Scale(6)).Add(geometry.Vec3{Y: -5}), flight)
		target = geometry.Lerp(target, car.position.Add(forward.Scale(2)), flight)
	}
	u := ease(OverheadAt, RooftopAt, seconds)
	eye = geometry.Lerp(eye, geometry.Vec3{Y: -485, Z: 899.99}, u)
	target = geometry.Lerp(target, geometry.Vec3{Y: -28, Z: 900}, u)
	v := lookAt(eye, target, mix(center, 320, u))
	if u > 0 {
		// Settle roll before the overhead limit; a vertical look vector alone
		// cannot choose a stable right axis for the final logo registration.
		angle := math.Atan2(v.right.Z, v.right.X) * (1 - u)
		right := geometry.Vec3{X: math.Cos(angle), Z: math.Sin(angle)}
		v.down = v.forward.Cross(right).Unit()
		v.right = v.down.Cross(v.forward).Unit()
	}
	return v
}
