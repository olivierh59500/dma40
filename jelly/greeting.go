package jelly

import (
	"fmt"
	"image"
	"image/draw"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/sprites"
	"github.com/olivierh59500/dma40/internal/decipher"
)

const greetingAlphabet = decipher.Alphabet + ":!'?"

var greetingLines = [...]string{"BILIZIR FROM DMA", "", "IS THE AUDIENCE", "LISTENING ?"}

type greeting struct {
	texture  *ebiten.Image
	renderer *sprites.FieldRenderer
	style    sprites.FieldStyle
	samples  []sprites.FieldSample
	letters  [64]decipher.Cell
}

func newGreeting(font image.Image) (*greeting, error) {
	g := &greeting{texture: ebiten.NewImageFromImage(greetingQuestionFont(font)), renderer: sprites.NewFieldRenderer(64), samples: make([]sprites.FieldSample, 0, 64)}
	grid, err := presets.BitmapFont("cuddly-loader", g.texture, ebiten.FilterNearest)
	if err != nil {
		g.close()
		return nil, err
	}
	frames := make([]image.Rectangle, 0, len(greetingAlphabet))
	for _, ch := range greetingAlphabet {
		r, ok := grid.Region(ch)
		frame := image.Rect(int(r.X), int(r.Y), int(r.X+r.Width), int(r.Y+r.Height))
		if !ok || frame.Empty() || !frame.In(g.texture.Bounds()) {
			g.close()
			return nil, fmt.Errorf("greeting font has no glyph %q", ch)
		}
		frames = append(frames, frame)
	}
	g.style = sprites.FieldStyle{Image: g.texture, Frames: frames, Filter: ebiten.FilterNearest,
		Appearance: sprites.FieldAppearance{Width: 16, Height: 16},
		Sample: func(s sprites.FieldSample, a *sprites.FieldAppearance) bool {
			if !g.letters[s.Index].Settled {
				a.Tint.Scale(.55, .95, .75, 1)
			}
			return true
		},
	}
	return g, nil
}

// The source loader atlas leaves '?' empty. Assemble its hook and dot from
// the original '2' and '!' strokes, retaining their exact pixels and shading.
// The verified source image and its on-disk checksum remain unchanged.
func greetingQuestionFont(font image.Image) image.Image {
	columns := font.Bounds().Dx() / 16
	if columns < 1 {
		return font
	}
	cell := func(ch rune) image.Point {
		index := int(ch - 32)
		return font.Bounds().Min.Add(image.Pt(index%columns*16, index/columns*16))
	}
	question := cell('?')
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if _, _, _, alpha := font.At(question.X+x, question.Y+y).RGBA(); alpha != 0 {
				return font
			}
		}
	}
	result := image.NewNRGBA(font.Bounds())
	draw.Draw(result, result.Bounds(), font, font.Bounds().Min, draw.Src)
	copyStroke := func(rect image.Rectangle, source image.Point) {
		draw.Draw(result, rect.Add(question), font, source, draw.Src)
	}
	two, exclamation := cell('2'), cell('!')
	copyStroke(image.Rect(0, 0, 16, 6), two)
	copyStroke(image.Rect(6, 6, 12, 8), two.Add(image.Pt(6, 6)))
	copyStroke(image.Rect(6, 8, 10, 10), exclamation.Add(image.Pt(6, 6)))
	copyStroke(image.Rect(6, 12, 10, 14), exclamation.Add(image.Pt(6, 12)))
	return result
}

func (g *greeting) draw(dst *ebiten.Image, seconds float64) {
	g.samples = g.samples[:0]
	index := 0
	top := 200 - float64(len(greetingLines)-1)*16 - 8
	for row, line := range greetingLines {
		x := 185 - float64(len(line))*8
		for col, ch := range line {
			age := seconds - GreetingAt - float64(index)*.022
			letter := decipher.Sample(greetingAlphabet, ch, index+1000, age)
			g.letters[index] = letter
			if letter.Active && letter.Glyph >= 0 {
				g.samples = append(g.samples, sprites.FieldSample{Index: index, Image: letter.Glyph, X: x + float64(col)*16, Y: top + float64(row)*32, Scale: 1})
			}
			index++
		}
	}
	g.renderer.Draw(dst, g.samples, g.style)
}

func (g *greeting) close() {
	if g == nil {
		return
	}
	if g.renderer != nil {
		g.renderer.Close()
	}
	if g.texture != nil {
		g.texture.Deallocate()
	}
}
