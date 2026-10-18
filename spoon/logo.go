package spoon

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/presets"
)

// cuddlyLogo reuses the intro's TCB row-then-column wave chain. Only its working
// dimensions and anchors change to retain the supplied DMA mark's proportions.
type cuddlyLogo struct {
	chain       *composite.WaveChain
	art, source *ebiten.Image
}

func newCuddlyLogo(logo image.Image) (*cuddlyLogo, error) {
	const width, margin = 220, 30
	height := int(math.Round(width * float64(logo.Bounds().Dy()) / float64(logo.Bounds().Dx())))
	size := image.Pt(width+2*margin, height+2*margin)
	passes := presets.CuddlyIntroMainLogoWarp()
	passes[0].Size = size
	passes[0].X = float64(size.X) / 2
	passes[0].Y = margin + .5
	passes[1].Size = size
	passes[1].X = .5
	passes[1].Y = float64(size.Y) / 2
	chain, err := composite.NewWaveChain(passes...)
	if err != nil {
		return nil, err
	}
	p := &cuddlyLogo{chain: chain, art: ebiten.NewImageFromImage(logo), source: ebiten.NewImage(width, height)}
	op := ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(width/float64(logo.Bounds().Dx()), float64(height)/float64(logo.Bounds().Dy()))
	p.source.DrawImage(p.art, &op)
	return p, nil
}

func (p *cuddlyLogo) Update(f kit.Frame) error {
	// Absolute phases keep pause, preview seeking and repeated Draw deterministic.
	for pass := 0; pass < 2; pass++ {
		for i := range p.chain.Pass(pass).Waves {
			wave := &p.chain.Pass(pass).Waves[i]
			wave.Phase = f.Time * 60 * wave.Speed
		}
	}
	return nil
}
func (p *cuddlyLogo) Draw(dst *ebiten.Image) {
	im := p.chain.Render(p.source)
	op := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	op.GeoM.Translate((Width-float64(im.Bounds().Dx()))/2, 195-float64(im.Bounds().Dy())/2)
	dst.DrawImage(im, &op)
}
func (p *cuddlyLogo) Close() error {
	p.chain.Close()
	p.art.Deallocate()
	p.source.Deallocate()
	return nil
}
