package jelly

import (
	"math"

	"github.com/olivierh59500/democonstructionkit/geometry"
)

func logoWobble() geometry.WobbleField {
	return geometry.WobbleField{
		Key: geometry.Vec3{X: .009, Y: .016, Z: .025}, BaseInfluence: 1,
		X: []geometry.Harmonic{{Gain: .7, Speed: 5.4, Spatial: 1.8}, {Gain: .3, Speed: 8.1, Spatial: 2.1, Phase: 1}},
		Y: []geometry.Harmonic{{Gain: .35, Speed: 6.5, Spatial: 1.1}},
		Z: []geometry.Harmonic{{Gain: 1.8, Speed: 4.2, Spatial: 1.3, Phase: .7}},
	}
}

// The logo's source points never move. Cache the spatial half of its DCK
// harmonic recipe and evaluate sin(a+b) with four temporal samples per frame.
// The shared enclosure still uses DCK's general, position-dependent program.
type logoWaves struct {
	waves [4]geometry.Harmonic
	basis [][4][2]float64
}

func newLogoWaves(points []geometry.Vec3) logoWaves {
	field := logoWobble()
	p := logoWaves{waves: [4]geometry.Harmonic{field.X[0], field.X[1], field.Y[0], field.Z[0]}, basis: make([][4][2]float64, len(points))}
	for i, point := range points {
		key := point.Dot(field.Key)
		for j, wave := range p.waves {
			s, c := math.Sincos(key*wave.Spatial + wave.Phase)
			p.basis[i][j] = [2]float64{s * wave.Gain, c * wave.Gain}
		}
	}
	return p
}

func (p *logoWaves) sample(dst []geometry.Vec3, dx []float64, source []geometry.Vec3, seconds, amount float64) {
	var temporal [4][2]float64
	for j, wave := range p.waves {
		s, c := math.Sincos(seconds * wave.Speed)
		temporal[j] = [2]float64{s * amount, c * amount}
	}
	for i, point := range source {
		var value [4]float64
		for j, basis := range p.basis[i] {
			value[j] = basis[0]*temporal[j][1] + basis[1]*temporal[j][0]
		}
		dx[i] = value[0] + value[1]
		dst[i] = geometry.Vec3{X: point.X, Y: point.Y + value[2], Z: point.Z + value[3]}
	}
}
