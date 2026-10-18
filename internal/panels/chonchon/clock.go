// Native table-driven clocks adapted from go-chonchon.
package chonchon

import (
	"encoding/binary"
	"fmt"
)

type Clock struct {
	Tick, LogoPhase, LogoYPhase, LogoY, ScrollPhase, MessageCursor, Counter int
	LogoPath, LogoYTable, Wave, Message                                     []byte
	Font                                                                    []byte
	Ring                                                                    [2][3432]byte
	Write                                                                   [2]int
	Read                                                                    [2]int
	Background                                                              []byte
	Screen                                                                  [32000]byte
	Stars                                                                   []byte
	StarBase                                                                [100]int
	StarCursor                                                              [100]int
	waveLength                                                              int
}

func NewClock(background, font, path, ys, wave, message, stars, index []byte) (*Clock, error) {
	if len(background) != 32000 || len(path) < 63*6 || len(index) != 600 || len(wave) < 80 {
		return nil, fmt.Errorf("source: incomplete native tables")
	}
	c := &Clock{Background: background, Font: font, LogoPath: path, LogoYTable: ys, Wave: wave, Message: message, Stars: stars, Counter: 0, Write: [2]int{2808, 2808}, Read: [2]int{0, 0}}
	// The extracted range also contains data after the waveform terminator.
	// Wrap the complete visible window within this first authored table only.
	for c.waveLength < len(wave)/2 && int16(binary.BigEndian.Uint16(wave[c.waveLength*2:])) >= 0 {
		c.waveLength++
	}
	if c.waveLength < 2 {
		return nil, fmt.Errorf("source: empty scroll waveform")
	}
	for i := range c.StarBase {
		c.StarBase[i] = int(binary.BigEndian.Uint32(index[i*6:])) - 0x1b018 - 0x1cc88
		c.StarCursor[i] = int(binary.BigEndian.Uint16(index[i*6+4:]))
	}
	c.Step()
	c.Step()
	c.Tick = 0
	return c, nil
}
func (c *Clock) LogoRow(row int) (x, phase, y int) {
	p := c.LogoPhase + row*6
	offset := int(binary.BigEndian.Uint16(c.LogoPath[p:]))
	phase = int(binary.BigEndian.Uint32(c.LogoPath[p+2:])) / 72
	return offset / 8 * 16, phase, c.LogoY + row
}
func (c *Clock) Step() {
	c.Tick++
	copy(c.Screen[:], c.Background)
	c.LogoPhase += 12
	if int16(binary.BigEndian.Uint16(c.LogoPath[c.LogoPhase+63*6:])) < 0 {
		c.LogoPhase = 0
	}
	c.LogoYPhase++
	if c.LogoYPhase*2 >= len(c.LogoYTable) || int16(binary.BigEndian.Uint16(c.LogoYTable[c.LogoYPhase*2:])) < 0 {
		c.LogoYPhase = 0
	}
	y := int(binary.BigEndian.Uint16(c.LogoYTable[c.LogoYPhase*2:]))
	c.LogoY = y / 160
	for i := range c.StarBase {
		p := c.StarBase[i] + c.StarCursor[i] + 6
		if p+6 > len(c.Stars) || binary.BigEndian.Uint16(c.Stars[p:]) == 0 {
			c.StarCursor[i] = 0
			p = c.StarBase[i]
		} else {
			c.StarCursor[i] += 6
		}
		off := int(binary.BigEndian.Uint16(c.Stars[p:]))
		if off+4 <= len(c.Screen) {
			v := binary.BigEndian.Uint32(c.Screen[off:]) | binary.BigEndian.Uint32(c.Stars[p+2:])
			binary.BigEndian.PutUint32(c.Screen[off:], v)
		}
	}
	bank := (c.Tick - 1) % 2
	if bank == 0 && c.Counter == 0 {
		if c.MessageCursor+2 > len(c.Message) || int16(binary.BigEndian.Uint16(c.Message[c.MessageCursor:])) < 0 {
			c.MessageCursor = 0
		}
		off := int(binary.BigEndian.Uint16(c.Message[c.MessageCursor:]))
		c.Write[0] = (c.Write[0] + 312) % 3432
		if off+312 <= len(c.Font) {
			copy(c.Ring[0][c.Write[0]:c.Write[0]+312], c.Font[off:off+312])
		}
	}
	if bank == 1 {
		c.Counter--
		if c.Counter < 0 {
			c.Counter = 1
			if c.MessageCursor+2 > len(c.Message) || int16(binary.BigEndian.Uint16(c.Message[c.MessageCursor:])) < 0 {
				c.MessageCursor = 0
			}
			off := int(int16(binary.BigEndian.Uint16(c.Message[c.MessageCursor:])))
			c.MessageCursor += 2
			c.Write[1] = (c.Write[1] + 312) % 3432
			packed := 0x18369 - 0xce2a + off
			if packed >= 0 && packed+312 <= len(c.Font) {
				for row := 0; row < 26; row++ {
					src := packed + row*12
					dst := c.Write[1] + row*6
					for plane := 0; plane < 3; plane++ {
						a := (dst - 155 + plane*2 + 3432) % 3432
						c.Ring[1][a] = c.Font[src+plane]
					}
					for j := 0; j < 6; j++ {
						c.Ring[1][(dst+j)%3432] = c.Font[src+3+j]
					}
					for plane := 0; plane < 3; plane++ {
						c.Ring[1][(dst+156+plane*2)%3432] = c.Font[src+9+plane]
					}
				}
			}
		}
	}
	c.ScrollPhase = (c.ScrollPhase + 2) % c.waveLength
	c.Read[bank] = (c.Read[bank] + 156) % 3432
	for col := 0; col < 18; col++ {
		p := (c.ScrollPhase + col*2) % c.waveLength
		p2 := (p + 1) % c.waveLength
		y1 := int(binary.BigEndian.Uint16(c.Wave[p*2:])) / 160
		y2 := int(binary.BigEndian.Uint16(c.Wave[p2*2:])) / 160
		for row := 0; row < 26; row++ {
			r := (c.Read[bank] + col*156 + row*6) % 3432
			for plane := 0; plane < 3; plane++ {
				a := (y1+row)*160 + (col+1)*8 + plane*2
				b := (y2+row)*160 + (col+1)*8 + plane*2 + 1
				if a >= 0 && a < len(c.Screen) {
					c.Screen[a] = c.Ring[bank][r+plane*2]
				}
				if b >= 0 && b < len(c.Screen) {
					c.Screen[b] = c.Ring[bank][r+plane*2+1]
				}
			}
		}
	}
}

// Each lookup expands one planar byte into eight byte-sized bit lanes. Four
// shifted lookups recover eight indices without rereading every plane per pixel.
var bitLanes = func() (lanes [256]uint64) {
	for value := range lanes {
		for pixel := range 8 {
			lanes[value] |= uint64(value>>(7-pixel)&1) << (pixel * 8)
		}
	}
	return
}()

func (c *Clock) Pixels(dst []byte) {
	for block := 0; block < len(c.Screen); block += 8 {
		planes := c.Screen[block : block+8]
		pixels := dst[block*8 : block*8+64]
		for half := range 2 {
			packed := bitLanes[planes[half]] | bitLanes[planes[half+2]]<<1 | bitLanes[planes[half+4]]<<2 | bitLanes[planes[half+6]]<<3
			// Every lane is at most 15, so multiplying by 17 cannot carry into
			// a neighboring byte. DCK's palette shader keeps the same red indices.
			packed *= 17
			for pixel := range 8 {
				binary.LittleEndian.PutUint32(pixels[(half*8+pixel)*4:], uint32(byte(packed))|0xff000000)
				packed >>= 8
			}
		}
	}
}
