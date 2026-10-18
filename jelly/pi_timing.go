package jelly

import (
	"github.com/olivierh59500/dma40/internal/decipher"
	"math"
)

const PiAt = GreetingSettledAt + 2
const PiCursorAt = PiAt + .7
const PiClickAt = PiCursorAt + 2.1
const PiPageAt = PiClickAt + .3
const piColumns, piRows, piCell = Width / 16, Height / 16, 16
const piCharacters = piColumns * piRows
const piSeed = 4000
const piStagger = .0025
const piX, piY = Width - 32, Height - 32
const piClickX, piClickY = piX + 8, piY + 3
const PiHoldSeconds = 5

// PiSettledTick is the first native frame on which all thousand cells have
// locked. The ending follows five seconds (250 native updates) later.
var piSettledTick = computePiSettledTick()

func PiSettledTick() int { return piSettledTick }

func computePiSettledTick() int {
	latest := 0.0
	for i := 0; i < piCharacters; i++ {
		latest = max(latest, float64(i)*piStagger+decipher.SettleAfter(piSeed+i))
	}
	return int(math.Ceil((PiPageAt+latest)*FPS - 1e-8))
}

func PiEndTick() int { return PiSettledTick() + PiHoldSeconds*FPS }
