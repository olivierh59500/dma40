package jelly

import "math"

// BoidsAt follows a complete camera revolution around the jelly enclosure.
const BoidsAt = OrbitAt + OrbitSeconds
const FlockGatherAt = BoidsAt + 14
const PortraitAt = FlockGatherAt + 5
const PortraitMoveAt = PortraitAt + 4.2
const GreetingAt = PortraitMoveAt + 2.4
const GreetingSettledAt = GreetingAt + 7.5
const Greeting = "BILIZIR FROM DMA\n\nIS THE AUDIENCE LISTENING ?"
const portraitFloor = Height - 8.0

type portraitPose struct {
	x, bottom, size, sx, sy, wobble float64
}

// portraitAt keeps the bottom edge on the visible screen during compression.
// Analytic ballistic arcs make seeking and live interpolation agree exactly.
func portraitAt(seconds float64) portraitPose {
	t := max(0, seconds-PortraitAt)
	const gravity, restitution = 700.0, .70
	const initialBottom = 270.0
	first := math.Sqrt(2 * (portraitFloor - initialBottom) / gravity)
	height := portraitFloor - initialBottom - gravity*t*t/2
	compression := 0.0
	if t >= first {
		age := t - first
		velocity := gravity * first * restitution
		for bounce := 0; bounce < 6; bounce++ {
			flight := 2 * velocity / gravity
			amplitude := .32 * math.Pow(restitution, float64(bounce))
			if age < flight || bounce == 5 {
				height = max(0, velocity*age-gravity*age*age/2)
				compression = amplitude * math.Exp(-age*8) * math.Cos(age*18)
				break
			}
			age -= flight
			velocity *= restitution
		}
	}
	settle := 1 - ease(PortraitAt+3.4, PortraitMoveAt, seconds)
	compression *= settle
	height = max(0, height) * settle
	move := ease(PortraitMoveAt, PortraitMoveAt+2, seconds)
	size := 260 - 20*move
	return portraitPose{x: 320 + 180*move, bottom: (portraitFloor-height)*(1-move) + 320*move,
		size: size, sx: 1 / math.Sqrt(1-compression), sy: 1 - compression, wobble: compression * (1 - move)}
}

func (p portraitPose) point(x, y float64) (float64, float64) {
	nx, ny := x/260-.5, y/260
	// Horizontal shear vanishes at the contact line. Internal vertical wobble
	// is also zero at both edges, so the visible contact cannot slide below it.
	wave := math.Sin(ny * math.Pi)
	return p.x + nx*p.size*p.sx + p.wobble*10*wave,
		p.bottom - (1-ny)*p.size*p.sy + p.wobble*4*wave*math.Sin(nx*math.Pi)
}
