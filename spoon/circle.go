package spoon

import (
	"image"
	"image/color"
	"math"
	"sort"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/font"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/dma40/city"
	"github.com/olivierh59500/dma40/internal/mentalmedia"
)

const CircleText = "FRENCH FRIES ARE GOOD FOR YOU...    LOCKBUSTER WE MISS YOU...    "

const circleSourceAdvance, circleArc, circlePadding = 30.0, 190.0, 32.0
const circleSpeed = 60.0
const circleGlyphScale = .68
const circleAdvance = circleSourceAdvance * circleGlyphScale
const circleStart = math.Pi/2 + (256-circleArc)*math.Pi/256

// CircularFadeAt reserves an entire readable text pass, including the arc's
// trailing edge and the entrance fade. Editing the message changes the cue.
func CircularFadeAt(text string) float64 {
	pass := (float64(utf8.RuneCountInString(text))*circleAdvance + circleArc) / circleSpeed
	return math.Ceil(max(CircleAt+2+pass, CircleLensAt+12)*FPS) / FPS
}

func CityAt() float64   { return CircularFadeAt(CircleText) }
func FinaleAt() float64 { return CityAt() + city.Duration }

// circlePoint keeps the source font's baseline at row zero and its capitals at
// row ten. Increasing source X and decreasing row form an ordinary reading frame.
func circlePoint(distance, row, seconds float64) geometry.Vec3 {
	angle := circleStart + distance*2*math.Pi/256
	radius := 125 + row*(6.2+1.4*math.Sin(seconds*.57))
	return geometry.Vec3{X: radius * math.Cos(angle), Y: radius * math.Sin(angle)}
}

// Scale contours and their spacing without shrinking the circular baseline or
// changing the camera trajectory. The radial center stays at source row five.
func circleGlyphDistance(phase, slot, x float64) float64 {
	return phase + slot*circleAdvance + x*circleGlyphScale
}
func circleGlyphRow(row float64) float64 { return 5 + (row-5)*circleGlyphScale }

func circleCamera() geometry.Camera {
	return geometry.Camera{Center: geometry.Vec2{X: 320, Y: 195}, Focal: 730, Near: 40}
}
func circleRotation(seconds float64) geometry.Rotation {
	return geometry.RotateXYZ(geometry.Vec3{X: .30 + .28*math.Sin(seconds*.32), Y: .27 * math.Sin(seconds*.23), Z: .12 * math.Sin(seconds*.19)})
}

type circular struct {
	scroll   *scrolling.Scrolling
	seconds  float64
	order    [12]int
	letters  []rune
	base     int
	phase    float64
	rotation geometry.Rotation
	camera   geometry.Camera
	mask     *ebiten.Image
}

func newCircle(art mentalmedia.Circle) (*circular, error) {
	c := &circular{letters: []rune(CircleText), camera: circleCamera()}
	glyphs := make(map[rune]font.ContourGlyph, len(art.Glyphs))
	for ch, paths := range art.Glyphs {
		glyph := font.ContourGlyph{Advance: circleAdvance}
		for _, path := range paths {
			points := make([]geometry.Vec2, len(path))
			for i, p := range path {
				points[i] = geometry.Vec2{X: float64(p[0]), Y: float64(p[1])}
			}
			glyph.Contours = append(glyph.Contours, points)
		}
		glyphs[rune(ch)] = glyph
	}
	bank, err := font.NewContourBank(font.ContourBankConfig{Glyphs: glyphs, Closed: true})
	if err != nil {
		return nil, err
	}
	paint := color.NRGBA{R: 51, G: 170, B: 221, A: 255}
	// A one-row alpha material clips both ends before the angular coordinate can
	// wrap around the circle. The resulting lower gap has two fixed, sharp edges.
	c.mask = ebiten.NewImage(288, 1)
	c.mask.SubImage(image.Rect(int(circlePadding), 0, int(circlePadding+circleArc), 1)).(*ebiten.Image).Fill(color.White)
	c.scroll, err = scrolling.New(scrolling.Config{Shape: "circle", GlyphWindow: &scrolling.GlyphWindowConfig{Count: len(c.order), Advance: circleAdvance, Glyph: func(index int) scrolling.Glyph {
		slot := c.order[index]
		letter := ' '
		if position := c.base + slot; position >= 0 {
			letter = c.letters[position%len(c.letters)]
		}
		return scrolling.Glyph{Rune: letter, Font: "mental", Advance: circleAdvance, X: float64(slot)}
	}}, Modes: map[string]scrolling.Mode{"circle": {Contours: &scrolling.ContourPainterConfig{Fonts: map[string]*font.ContourBank{"mental": bank}, Font: "mental", Color: &paint, Texture: c.mask, Address: ebiten.AddressClampToZero, FillRule: ebiten.FillRuleEvenOdd, BatchTriangles: 4096,
		UV: func(s scrolling.ContourSample, _ geometry.Vec2) geometry.Vec2 {
			return geometry.Vec2{X: circlePadding + circleGlyphDistance(c.phase, s.Glyph.X, s.Point.X), Y: .5}
		},
		Map: func(s scrolling.ContourSample, _ ebiten.GeoM) (geometry.Vec2, bool) {
			p := circlePoint(circleGlyphDistance(c.phase, s.Glyph.X, s.Point.X), circleGlyphRow(s.Point.Y), c.seconds)
			p = c.rotation.Apply(p).Add(geometry.Vec3{Z: 840})
			xy, _, ok := c.camera.Project(p)
			return xy, ok
		}}}}})
	if err != nil {
		c.mask.Deallocate()
		return nil, err
	}
	return c, nil
}
func (c *circular) Update(f kit.Frame) error {
	c.seconds = f.Time
	// Split one quotient so the character index and its offset agree at cell
	// boundaries. Separate division and Mod can disagree for fractional advances.
	// The first glyph begins at the right cut. Negative indices are the empty
	// entrance arc, rather than wrapped letters from the end of the message.
	cells, fraction := math.Modf((f.Time*circleSpeed - circleArc) / circleAdvance)
	if fraction < 0 {
		cells--
		fraction++
	}
	c.base = int(cells)
	if c.base >= 0 {
		c.base %= len(c.letters)
	}
	c.phase = -fraction * circleAdvance
	c.rotation = circleRotation(f.Time)
	for i := range c.order {
		c.order[i] = i
	}
	sort.SliceStable(c.order[:], func(i, j int) bool {
		depth := func(slot int) float64 {
			return c.rotation.Apply(circlePoint(c.phase+float64(slot)*circleAdvance+circleAdvance/2, 5, c.seconds)).Z
		}
		return depth(c.order[i]) > depth(c.order[j])
	})
	if err := c.scroll.Update(f); err != nil {
		return err
	}
	return c.scroll.Err()
}
func (c *circular) Draw(dst *ebiten.Image) {
	state := scrolling.IdentityState()
	state.End = len(c.order)
	state.Time = c.seconds
	state.Shape = "circle"
	c.scroll.DrawAt(dst, state)
}
func (c *circular) Close() error { c.mask.Deallocate(); return c.scroll.Close() }
