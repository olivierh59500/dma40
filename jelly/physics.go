package jelly

import "math"

const physicsHz, initialHeight, groundY = 100, 110.0, 0.0
const gravity, flightDuration, contactDuration = 360.0, 1.6, .2
const bouncePeriod = flightDuration + contactDuration

func firstContact() float64 { return math.Sqrt(2 * initialHeight / gravity) }

// bounce is a deterministic vertical body coupled to a damped squash spring.
// DCK applies the resulting volume-preserving scale and spatial waves to the
// complete enclosure and its logo. A fixed flight/contact cycle gives every
// subsequent impact the same cadence while the spring continues ringing.
type bounce struct {
	tick                      int
	height, velocity          float64
	compression, compressionV float64
	contacts                  int
}

func newBounce() bounce { return bounce{height: initialHeight} }

func (b *bounce) sample(seconds float64) {
	target := int(math.Floor(max(0, seconds)*physicsHz + 1e-7))
	if target < b.tick {
		*b = newBounce()
	}
	for b.tick < target {
		b.step()
	}
}

func (b *bounce) step() {
	const dt = 1.0 / physicsHz
	b.tick++
	seconds := float64(b.tick) / physicsHz
	if seconds < firstContact() {
		b.height = initialHeight - .5*gravity*seconds*seconds
		b.velocity = -gravity * seconds
	} else {
		elapsed := seconds - firstContact()
		contacts := int(math.Floor(elapsed/bouncePeriod)) + 1
		if contacts > b.contacts {
			b.compressionV += 11.8
			b.contacts = contacts
		}
		phase := math.Mod(elapsed, bouncePeriod)
		if phase < contactDuration {
			b.height, b.velocity = 0, 0
		} else {
			t := phase - contactDuration
			launch := gravity * flightDuration / 2
			b.height = max(0, launch*t-.5*gravity*t*t)
			b.velocity = launch - gravity*t
		}
	}
	b.compressionV += (-220*b.compression - 7*b.compressionV) * dt
	b.compression += b.compressionV * dt
	if b.compression > .52 {
		b.compression, b.compressionV = .52, min(0, b.compressionV)
	} else if b.compression < -.28 {
		b.compression, b.compressionV = -.28, max(0, b.compressionV)
	}
}
