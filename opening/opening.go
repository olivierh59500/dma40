// Package opening composes the authorized DCK effects for the DMA40 opening.
package opening

import (
	"image"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/render"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/democonstructionkit/sprites"
	"github.com/olivierh59500/democonstructionkit/timeline"
	"github.com/olivierh59500/dma40/internal/displayclock"
	"github.com/olivierh59500/dma40/internal/introassets"
	"github.com/olivierh59500/dma40/internal/startup"
)

const Width, Height = 640, 400
const scrollSpeed = 192
const ScrollText = "WELCOME TO THE DMA40 MEGADEMO!"

// Scene owns DCK controllers and borrowed original images. Background effects
// can cross the straight foreground scroller; the entry prompt is drawn last.
type Scene struct {
	cube          *LeonardCube
	logo, rasters *cachedWarp
	balls         [3]*sprites.HarmonicField
	scroll        *scrolling.Scrolling
	grey          scrolling.BitmapGrid
	textures      []*ebiten.Image
	layers        *kit.Layers
	owned         kit.Group
	seconds       float64
	canvas        *ebiten.Image
	dirty         bool
	scrollClock   displayclock.Clock
}

func New(source *introassets.Set) (*Scene, error) { return newScene(source, true) }

// NewSinglePass preserves live choreography while letting an export show the
// complete text once before its scripted transition.
func NewSinglePass(source *introassets.Set) (*Scene, error) { return newScene(source, false) }

func newScene(source *introassets.Set, repeat bool) (_ *Scene, err error) {
	s := &Scene{}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	s.canvas = render.NewSurface(Width, Height)
	texture := func(im image.Image) *ebiten.Image {
		t := ebiten.NewImageFromImage(im)
		s.textures = append(s.textures, t)
		return t
	}
	logo, ball, chrome, bars := texture(source.Logo), texture(source.Ball), texture(source.Chrome), texture(source.Bars)
	s.grey, err = presets.BitmapFont("cuddly-loader", texture(source.FontGrey), ebiten.FilterNearest)
	if err != nil {
		return nil, err
	}
	s.cube, err = newLeonardCube()
	if err != nil {
		return nil, err
	}
	s.owned = append(s.owned, s.cube)
	s.logo, err = s.makeLogo(logo)
	if err != nil {
		return nil, err
	}
	s.owned = append(s.owned, s.logo)
	s.rasters, err = s.makeRasters(bars)
	if err != nil {
		return nil, err
	}
	s.owned = append(s.owned, s.rasters)
	bobs := kit.Group{}
	for group := 0; group < 3; group++ {
		index := group
		style := sprites.FieldStyle{Image: ball, Appearance: sprites.FieldAppearance{Width: 22, Height: 22, AnchorX: .5, AnchorY: .5}}
		style.Sample = func(sample sprites.FieldSample, a *sprites.FieldAppearance) bool {
			age := startup.BobAge(s.seconds, index*20+sample.Index)
			if age < 0 {
				return false
			}
			a.Tint.ScaleAlpha(float32(motion.Smooth(age / .45)))
			return true
		}
		s.balls[group], err = sprites.NewHarmonicField(sprites.HarmonicFieldConfig{Count: 20, Motion: ballFormation(group), ClockScale: [2]float64{85}, PixelSnap: true, Style: style})
		if err != nil {
			return nil, err
		}
		s.owned = append(s.owned, s.balls[group])
		bobs = append(bobs, &bobEntrance{HarmonicField: s.balls[group], group: group})
	}
	fontSpec, _ := presets.FindFont("go-cuddlymenu")
	metrics, err := fontSpec.Build(chrome.Bounds())
	if err != nil {
		return nil, err
	}
	gap := 128.0
	if !repeat {
		// DCK stops a finite ribbon after Length+Gap pixels. Include the entry
		// distance so the final glyph reaches the left edge before it stops.
		gap = Width
	}
	s.scroll, err = scrolling.New(scrolling.Config{Text: ScrollText, Fonts: map[string]scrolling.Face{"chrome": {Atlas: chrome, Metrics: metrics}}, Font: "chrome", X: Width, Y: 310, Speed: scrollSpeed, Gap: gap, Repeat: repeat,
		Map: func(_ scrolling.Sample, op *ebiten.DrawImageOptions) bool {
			op.Filter = ebiten.FilterLinear
			return true
		}})
	if err != nil {
		return nil, err
	}
	s.owned = append(s.owned, s.scroll)
	s.layers, err = kit.NewLayers(Width, Height,
		kit.TimedLayer{Effect: s.rasters, Window: timeline.Window{Start: startup.RastersAt}},
		kit.TimedLayer{Effect: bobs, Window: timeline.Window{Start: startup.BobsAt}},
		kit.TimedLayer{Effect: s.cube},
		kit.TimedLayer{Effect: s.logo, Window: timeline.Window{Start: startup.LogoAt}},
	)
	if err != nil {
		return nil, err
	}
	s.owned = kit.Group{s.layers, s.scroll}
	return s, nil
}

