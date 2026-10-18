package spoon

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/render"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/democonstructionkit/sprites"
)

const columns, rows, cell = 40, 25, 16
const pacmanBands, pacmanSweepSeconds = (rows + 1) / 2, 4.0

// The masks are small authored pixel strokes for half-width katakana, digits
// and punctuation. Mirroring the atlas gives the familiar digital-rain forms.
// They are runtime geometry, not a copied film font or external bitmap asset.
var codeStrokes = [][7]byte{
	{31, 1, 5, 4, 4, 8, 16}, {4, 4, 8, 12, 20, 4, 4}, {4, 31, 17, 1, 2, 4, 8},
	{0, 31, 4, 4, 4, 4, 31}, {4, 31, 4, 12, 20, 4, 4}, {8, 31, 9, 9, 9, 17, 2},
	{4, 31, 4, 31, 4, 4, 4}, {15, 17, 1, 2, 4, 8, 16}, {8, 8, 31, 9, 1, 2, 4},
	{0, 31, 1, 1, 1, 1, 31}, {10, 31, 10, 10, 2, 4, 8}, {16, 8, 1, 17, 2, 4, 24},
	{31, 1, 2, 4, 8, 20, 2}, {8, 31, 9, 10, 8, 8, 7}, {17, 9, 1, 2, 4, 8, 16},
	{15, 17, 9, 6, 2, 4, 8}, {0, 14, 4, 31, 4, 4, 8}, {20, 10, 1, 1, 2, 4, 8},
	{0, 31, 0, 31, 4, 4, 8}, {8, 8, 12, 10, 9, 8, 8}, {4, 4, 31, 4, 4, 8, 16},
	{0, 31, 0, 0, 0, 0, 31}, {31, 1, 2, 12, 10, 17, 0}, {4, 31, 2, 12, 21, 4, 4},
	{1, 1, 2, 2, 4, 8, 16}, {4, 4, 10, 10, 17, 17, 0}, {16, 16, 30, 16, 16, 16, 15},
	{31, 1, 1, 2, 4, 8, 0}, {0, 4, 10, 17, 0, 0, 0}, {4, 31, 4, 4, 21, 21, 4},
	{31, 1, 2, 4, 10, 4, 0}, {14, 0, 14, 0, 14, 0, 0}, {4, 8, 8, 17, 17, 31, 1},
	{1, 17, 10, 4, 10, 17, 0}, {31, 4, 4, 31, 4, 4, 3}, {8, 31, 9, 10, 8, 8, 8},
	{0, 30, 2, 2, 2, 2, 31}, {31, 1, 1, 31, 1, 1, 31}, {31, 0, 31, 1, 2, 4, 8},
	{17, 17, 17, 1, 1, 2, 4}, {10, 10, 10, 10, 11, 18, 18}, {16, 16, 16, 17, 18, 20, 24},
	{31, 17, 17, 17, 17, 17, 31}, {31, 17, 1, 1, 2, 4, 8}, {31, 1, 31, 1, 2, 4, 8},
	{14, 17, 19, 21, 25, 17, 14}, {4, 12, 4, 4, 4, 4, 14}, {14, 17, 1, 2, 4, 8, 31},
	{30, 1, 1, 14, 1, 1, 30}, {2, 6, 10, 18, 31, 2, 2}, {31, 16, 16, 30, 1, 1, 30},
	{14, 16, 16, 30, 17, 17, 14}, {31, 1, 2, 4, 8, 8, 8}, {14, 17, 17, 14, 17, 17, 14},
	{14, 17, 17, 15, 1, 1, 14}, {0, 4, 0, 31, 0, 4, 0}, {0, 0, 0, 31, 0, 0, 0},
}

type revealMessage struct {
	text        string
	column, row int
	start       float64
}

