package gesture

import "time"

const tapDuration = 250 * time.Millisecond
const doubleTapInterval = 400 * time.Millisecond
const tapTravel, doubleTapDistance = 24.0, 48.0

// Contact is one touch in the demo's logical coordinates, independent of the
// platform input API. A complete press and release is required for each tap.
type Contact struct {
	ID   int
	X, Y float64
}

// DoubleTap rejects drags, held fingers and multitouch. Its explicit real-time
// clock keeps pause and Activity suspension from extending the gesture window.
type DoubleTap struct {
	down, blocked, pending bool
	contact                Contact
	started, first         time.Time
	firstX, firstY         float64
}

func (d *DoubleTap) Reset() { *d = DoubleTap{} }

func (d *DoubleTap) Update(now time.Time, contacts []Contact) bool {
	if d.pending && (now.Before(d.first) || now.Sub(d.first) > doubleTapInterval) {
		d.pending = false
	}
	if len(contacts) > 1 {
		d.Reset()
		d.blocked = true
		return false
	}
	if len(contacts) == 1 {
		p := contacts[0]
		if d.blocked {
			return false
		}
		if !d.down {
			d.down, d.contact, d.started = true, p, now
		} else if p.ID != d.contact.ID || now.Before(d.started) || now.Sub(d.started) > tapDuration || squaredDistance(p.X, p.Y, d.contact.X, d.contact.Y) > tapTravel*tapTravel {
			d.Reset()
			d.blocked = true
		}
		return false
	}
	if d.blocked {
		d.Reset()
		return false
	}
	if !d.down {
		return false
	}
	d.down = false
	if now.Before(d.started) || now.Sub(d.started) > tapDuration {
		d.pending = false
		return false
	}
	if d.pending && squaredDistance(d.contact.X, d.contact.Y, d.firstX, d.firstY) <= doubleTapDistance*doubleTapDistance {
		d.Reset()
		return true
	}
	d.pending, d.first, d.firstX, d.firstY = true, now, d.contact.X, d.contact.Y
	return false
}

func squaredDistance(x, y, otherX, otherY float64) float64 {
	dx, dy := x-otherX, y-otherY
	return dx*dx + dy*dy
}
