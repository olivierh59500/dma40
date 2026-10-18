// Native table-driven clocks adapted from go-lockbuster3.
package lockbuster

import "encoding/binary"

// Letter preserves the original signed-word position, velocity and acceleration.
type Letter struct{ Words [8]int16 }

func ReadLetters(data []byte) [3]Letter {
	var letters [3]Letter
	for i := range letters {
		for j := range letters[i].Words {
			letters[i].Words[j] = int16(binary.BigEndian.Uint16(data[i*16+j*2:]))
		}
	}
	return letters
}
func (l *Letter) Step() {
	w := &l.Words
	w[0] += w[2]
	w[1] += w[3]
	w[2] += w[4]
	w[3] += w[5]
	if w[2] == w[6] || w[2] == -w[6] {
		w[4] = -w[4]
	}
	if w[3] == w[7] || w[3] == -w[7] {
		w[5] = -w[5]
	}
}

// PanelPose identifies a cropped frame in one of the three authored sheets.
type PanelPose struct{ Cell, Bank, Frame int }
type PanelClock struct {
	Order                           []uint32
	Cursor, Direction, Cycle, StepN int
	Offset, Base, Mode              int
	Down, Peak                      bool
	Pose                            PanelPose
	Draw, Commit                    bool
}

func NewPanelClock(data []byte) *PanelClock {
	c := &PanelClock{Direction: 1, Cycle: 5, Order: make([]uint32, len(data)/4)}
	for i := range c.Order {
		c.Order[i] = binary.BigEndian.Uint32(data[i*4:])
	}
	return c
}
func (c *PanelClock) Step() {
	c.Draw, c.Commit = false, false
	entry := c.Order[c.Cursor]
	switch entry {
	case 250:
		c.Direction, c.Cursor, c.Cycle, c.StepN, c.Offset, c.Base = 1, 1, 5, 0, 0, 0
		c.Down, c.Peak = false, false
		return
	case 253:
		c.Mode ^= 1
		c.Direction = -1
		c.Cursor--
		c.Offset, c.Base, c.StepN, c.Down = 25600, 25600, 0, false
		return
	case 254:
		c.Cursor += c.Direction
		c.Offset, c.Base, c.StepN, c.Down = 51200, 51200, 0, false
		return
	case 255:
		c.Cursor += c.Direction
		c.Mode ^= 1
		c.Offset = c.Mode * 25600
		c.Base, c.StepN, c.Down = c.Offset, 0, false
		return
	}
	x := int(entry%6400) / 32
	y := int(entry) / 6400
	local := c.Offset % 25600
	c.Pose = PanelPose{Cell: y*5 + x, Bank: c.Offset / 25600, Frame: local/6400*5 + local%6400/32}
	c.Draw = true
	if c.Peak {
		c.Peak = false
		c.Commit = true
		c.Cursor += c.Direction
		return
	}
	if c.Down {
		c.StepN--
		if c.StepN == 0 {
			c.Cycle, c.Down = 5, false
			return
		}
		c.Cycle--
		if c.Cycle == 0 {
			c.Cycle = 5
			c.Offset -= 6240
		}
		c.Offset -= 32
		return
	}
	c.StepN++
	if c.StepN == 20 {
		c.Cycle, c.Down, c.Peak = 5, true, true
		return
	}
	c.Cycle--
	if c.Cycle == 0 {
		c.Cycle = 5
		c.Offset += 6240
	}
	c.Offset += 32
}

// ScrollColumn stores a glyph strip instead of moving or uploading pixel rows.
type ScrollColumn struct {
	Font, Glyph, Column int
	Initial, Row, Slot  int
}
type ScrollClock struct {
	Message            []byte
	Fonts              []int
	Columns            [2][40]ScrollColumn
	Glyph, Column, Row int
	Changes            int
	Remaining          int
}

func NewScrollClock(message []byte) *ScrollClock {
	c := &ScrollClock{Message: append([]byte(nil), message...), Fonts: make([]int, len(message)), Remaining: len(message)}
	bank := 0
	for i, r := range message {
		if r == 5 {
			bank = 1
		} else if r == 6 {
			bank = 0
		}
		c.Fonts[i] = bank
	}
	for row := range c.Columns {
		for slot := range c.Columns[row] {
			c.Columns[row][slot] = ScrollColumn{Initial: 1, Row: row, Slot: slot}
		}
	}
	return c
}
func (c *ScrollClock) Step() {
	wanted := c.Fonts[c.Glyph]
	if wanted != c.Row {
		c.Row = wanted
		if wanted == 0 {
			c.Remaining = len(c.Message) - c.Glyph
		}
		c.Changes++
		return // The original copies the held row once at each handoff.
	}
	r := c.Message[c.Glyph]
	glyph := int(uint8(r - 32))
	if r == ' ' {
		glyph = 3
	}
	copy(c.Columns[c.Row][:39], c.Columns[c.Row][1:])
	c.Columns[c.Row][39] = ScrollColumn{Font: wanted, Glyph: glyph, Column: c.Column}
	c.Column++
	if c.Column == 4 {
		c.Column = 0
		c.Glyph++
		if c.Row == 0 {
			c.Remaining--
		}
		if c.Glyph == len(c.Message) {
			c.Glyph = 0
			c.Remaining = len(c.Message)
		} else if c.Row == 1 && c.Fonts[c.Glyph] == 0 {
			c.Remaining = len(c.Message) - c.Glyph
		}
	}
}