// makeLogo adapts Leonard's row-wise phase associations to the wider DMA mark.
func (s *Scene) makeLogo(im *ebiten.Image) (*cachedWarp, error) {
	const width, height = 430, 182
	term := func(amplitude, rate, spacing float64) motion.IndexedHarmonic {
		return motion.IndexedHarmonic{Amplitude: amplitude, Rate: rate, IndexRate: spacing, PhasePeriod: 2 * math.Pi, RoundProduct: true, FusedIndexPhase: true}
	}
	global := 0
	rows, err := motion.NewHarmonicRowProfile(motion.HarmonicRowProfileConfig{Count: 17, ClockScale: [2]float64{85}, Stages: []motion.HarmonicRowStage{
		{Motion: motion.HarmonicFormationConfig{X: []motion.IndexedHarmonic{term(10, .0342, .2736)}}},
		{Motion: motion.HarmonicFormationConfig{X: []motion.IndexedHarmonic{term(10, .0143, .1144)}}},
		{Motion: motion.HarmonicFormationConfig{Y: []motion.IndexedHarmonic{term(13, .0342, 0)}}, Index: &global},
		{Motion: motion.HarmonicFormationConfig{Y: []motion.IndexedHarmonic{term(13, .0443, 0)}}, Index: &global},
	}})
	if err != nil {
		return nil, err
	}
	source := kit.Func{OnDraw: func(dst *ebiten.Image) {
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(width/float64(im.Bounds().Dx()), height/float64(im.Bounds().Dy()))
		dst.DrawImage(im, &op)
	}}
	warp, err := effects.NewWarp(source, width, height, 16, 16)
	if err != nil {
		return nil, err
	}
	warp.Map = func(x, y, t float64) geometry.Vec2 {
		age := max(0, t-startup.LogoAt)
		entrance := (1 - motion.Smooth(age/1.4)) * -240
		p, _ := rows.Apply(int(math.Round(y*16/height)), age, motion.Point{X: 105 + x + 30*math.Sin(age*.64), Y: 22 + y + entrance})
		return geometry.Vec2{X: p.X, Y: p.Y}
	}
	warp.Tint = func(x, y, t float64) color.Color {
		return color.NRGBA{255, 255, 255, uint8(255 * motion.Smooth((t-startup.LogoAt)/.7))}
	}
	return cacheWarp(warp, width, height, 16, 16), nil
}

// makeRasters reuses Bilizir's two-clock copper controller. Its expanded waves
// extend into the lower screen behind the chrome text and foreground prompt.
func (s *Scene) makeRasters(im *ebiten.Image) (*cachedWarp, error) {
	c := presets.BilizirCopperBars(im, rasterHeight, composite.CopperQuads, composite.MaskedClock)
	bars, err := composite.NewCopperBars(c)
	if err != nil {
		return nil, err
	}
	// Keep gaps between copper bars transparent; repeating the source strip
	// behind them adds a second, unrelated pattern under the warped rasters.
	warp, err := effects.NewWarp(bars, rasterWidth, rasterHeight, 32, 46)
	if err != nil {
		return nil, err
	}
	warp.Map = rasterProjection
	warp.Tint = func(x, y, t float64) color.Color {
		return color.NRGBA{255, 255, 255, uint8(98 * motion.Smooth((t-startup.RastersAt)/1.5))}
	}
	return cacheWarp(warp, rasterWidth, rasterHeight, 32, 46), nil
}

const rasterMargin = 64
const rasterWidth, rasterHeight = Width + 2*rasterMargin, Height + rasterMargin