type terminal struct {
	atlas        *ebiten.Image
	renderer     *sprites.FieldRenderer
	style        sprites.FieldStyle
	samples      []sprites.FieldSample
	paints       [columns * rows]ebiten.ColorScale
	letters      [columns * rows]letterCell
	messages     []revealMessage
	pacman       *render.Batch
	white        *ebiten.Image
	seconds      float64
	textRenderer *sprites.FieldRenderer
	textStyle    sprites.FieldStyle
	textSamples  []sprites.FieldSample
}

func newTerminal(grid scrolling.BitmapGrid) (*terminal, error) {
	if grid.Image == nil {
		return nil, fmt.Errorf("deciphering requires the Cuddly prompt atlas")
	}
	textFrames := make([]image.Rectangle, 0, len(decipherAlphabet))
	for _, ch := range decipherAlphabet {
		region, ok := grid.Region(ch)
		rect := image.Rect(int(region.X), int(region.Y), int(region.X+region.Width), int(region.Y+region.Height))
		if !ok || rect.Empty() || !rect.In(grid.Image.Bounds()) {
			return nil, fmt.Errorf("deciphering font has no glyph %q", ch)
		}
		textFrames = append(textFrames, rect)
	}
	im := image.NewNRGBA(image.Rect(0, 0, len(codeStrokes)*8, 10))
	frames := make([]image.Rectangle, len(codeStrokes))
	for i, glyph := range codeStrokes {
		frames[i] = image.Rect(i*8, 0, (i+1)*8, 10)
		for y, bits := range glyph {
			for x := 0; x < 5; x++ {
				if bits>>uint(x)&1 != 0 {
					im.SetNRGBA(i*8+1+x, 1+y, color.NRGBA{255, 255, 255, 255})
				}
			}
		}
	}
	c := &terminal{atlas: ebiten.NewImageFromImage(im), renderer: sprites.NewFieldRenderer(columns * rows), samples: make([]sprites.FieldSample, 0, columns*rows), pacman: render.NewBatch(pacmanBands * 128), white: ebiten.NewImage(1, 1)}
	c.white.Fill(color.White)
	c.textRenderer = sprites.NewFieldRenderer(columns * 2)
	c.textSamples = make([]sprites.FieldSample, 0, columns*2)
	c.textStyle = sprites.FieldStyle{Image: grid.Image, Frames: textFrames, Filter: grid.Filter,
		Appearance: sprites.FieldAppearance{Width: cell, Height: cell},
		Sample: func(s sprites.FieldSample, a *sprites.FieldAppearance) bool {
			if !c.letters[s.Index].settled {
				a.Tint.Scale(.55, .95, .75, 1)
			}
			return true
		},
	}
	c.style = sprites.FieldStyle{Image: c.atlas, Frames: frames, Appearance: sprites.FieldAppearance{Width: 16, Height: 16}, Sample: func(s sprites.FieldSample, a *sprites.FieldAppearance) bool {
		a.Tint = c.paints[s.Index]
		return !c.letters[s.Index].active
	}}
	c.messages = []revealMessage{{text: Credit, column: 7, row: 8, start: SecretsAt + 2}, {text: Slogan, column: (columns - len(Slogan)) / 2, row: 16, start: SecretsAt + 6}}
	return c, nil
}

func pacmanPosition(seconds float64, band int) (x, y float64, right, active bool) {
	age := seconds - PacmanAt
	if age < 0 || age >= pacmanSweepSeconds || band < 0 || band >= pacmanBands {
		return 0, 0, false, false
	}
	u := age / pacmanSweepSeconds
	right = band%2 == 0
	x = -24 + u*(Width+48)
	if !right {
		x = Width - x
	}
	// The last, unpaired row has a half-sized Pac-Man and stays inside the canvas.
	y = float64(band*2*cell) + float64(min(2, rows-band*2)*cell)/2
	return x, y, right, true
}

// Eaten shares each band's mouth position with its rendered Pac-Man. All bands
// sweep together in alternating directions without resurrecting consumed cells.
func Eaten(seconds float64, column, row int) bool {
	if seconds < PacmanAt {
		return false
	}
	x, _, right, active := pacmanPosition(seconds, row/2)
	if !active {
		return true
	}
	center := float64(column*cell + cell/2)
	if right {
		return center < x+8
	}
	return center > x-8
}

