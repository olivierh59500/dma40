package spoon

import "github.com/olivierh59500/dma40/internal/decipher"

const decipherAlphabet = decipher.Alphabet + "!"
const decipherJumbleSeconds = decipher.JumbleSeconds

type letterCell struct {
	glyph           int
	active, settled bool
}

// decipherCell uses a local clock independent of the frozen Matrix field.
// Two seconds of fast Latin masks precede staggered locks. Both masks and final
// letters use the same atlas; a mask never impersonates the settled character.
func decipherCell(target rune, seed int, age float64) letterCell {
	c := decipher.Sample(decipherAlphabet, target, seed, age)
	return letterCell{glyph: c.Glyph, active: c.Active, settled: c.Settled}
}
