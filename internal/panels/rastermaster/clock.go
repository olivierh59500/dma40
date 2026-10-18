// Native table-driven clocks adapted from go-jinx-rastermaster.
package rastermaster

import (
	"encoding/binary"
	"fmt"
)

type Column struct {
	Glyph, Slice int
	Blank        bool
}
type Clock struct {
	Tick              int
	Logo, From, Small []int
	Message           []byte
	Columns           [40]Column
	Cursor, Column    int
}

func words(data []byte) ([]int, error) {
	if len(data) == 0 || len(data)%4 != 0 {
		return nil, fmt.Errorf("source: invalid word table")
	}
	v := make([]int, len(data)/4)
	for i := range v {
		v[i] = int(int32(binary.BigEndian.Uint32(data[i*4:])))
	}
	return v, nil
}
func NewClock(logo, from, small, message []byte) (*Clock, error) {
	c := &Clock{Message: append([]byte(nil), message...), Column: 4}
	var err error
	if c.Logo, err = words(logo); err != nil {
		return nil, err
	}
	if c.From, err = words(from); err != nil {
		return nil, err
	}
	if c.Small, err = words(small); err != nil {
		return nil, err
	}
	if len(c.Logo) != 1572 || len(c.From) != 1800 || len(c.Small) < 3500 || len(message) == 0 {
		return nil, fmt.Errorf("source: incomplete native clock")
	}
	for i := range c.Columns {
		c.Columns[i].Blank = true
	}
	return c, nil
}
func (c *Clock) Step() {
	c.Tick++
	if c.Column == 4 {
		if c.Cursor == len(c.Message) {
			c.Cursor = 0
		}
		c.Column = 0
	}
	copy(c.Columns[:], c.Columns[1:])
	c.Columns[39] = Column{Glyph: max(0, min(59, int(c.Message[c.Cursor])-32)), Slice: c.Column}
	c.Column++
	if c.Column == 4 {
		c.Cursor++
	}
}

// LogoRow converts the native pre-shifted byte address into an atlas crop.
// An extra preceding bank preserves reads across the original row boundary.
func (c *Clock) LogoRow(row int) (x, y int) {
	offset := c.Logo[c.Tick%1542+row]
	phase := offset / 320
	if offset < 0 && offset%320 != 0 {
		phase--
	}
	return (offset - phase*320) / 8 * 16, row*16 + phase + 1
}
func (c *Clock) FromRow(row int) int {
	tick := max(0, c.Tick-1) % 60
	return c.From[tick*30+row]/64 + 1
}
func (c *Clock) SmallX(lane int) int {
	index := c.Tick
	if index >= 3476 {
		index = 2200 + (index-3476)%1276
	}
	offset := c.Small[index+lane*2]
	return offset/264 - offset%264*8
}

// Rasters reproduces the original ordered palette writes, including mirrored
// thin bars, three bouncing wide bars and the rolling forty-four-row band.
type Rasters struct {
	Blue, Black                            []byte
	Positions                              []int
	Small, Wide, Rolling, Materials        []byte
	Pixels                                 [202]uint16
	Thin                                   [8]int
	WidePosition                           [3]int
	WideVelocity                           [3]int
	RollPosition, RollVelocity, RollOffset int
	BlueBackground                         bool
}

func NewRasters(blue, black, positions, small, wide, rolling, materials []byte) (*Rasters, error) {
	p, err := words(positions)
	if err != nil {
		return nil, err
	}
	if len(blue) != 402 || len(black) != 402 || len(small) != 44 || len(wide) != 44 || len(rolling) != 96 || len(materials) != 800 {
		return nil, fmt.Errorf("source: incomplete raster tables")
	}
	r := &Rasters{Blue: blue, Black: black, Positions: p, Small: small, Wide: wide, Rolling: rolling, Materials: materials, BlueBackground: true, WidePosition: [3]int{0, 80, 160}, WideVelocity: [3]int{4, 4, 4}, RollVelocity: 2}
	for i := range r.Thin {
		r.Thin[i] = (i + 1) * 5
	}
	return r, nil
}
func (r *Rasters) Step() {
	base := r.Black
	if r.BlueBackground {
		base = r.Blue
	}
	for i := 0; i < 201; i++ {
		r.Pixels[i] = binary.BigEndian.Uint16(base[i*2:])
	}
	copyWords := func(at int, data []byte) {
		for i := 0; i < len(data)/2; i++ {
			r.Pixels[at/2+i] = binary.BigEndian.Uint16(data[i*2:])
		}
	}
	for i := range r.Thin {
		index := r.Thin[i]
		if r.Positions[index] == -1 {
			index = 0
		}
		at := r.Positions[index]
		copyWords(at, r.Small[:22])
		copyWords(380-at, r.Small[22:])
		r.Thin[i] = index + 1
	}
	for i := range r.WidePosition {
		if r.WidePosition[i] == 0 {
			r.WideVelocity[i] = 4
		}
		if r.WidePosition[i] == 352 {
			r.WideVelocity[i] = -4
		}
		r.WidePosition[i] += r.WideVelocity[i]
		copyWords(r.WidePosition[i], r.Wide)
	}
	if r.RollPosition == 0 {
		r.RollVelocity, r.RollOffset = 2, 0
	}
	if r.RollPosition == 310 {
		r.RollVelocity, r.RollOffset = -2, 8
	}
	r.RollPosition += r.RollVelocity
	copyWords(r.RollPosition, r.Rolling[r.RollOffset:r.RollOffset+88])
	r.RollOffset += r.RollVelocity
	if r.RollOffset == 8 {
		r.RollOffset = 0
	} else if r.RollOffset == 0 {
		r.RollOffset = 8
	}
	r.Pixels[45], r.Pixels[46] = 0x777, 0x777
}
