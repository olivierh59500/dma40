package opening

import (
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/dma40/internal/startup"
)

// Loader reuses Union's column-first character reveal and original atlas.
// Each destination identifies the next part of the automatic presentation.
type Loader struct {
	atlas  *ebiten.Image
	reveal *scrolling.Scrolling
	paper  *ebiten.Image
	tick   int
}

func NewLoader(atlas image.Image, destination string) (*Loader, error) {
	l := &Loader{atlas: ebiten.NewImageFromImage(atlas), paper: ebiten.NewImage(Width, Height)}
	font, err := presets.BitmapFont("union-loader", l.atlas, ebiten.FilterNearest)
	if err != nil {
		l.Close()
		return nil, err
	}
	lines := []string{
		"", "", "DMA40 MEGADEMO", "40 YEARS OF DMA", "", destination, "",
		"CODE, PIXELS, MUSIC", "AND FRIENDSHIP", "", "IN MEMORY OF FRIENDS",
		"WHO HAVE LEFT US", "", "YOU ARE PART OF DMA", "", "PLEASE WAIT WHILE",
		"PREPARING THE SCREEN", "", "", "", "", "", "",
	}
	if destination == "FOUR DEMOS" {
		lines = []string{
			"", "", "DMA40 MEGADEMO", "40 YEARS OF DMA", "", destination, "",
			"LOCKBUSTER 3", "JINX RASTER MASTER", "CHONCHON", "DMA IS BACK", "",
			"4 SCREENS TOGETHER", "", "IN MEMORY OF FRIENDS", "WHO HAVE LEFT US", "",
			"PLEASE WAIT WHILE", "PREPARING THE SCREEN", "", "", "", "",
		}
	}
	// Union measures reveal order in a fixed 20-column rectangle, including blanks.
	for i, line := range lines {
		if len(line) > 20 {
			line = line[:20]
		}
		padding := (20 - len(line)) / 2
		lines[i] = strings.Repeat(" ", padding) + line + strings.Repeat(" ", 20-len(line)-padding)
	}
	config := presets.UnionCreditsReveal(font, lines)
	l.reveal, err = scrolling.New(scrolling.Config{Reveal: &scrolling.RevealTransportConfig{Reveal: config,
		TimeAt: func(frame kit.Frame) float64 { return float64(frame.Tick) * 140 * 60 / startup.FPS },
	}})
	if err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}

func (l *Loader) Update(tick int) error {
	l.tick = tick
	return l.reveal.Update(kit.Frame{Tick: uint64(max(0, tick-startup.MusicFadeTicks))})
}
func (l *Loader) Draw(dst *ebiten.Image) {
	dst.Fill(color.Black)
	if l.tick < startup.MusicFadeTicks {
		return
	}
	l.paper.Clear()
	l.reveal.Draw(l.paper)
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(64, 60)
	op.GeoM.Scale(5.0/6, 5.0/6)
	dst.DrawImage(l.paper, &op)
}
func (l *Loader) Close() {
	if l.reveal != nil {
		l.reveal.Close()
	}
	l.atlas.Deallocate()
	l.paper.Deallocate()
}