// Overscan exceeds the horizontal wave's 12-pixel excursion. The bottom edge
// stays at least 114+1.15*464-48-24 = 575.6 pixels down, past the screen bottom.
func rasterProjection(x, y, t float64) geometry.Vec2 {
	return geometry.Vec2{X: x - rasterMargin + 12*math.Sin(y*.04+t*1.1), Y: 114 + y*1.15 + 48*math.Sin(x*.009+t*.35) + 24*math.Sin(y*.035-t*.5)}
}

// ballFormation retains Leonard's four oscillator ratios and phase spacing;
// amplitude and origin are adapted to the upper portion of this composition.
func ballFormation(group int) motion.HarmonicFormationConfig {
	term := func(a, r, index, phase float64) motion.IndexedHarmonic {
		return motion.IndexedHarmonic{Amplitude: a, Rate: r, IndexRate: index, Phase: phase, PhasePeriod: 2 * math.Pi}
	}
	phase := float64(group) * 2.1
	return motion.HarmonicFormationConfig{Origin: motion.Point{X: 320, Y: 218}, X: []motion.IndexedHarmonic{term(139, .0242, .1254, phase), term(139, .0193, .2530, -phase)}, Y: []motion.IndexedHarmonic{term(29, .0242, .1145, phase), term(29, .0203, .2230, phase*.7)}, Bounds: &motion.FormationBounds{Min: motion.Point{X: 16, Y: 150}, Max: motion.Point{X: 624, Y: 281}}}
}

// bobEntrance adds this production's staggered edge entrances to a DCK field.
// The embedded controller owns harmonic motion, sprite drawing and cleanup.
type bobEntrance struct {
	*sprites.HarmonicField
	group int
}

func (b *bobEntrance) Update(frame kit.Frame) error {
	if err := b.HarmonicField.Update(frame); err != nil {
		return err
	}
	startX := -32.0
	if b.group&1 != 0 {
		startX = Width + 32
	}
	for i := range b.Samples() {
		sample := &b.Samples()[i]
		age := startup.BobAge(frame.Time, b.group*20+i)
		sample.X = motion.Tween(startX, sample.X, 0, .7, age, motion.Smooth)
	}
	return nil
}

// Update samples DCK's timed layer tree once; drawing never advances its clocks.
func (s *Scene) Update(seconds float64) error {
	s.seconds = seconds
	s.dirty = true
	if err := s.layers.Update(kit.Frame{Time: seconds, Tick: uint64(seconds * startup.FPS)}); err != nil {
		return err
	}
	if seconds >= startup.ScrollAt {
		return s.scroll.Update(kit.Frame{Time: seconds - startup.ScrollAt, Tick: uint64(seconds * startup.FPS)})
	}
	return nil
}

// Draw preserves exact simulation-time sampling for captures and exports.
func (s *Scene) Draw(dst *ebiten.Image) { s.DrawAt(dst, s.seconds) }

// DrawLive advances only the foreground ribbon at the display cadence. Source
// demo controllers and their cached background keep their original 50 Hz clock.
func (s *Scene) DrawLive(dst *ebiten.Image, now time.Time) {
	s.DrawAt(dst, s.scrollClock.Sample(now, s.seconds))
}

// DrawAt samples DCK's ribbon without advancing its transport. DrawOffset keeps
// automatic culling and both repeated text copies alive across the loop seam.
func (s *Scene) DrawAt(dst *ebiten.Image, seconds float64) {
	if s.dirty {
		s.canvas.Fill(color.Black)
		s.layers.Draw(s.canvas)
		s.dirty = false
	}
	dst.DrawImage(s.canvas, nil)
	if s.seconds >= startup.ScrollAt {
		_ = s.scroll.DrawOffset(dst, -(seconds-s.seconds)*scrollSpeed, 0)
		s.printCentered(dst, "CREATIVITY CODING PLEASURE", 290, .5)
	}
}

// ScrollEnd is the time when the last glyph has passed the left screen edge.
func (s *Scene) ScrollEnd() float64 { return startup.ScrollAt + (Width+s.scroll.Length())/scrollSpeed }

func (s *Scene) printCentered(dst *ebiten.Image, text string, y, scale float64) {
	s.grey.Print(dst, text, (Width-float64(len(text))*16*scale)/2, y, scale, scale)
}

func (s *Scene) Close() {
	s.owned.Close()
	s.owned = nil
	for _, t := range s.textures {
		t.Deallocate()
	}
	s.textures = nil
	if s.canvas != nil {
		s.canvas.Deallocate()
		s.canvas = nil
	}
}
