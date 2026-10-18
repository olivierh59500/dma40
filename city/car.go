package city

import (
	"image/color"
	"math"

	"github.com/olivierh59500/democonstructionkit/geometry"
)

const thrusterX, thrusterY, thrusterZ = .64, -.53, -2.66

type wheel struct {
	start, end int
	center     geometry.Vec3
	radius     float64
}

type carModel struct {
	builder
	wheels []wheel
	base   []geometry.Vec3
}

// newCar builds an authored DMC-12-style coupe. Its closed angular cabin,
// stainless panels, dark beltline and rear louvers remain actual 3D surfaces.
func newCar() *carModel {
	c := &carModel{}
	steel := color.NRGBA{R: 174, G: 183, B: 190, A: 255}
	bright := color.NRGBA{R: 215, G: 222, B: 225, A: 255}
	dark := color.NRGBA{R: 24, G: 29, B: 34, A: 255}
	glass := color.NRGBA{R: 40, G: 76, B: 91, A: 225}
	var previous [6]geometry.Vec3
	for station, spec := range [][3]float64{{-2.15, .94, -.76}, {-1.42, 1.01, -.86}, {-.55, 1.00, -.89}, {.63, .98, -.81}, {1.52, .94, -.70}, {2.16, .86, -.59}} {
		z, w, y := spec[0], spec[1], spec[2]
		ring := [6]geometry.Vec3{{X: -w, Y: -.33, Z: z}, {X: w, Y: -.33, Z: z}, {X: w, Y: y + .12, Z: z}, {X: w * .83, Y: y, Z: z}, {X: -w * .83, Y: y, Z: z}, {X: -w, Y: y + .12, Z: z}}
		if station > 0 {
			for i := range ring {
				paint := steel
				if i == 2 || i == 4 {
					paint = bright
				}
				if i == 0 {
					paint = dark
				}
				j := (i + 1) % len(ring)
				c.quad(previous[i], ring[i], ring[j], previous[j], paint)
			}
			for _, side := range []float64{-1, 1} {
				px, cx := side*(math.Abs(previous[0].X)+.008), side*(w+.008)
				c.quad(geometry.Vec3{X: px, Y: -.60, Z: previous[0].Z}, geometry.Vec3{X: cx, Y: -.60, Z: z}, geometry.Vec3{X: cx, Y: -.54, Z: z}, geometry.Vec3{X: px, Y: -.54, Z: previous[0].Z}, dark)
			}
		} else {
			c.quad(ring[0], ring[1], ring[2], ring[5], steel)
		}
		previous = ring
	}
	c.quad(previous[1], previous[0], previous[5], previous[2], steel)

	// The roof and raked glazing replace the former open roadster cockpit.
	c.quad(geometry.Vec3{X: -.72, Y: -1.26, Z: -.69}, geometry.Vec3{X: .72, Y: -1.26, Z: -.69}, geometry.Vec3{X: .68, Y: -1.24, Z: .40}, geometry.Vec3{X: -.68, Y: -1.24, Z: .40}, bright)
	c.quad(geometry.Vec3{X: -.84, Y: -.79, Z: 1.13}, geometry.Vec3{X: .84, Y: -.79, Z: 1.13}, geometry.Vec3{X: .68, Y: -1.24, Z: .40}, geometry.Vec3{X: -.68, Y: -1.24, Z: .40}, dark)
	c.quad(geometry.Vec3{X: -.775, Y: -.82, Z: 1.10}, geometry.Vec3{X: .775, Y: -.82, Z: 1.10}, geometry.Vec3{X: .63, Y: -1.205, Z: .43}, geometry.Vec3{X: -.63, Y: -1.205, Z: .43}, glass)
	for _, side := range []float64{-1, 1} {
		c.quad(geometry.Vec3{X: side * .91, Y: -.85, Z: -1.34}, geometry.Vec3{X: side * .72, Y: -1.26, Z: -.69}, geometry.Vec3{X: side * .68, Y: -1.24, Z: .40}, geometry.Vec3{X: side * .84, Y: -.79, Z: 1.13}, steel)
		c.quad(geometry.Vec3{X: side * .888, Y: -.88, Z: -1.19}, geometry.Vec3{X: side * .745, Y: -1.20, Z: -.63}, geometry.Vec3{X: side * .708, Y: -1.185, Z: .36}, geometry.Vec3{X: side * .84, Y: -.84, Z: .99}, glass)
		// Closed gull-wing door outlines and recessed handles.
		for _, z := range []float64{-1.10, .72} {
			c.quad(geometry.Vec3{X: side * 1.012, Y: -.79, Z: z}, geometry.Vec3{X: side * 1.012, Y: -.40, Z: z}, geometry.Vec3{X: side * 1.012, Y: -.40, Z: z + .014}, geometry.Vec3{X: side * 1.012, Y: -.79, Z: z + .014}, dark)
		}
		c.box(geometry.Vec3{X: side * 1.015, Y: -.71, Z: -.84}, geometry.Vec3{X: .015, Y: .035, Z: .16}, dark)
	}
	c.quad(geometry.Vec3{X: -.009, Y: -1.269, Z: -.67}, geometry.Vec3{X: .009, Y: -1.269, Z: -.67}, geometry.Vec3{X: .009, Y: -1.249, Z: .38}, geometry.Vec3{X: -.009, Y: -1.249, Z: .38}, dark)
	c.box(geometry.Vec3{Y: -.88, Z: -.08}, geometry.Vec3{X: 1.38, Y: .12, Z: 1.12}, dark)
	c.box(geometry.Vec3{X: -.37, Y: -.99, Z: .04}, geometry.Vec3{X: .30, Y: .31, Z: .25}, dark)
	c.box(geometry.Vec3{X: -.37, Y: -1.12, Z: .07}, geometry.Vec3{X: .22, Y: .18, Z: .22}, color.NRGBA{R: 229, G: 169, B: 127, A: 255})

	// Rear-window louvers are separate raised slats across the sloping deck.
	deck := func(z, x float64) geometry.Vec3 {
		u := (z + .72) / (-1.81 + .72)
		return geometry.Vec3{X: x * (.74 + .16*u), Y: -1.245 + .45*u, Z: z}
	}
	c.quad(deck(-.72, -1), deck(-.72, 1), deck(-1.81, 1), deck(-1.81, -1), dark)
	for i := 0; i < 7; i++ {
		z := -.80 - float64(i)*.142
		a, b, d, e := deck(z, -1), deck(z, 1), deck(z-.085, 1), deck(z-.085, -1)
		a.Y -= .025
		b.Y -= .025
		d.Y -= .025
		e.Y -= .025
		c.quad(a, b, d, e, color.NRGBA{R: 36, G: 43, B: 49, A: 255})
		c.quad(a, b, b.Add(geometry.Vec3{Y: .018}), a.Add(geometry.Vec3{Y: .018}), color.NRGBA{R: 119, G: 132, B: 141, A: 255})
	}

	// Four rectangular headlights and segmented tail lamps sit in dark fascias.
	c.box(geometry.Vec3{Y: -.56, Z: 2.17}, geometry.Vec3{X: 1.86, Y: .27, Z: .08}, dark)
	c.box(geometry.Vec3{Y: -.35, Z: 2.14}, geometry.Vec3{X: 2.00, Y: .19, Z: .20}, dark)
	for _, x := range []float64{-.70, -.39, .39, .70} {
		c.quad(geometry.Vec3{X: x - .125, Y: -.65, Z: 2.215}, geometry.Vec3{X: x + .125, Y: -.65, Z: 2.215}, geometry.Vec3{X: x + .125, Y: -.49, Z: 2.215}, geometry.Vec3{X: x - .125, Y: -.49, Z: 2.215}, color.NRGBA{R: 239, G: 239, B: 213, A: 255})
	}
	c.box(geometry.Vec3{Y: -.63, Z: -2.17}, geometry.Vec3{X: 1.88, Y: .29, Z: .08}, dark)
	c.box(geometry.Vec3{Y: -.35, Z: -2.13}, geometry.Vec3{X: 2.04, Y: .20, Z: .20}, dark)
	for _, side := range []float64{-1, 1} {
		for lamp, paint := range []color.NRGBA{{R: 231, G: 45, B: 40, A: 255}, {R: 244, G: 134, B: 38, A: 255}, {R: 221, G: 220, B: 199, A: 255}} {
			x := side * (.77 - float64(lamp)*.205)
			c.quad(geometry.Vec3{X: x - .09, Y: -.72, Z: -2.215}, geometry.Vec3{X: x + .09, Y: -.72, Z: -2.215}, geometry.Vec3{X: x + .09, Y: -.54, Z: -2.215}, geometry.Vec3{X: x - .09, Y: -.54, Z: -2.215}, paint)
		}
		c.box(geometry.Vec3{X: side * thrusterX, Y: thrusterY, Z: -2.43}, geometry.Vec3{X: .28, Y: .27, Z: .46}, color.NRGBA{R: 73, G: 84, B: 94, A: 255})
	}
	for _, x := range []float64{-.89, .89} {
		c.addWheel(geometry.Vec3{X: x, Y: -.36, Z: -1.30}, .36)
		c.addWheel(geometry.Vec3{X: x, Y: -.335, Z: 1.34}, .335)
	}
	c.base = append([]geometry.Vec3(nil), c.Points...)
	return c
}

