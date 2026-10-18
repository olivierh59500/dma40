// Package startup defines the presentation cues shared by the DCK effects.
package startup

import "github.com/olivierh59500/democonstructionkit/timeline"

const FPS = 50

const (
	LogoAt          = 1.5
	BobsAt          = 4.0
	RastersAt       = 9.0
	ScrollAt        = 14.0
	TransitionTicks = 250
	MusicFadeTicks  = 60
	BobCount        = 60
	BobSpacing      = .13
)

// BobAge gives each sprite its own entrance across the three formations.
func BobAge(seconds float64, index int) float64 {
	age, _, _ := (timeline.Window{Start: BobsAt + float64(index)*BobSpacing}).At(seconds)
	return age
}
