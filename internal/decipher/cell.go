// Package decipher samples the Latin scrambling reveal independently of drawing.
package decipher

import (
	"math"
	"strings"
)

const Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const JumbleSeconds = 2.0

// SettleAfter is the exact lock deadline used by Sample, before a caller's
// per-character delay. It lets a presentation hold after every cell settles.
func SettleAfter(seed int) float64 {
	return JumbleSeconds + .4 + float64(hash(uint32(seed*71+13)))/4294967296*4
}

type Cell struct {
	Glyph           int
	Active, Settled bool
}

// Sample keeps masks changing before staggered locks. A mask never equals the
// settled character. Appended punctuation may use the same original font atlas.
func Sample(alphabet string, target rune, seed int, age float64) Cell {
	if age < 0 {
		return Cell{}
	}
	if target == ' ' {
		return Cell{Glyph: -1, Active: true, Settled: true}
	}
	correct := strings.IndexRune(alphabet, target)
	deadline := SettleAfter(seed)
	if age >= deadline {
		return Cell{Glyph: correct, Active: true, Settled: true}
	}
	step := int(math.Floor(age*25 + 1e-7))
	if age >= JumbleSeconds {
		step = 50 + int(math.Floor((age-JumbleSeconds)*10+1e-7))
	}
	glyph := int(hash(uint32(seed)*977+uint32(step)*7919) % uint32(len(alphabet)-1))
	if glyph >= correct {
		glyph++
	}
	return Cell{Glyph: glyph, Active: true}
}

func hash(n uint32) uint32 {
	n ^= n >> 16
	n *= 0x7feb352d
	n ^= n >> 15
	n *= 0x846ca68b
	return n ^ (n >> 16)
}