func (c *carModel) addWheel(center geometry.Vec3, radius float64) {
	start := len(c.Points)
	for i := 0; i < 12; i++ {
		a, d := float64(i)*math.Pi/6, float64(i+1)*math.Pi/6
		point := func(side, angle, r float64) geometry.Vec3 {
			return center.Add(geometry.Vec3{X: side, Y: r * math.Cos(angle), Z: r * math.Sin(angle)})
		}
		c.quad(point(-.13, a, radius), point(.13, a, radius), point(.13, d, radius), point(-.13, d, radius), color.NRGBA{R: 16, G: 20, B: 24, A: 255})
		for _, side := range []float64{-.135, .135} {
			paint := color.NRGBA{R: 107, G: 121, B: 132, A: 255}
			if i%2 == 0 {
				paint = color.NRGBA{R: 203, G: 214, B: 219, A: 255}
			}
			c.quad(point(side, a, .07), point(side, a, radius*.81), point(side, d, radius*.81), point(side, d, .07), paint)
		}
	}
	c.wheels = append(c.wheels, wheel{start: start, end: len(c.Points), center: center, radius: radius})
}

func (c *carModel) roll(distance float64) {
	for _, wheel := range c.wheels {
		rotation := geometry.RotateXYZ(geometry.Vec3{X: distance / wheel.radius})
		for i := wheel.start; i < wheel.end; i++ {
			c.Points[i] = rotation.Apply(c.base[i].Sub(wheel.center)).Add(wheel.center)
		}
	}
}
