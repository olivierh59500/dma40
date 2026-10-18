package opening

import (
	"image/color"

	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
)

// cachedWarp retains DCK's warp renderer and source effect. Its shared grid
// vertices and spatially uniform tint are sampled once per update, avoiding
// repeated trigonometry and boxed colors at every corner of every drawn cell.
type cachedWarp struct {
	*effects.Warp
	mapAt                        func(float64, float64, float64) geometry.Vec2
	tintAt                       func(float64, float64, float64) color.Color
	points                       []geometry.Vec2
	tint                         color.NRGBA
	width, height, columns, rows int
}

func cacheWarp(warp *effects.Warp, width, height, columns, rows int) *cachedWarp {
	p := &cachedWarp{Warp: warp, mapAt: warp.Map, tintAt: warp.Tint, width: width, height: height, columns: columns, rows: rows,
		points: make([]geometry.Vec2, (columns+1)*(rows+1))}
	warp.Map = func(x, y, _ float64) geometry.Vec2 {
		column := min(columns, max(0, int(x*float64(columns)/float64(width)+.5)))
		row := min(rows, max(0, int(y*float64(rows)/float64(height)+.5)))
		return p.points[row*(columns+1)+column]
	}
	warp.Tint = func(_, _, _ float64) color.Color { return &p.tint }
	return p
}

func (p *cachedWarp) Update(f kit.Frame) error {
	if err := p.Warp.Update(f); err != nil {
		return err
	}
	for row := 0; row <= p.rows; row++ {
		for column := 0; column <= p.columns; column++ {
			x, y := float64(column*p.width)/float64(p.columns), float64(row*p.height)/float64(p.rows)
			point := geometry.Vec2{X: x, Y: y}
			if p.mapAt != nil {
				point = p.mapAt(x, y, f.Time)
			}
			p.points[row*(p.columns+1)+column] = point
		}
	}
	p.tint = color.NRGBA{255, 255, 255, 255}
	if p.tintAt != nil {
		p.tint = color.NRGBAModel.Convert(p.tintAt(0, 0, f.Time)).(color.NRGBA)
	}
	return nil
}
