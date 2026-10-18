// Package displayclock samples live presentation time between fixed updates.
package displayclock

import (
	"math"
	"time"
)

// Clock follows monotonic display time between fixed simulation samples.
// Keeping one epoch avoids velocity changes when a 50 Hz update is absent from
// a 60/90/120 Hz display frame. Seeks, stalls and resume rebase to simulation time.
type Clock struct {
	epoch, lastDraw time.Time
	base, lastTick  float64
	lastValue       float64
}

func (c *Clock) Sample(now time.Time, seconds float64) float64 {
	value := c.base + now.Sub(c.epoch).Seconds()
	if c.epoch.IsZero() || seconds < c.lastTick || now.Sub(c.lastDraw) > 100*time.Millisecond || math.Abs(value-seconds) > .1 {
		c.epoch, c.base, value = now, seconds, seconds
	}
	c.lastDraw, c.lastTick = now, seconds
	c.lastValue = value
	return value
}

// Hold freezes the last displayed pose during an explicit pause. Refreshing the
// epoch preserves that pose on resume instead of catching up suspended time.
func (c *Clock) Hold(now time.Time, seconds float64) float64 {
	if c.epoch.IsZero() {
		c.lastValue = seconds
	}
	c.epoch, c.lastDraw, c.base, c.lastTick = now, now, c.lastValue, seconds
	return c.lastValue
}
