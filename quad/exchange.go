package quad

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/dma40/internal/exchange"
)

const exchangeStart, exchangePeriod = 4.0, 34.0
const exchangeLeg, exchangeTravel = 6.0, 4.4
const exchangeDuration = 4 * exchangeLeg
const exchangeStagger = .55

// PresentationSeconds allows all six visitors to complete their first tour,
// then gives their native painters a short hold before the outgoing fade.
const PresentationSeconds = exchangeStart + 5*exchangeStagger + exchangeDuration + 1.25

type visitor struct {
	object              exchange.Object
	home, width, height int
	first, departure    float64
	active              bool
	legs                [4]motion.Curve
}

func newVisitor(object exchange.Object, home, width, height, ordinal int) visitor {
	v := visitor{object: object, home: home, width: width, height: height, first: exchangeStart + float64(ordinal)*exchangeStagger}
	regions := Regions()
	origin := motion.Point{X: float64(regions[home].Min.X), Y: float64(regions[home].Min.Y)}
	from := motion.Point{}
	// Horizontal partners exchange first, then both pairs cross vertically.
	// Each object visits all three neighbors and rejoins its live native pose.
	for leg, target := range [4]int{home ^ 1, home ^ 3, home ^ 2, home} {
		to := motion.Point{X: float64(regions[target].Min.X) - origin.X, Y: float64(regions[target].Min.Y) - origin.Y}
		dx, dy := to.X-from.X, to.Y-from.Y
		length := math.Hypot(dx, dy)
		normal := motion.Point{X: -dy / length, Y: dx / length}
		mid := motion.Point{X: origin.X + 160 + (from.X+to.X)/2, Y: origin.Y + 100 + (from.Y+to.Y)/2}
		if (320-mid.X)*normal.X+(200-mid.Y)*normal.Y < 0 {
			normal.X, normal.Y = -normal.X, -normal.Y
		}
		bend := 24 + float64(ordinal%3)*12
		v.legs[leg] = motion.BezierCurve(from,
			motion.Point{X: from.X + dx/3 + normal.X*bend, Y: from.Y + dy/3 + normal.Y*bend},
			motion.Point{X: from.X + dx*2/3 + normal.X*bend, Y: from.Y + dy*2/3 + normal.Y*bend}, to)
		from = to
	}
	return v
}

func (v *visitor) update(seconds float64) {
	v.active = false
	if seconds >= v.first {
		cycle := math.Floor((seconds - v.first) / exchangePeriod)
		v.departure = v.first + cycle*exchangePeriod
		v.active = seconds-v.departure < exchangeDuration
	}
	v.object.SetAway(v.active)
}

func (v *visitor) offset(seconds float64) motion.Point {
	local := max(0, min(exchangeDuration, seconds-v.departure))
	leg := min(3, int(local/exchangeLeg))
	u := motion.Smooth((local - float64(leg)*exchangeLeg) / exchangeTravel)
	return v.legs[leg](u)
}

func (v *visitor) draw(dst *ebiten.Image, seconds float64) {
	if !v.active {
		return
	}
	offset := v.offset(seconds)
	origin := Regions()[v.home].Min
	var transform ebiten.GeoM
	transform.Scale(320/float64(v.width), 200/float64(v.height))
	transform.Translate(float64(origin.X)+offset.X, float64(origin.Y)+offset.Y)
	v.object.Draw(dst, transform)
}
