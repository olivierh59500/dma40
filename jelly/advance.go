package jelly

import "math"

// NextTick shortens authored phases. The final fully readable page keeps its
// five-second hold even when Space is pressed.
func (s *Scene) NextTick() (int, bool) {
	for _, seconds := range [...]float64{FlashAt, FlashAt + .3, PullbackAt, FloorAt, SphereAt, SphereJellyAt, DropAt, DropAt + firstContact(), OrbitAt, BoidsAt, FlockGatherAt, PortraitAt, PortraitMoveAt, GreetingAt, GreetingSettledAt, PiAt, PiCursorAt, PiClickAt, PiPageAt, float64(PiSettledTick()) / FPS} {
		next := int(math.Round(seconds * FPS))
		if next > int(s.tick) {
			return next, true
		}
	}
	return 0, false
}