func (c *terminal) Update(f kit.Frame) error {
	c.seconds = f.Time
	c.samples = c.samples[:0]
	c.textSamples = c.textSamples[:0]
	clear(c.letters[:])
	freeze := smooth(SecretsAt-2, SecretsAt+3, f.Time)
	for row := 0; row < rows; row++ {
		for col := 0; col < columns; col++ {
			index := row*columns + col
			if Eaten(f.Time, col, row) {
				continue
			}
			clock := min(f.Time, SecretsAt+3)
			speed := 3.4 + noise(col*13+12)*5.5
			tail := 6 + noise(col*13+13)*10
			head := math.Mod(clock*speed+noise(col*13+14)*(rows+20), rows+20)
			distance := math.Mod(head-float64(row)+rows+20, rows+20)
			glow := 0.0
			if distance < tail {
				glow = math.Pow(1-distance/tail, 1.7)
			}
			v := glow*(1-freeze) + (.10+.07*noise(index+78))*freeze
			white := 0.0
			if distance < .7 {
				white = (1 - distance/.7) * (1 - freeze)
			}
			var tint ebiten.ColorScale
			tint.Scale(float32(v*.15+white*.65), float32(v*.92+white*.65), float32(v*.42+white*.65), 1)
			c.paints[index] = tint
			glyph := int(hash(uint32(index*301+int(clock*(5+noise(index+33)*12)))) % uint32(len(codeStrokes)))
			c.samples = append(c.samples, sprites.FieldSample{Index: index, Image: glyph, X: float64(col * cell), Y: float64(row * cell), Scale: 1})
		}
	}
	for _, m := range c.messages {
		for i, ch := range m.text {
			col, row := m.column+i, m.row
			index := row*columns + col
			if Eaten(f.Time, col, row) {
				continue
			}
			age := f.Time - m.start - float64(i)*.022
			letter := decipherCell(ch, index, age)
			c.letters[index] = letter
			if letter.active && letter.glyph >= 0 {
				c.textSamples = append(c.textSamples, sprites.FieldSample{
					Index: index, Image: letter.glyph, X: float64(col * cell), Y: float64(row * cell), Scale: 1,
				})
			}
		}
	}
	return nil
}

func (c *terminal) Draw(dst *ebiten.Image) {
	// Animated Latin masks replace Matrix cells before any final letter locks.
	c.renderer.Draw(dst, c.samples, c.style)
	c.textRenderer.Draw(dst, c.textSamples, c.textStyle)
	c.pacman.Begin(dst, c.white)
	for band := 0; band < pacmanBands; band++ {
		x, y, right, active := pacmanPosition(c.seconds, band)
		if !active {
			continue
		}
		opening := .16 + .95*math.Abs(math.Sin((c.seconds-PacmanAt)*math.Pi*7+float64(band)*.4))
		pixel := float64(min(2, rows-band*2)*cell) / 8
		for py := -4; py < 4; py++ {
			for px := -4; px < 4; px++ {
				xx, yy := float64(px)+.5, float64(py)+.5
				front := xx
				if !right {
					front = -xx
				}
				if xx*xx+yy*yy > 15.6 || front > 0 && math.Abs(yy) < front*opening {
					continue
				}
				if front < .6 && front > -.6 && py == -3 {
					continue
				}
				c.pacman.Rect(x+float64(px)*pixel, y+float64(py)*pixel, pixel, pixel, image.Rect(0, 0, 1, 1), color.NRGBA{255, 223, 30, 255})
			}
		}
	}
	c.pacman.Flush()
}
func (c *terminal) Close() error {
	c.renderer.Close()
	c.textRenderer.Close()
	c.atlas.Deallocate()
	c.white.Deallocate()
	return nil
}
