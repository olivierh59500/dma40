package spoon

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/render"
	"github.com/olivierh59500/dma40/internal/dmalogo"
)

const finaleLogoWidth, finaleLogoPadding, finaleLogoTop = 180, 32, 2
const finaleLogoVerticalGain = .22

// bilizirLogo applies the source's six-section row table and column cosine.
// Strip sizes and amplitudes fit the existing header; source padding preserves
// every alpha edge while the supplied DMA artwork keeps its proportions.
type bilizirLogo struct {
	clock                                    *motion.WarpTableClock
	warp                                     *composite.StripWarp
	art, source                              *ebiten.Image
	verticalStep                             float64
	frame                                    kit.Frame
	view                                     *ebiten.Image
	settleAt, age, strength, flatMix, travel float64
	verticalBias                             float64
}

func newBilizirLogo(art image.Image) (_ *bilizirLogo, err error) {
	p := &bilizirLogo{settleAt: math.Inf(1), strength: 1}
	defer func() {
		if err != nil {
			p.Close()
		}
	}()
	art, err = dmalogo.Crop(art)
	if err != nil {
		return nil, err
	}
	clock, err := presets.BilizirWarpClockConfig()
	if err != nil {
		return nil, err
	}
	p.verticalStep = clock.VerticalStep
	p.clock, err = motion.NewWarpTableClock(clock)
	if err != nil {
		return nil, err
	}
	config, err := presets.BilizirStripWarpConfig(p.clock)
	if err != nil {
		return nil, err
	}
	config.RowHeight, config.ColumnWidth = 1, 8
	config.VerticalBias *= finaleLogoVerticalGain
	p.verticalBias = config.VerticalBias
	height := int(math.Round(finaleLogoWidth * float64(art.Bounds().Dy()) / float64(art.Bounds().Dx())))
	width := finaleLogoWidth + 2*finaleLogoPadding
	p.warp, err = composite.NewStripWarp(image.Pt(width, height), config)
	if err != nil {
		return nil, err
	}
	margin := presets.BilizirWarpSampleOrigin
	if err = p.warp.SetVariation(composite.WarpVariation{
		HorizontalGain: .5, VerticalGain: finaleLogoVerticalGain, SampleOrigin: float64(margin),
	}); err != nil {
		return nil, err
	}
	// Stable source coordinates keep linear filtering identical during the hold
	// and after the logo passes to Jelly's separately owned still texture.
	p.art = ebiten.NewImageFromImageWithOptions(art, &ebiten.NewImageFromImageOptions{Unmanaged: true})
	p.source = render.NewSurface(width+2*margin, height)
	p.view = render.NewSurface(width, height+2*finaleLogoPadding)
	op := ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(finaleLogoWidth/float64(art.Bounds().Dx()), float64(height)/float64(art.Bounds().Dy()))
	op.GeoM.Translate(float64(margin+finaleLogoPadding), 0)
	p.source.DrawImage(p.art, &op)
	return p, nil
}

func (p *bilizirLogo) Update(f kit.Frame) error {
	p.age = f.Time - p.settleAt
	p.strength = 1 - smooth(LogoFadeSeconds, LogoSettleSeconds, p.age)
	p.travel = smooth(LogoFadeSeconds, LogoSettleSeconds, p.age)
	p.flatMix = smooth(LogoSettleSeconds-.4, LogoSettleSeconds, p.age)
	// Sample the source's 60 Hz clock directly so pause, repeated drawing and
	// backward preview seeks never accumulate a different deformation phase.
	tick := uint64(math.Floor(f.Time*60 + 1e-7))
	p.clock.SetTick(tick)
	p.frame = kit.Frame{Tick: tick, Time: f.Time}
	if err := p.clock.SetPhase(float64(tick) * p.verticalStep); err != nil {
		return err
	}
	return p.warp.SetVariation(composite.WarpVariation{
		HorizontalGain: .5 * p.strength, VerticalGain: finaleLogoVerticalGain * p.strength,
		SampleOrigin: presets.BilizirWarpSampleOrigin,
	})
}
func (p *bilizirLogo) Draw(dst *ebiten.Image) {
	if p.age >= 0 {
		height := float64(p.source.Bounds().Dy())
		width := finaleLogoWidth + (dmalogo.Width-finaleLogoWidth)*p.travel
		centerY := finaleLogoTop + height/2 + (dmalogo.CenterY-finaleLogoTop-height/2)*p.travel
		if p.flatMix < 1 {
			p.view.Clear()
			p.warp.DrawAt(p.view, p.source, p.frame, 0, finaleLogoPadding-p.verticalBias*(1-p.strength))
			var options ebiten.DrawImageOptions
			options.Filter = ebiten.FilterLinear
			scale := width / finaleLogoWidth
			options.GeoM.Scale(scale, scale)
			options.GeoM.Translate((Width-float64(p.view.Bounds().Dx())*scale)/2, centerY-(finaleLogoPadding+height/2)*scale)
			options.ColorScale.ScaleAlpha(float32(1 - p.flatMix))
			dst.DrawImage(p.view, &options)
		}
		if p.flatMix > 0 {
			dmalogo.Draw(dst, p.art, width, centerY, p.flatMix)
		}
		return
	}
	p.warp.DrawAt(dst, p.source, p.frame, float64(Width-p.warp.Size().X)/2, finaleLogoTop)
}
func (p *bilizirLogo) Close() error {
	if p.view != nil {
		p.view.Deallocate()
		p.view = nil
	}
	if p.warp != nil {
		p.warp.Close()
	}
	if p.source != nil {
		p.source.Deallocate()
	}
	if p.art != nil {
		p.art.Deallocate()
	}
	return nil
}
