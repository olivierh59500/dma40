package spoon

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/render"
)

// lensOptions samples an absolute-time trajectory. The logo finishes its fade
// before the lens grows into view, and pause or seeking never changes its phase.
func lensOptions(seconds float64) composite.MagnifierOptions {
	amount := smooth(CircleLensAt, CircleLensAt+1.25, seconds)
	if seconds >= CityAt() {
		amount = 0
	}
	if amount == 0 {
		return composite.MagnifierOptions{}
	}
	t := seconds - CircleLensAt
	return composite.MagnifierOptions{
		CenterX: 320 + 145*math.Sin(t*.63+.8) + 20*math.Sin(t*.19),
		CenterY: 195 + 95*math.Cos(t*.77+1.6),
		Radius:  76 * amount, Zoom: 1 + 1.6*amount, Falloff: .8, Opacity: amount, Feather: 1.25,
		Filter: ebiten.FilterLinear,
	}
}

// liveLens reuses DCK's analytic RGBA magnifier over one unmodified live frame.
// Only its small bounding box is shaded. Source and destination never alias.
type liveLens struct {
	magnifier *composite.Magnifier
	source    *ebiten.Image
	rim       *ebiten.Shader
	options   composite.MagnifierOptions
	geometry  [4]float32
	boxOrigin [2]float32
	uniforms  map[string]any
}

func newLiveLens() (_ *liveLens, err error) {
	l := &liveLens{}
	defer func() {
		if err != nil {
			l.Close()
		}
	}()
	l.magnifier, err = composite.NewMagnifier()
	if err != nil {
		return nil, err
	}
	l.rim, err = ebiten.NewShader([]byte(lensGlassShader))
	if err != nil {
		return nil, err
	}
	l.source = render.NewSurface(Width, Height)
	l.uniforms = map[string]any{"Geometry": l.geometry[:], "BoxOrigin": l.boxOrigin[:]}
	return l, nil
}
func (l *liveLens) Update(f kit.Frame) { l.options = lensOptions(f.Time) }
func (l *liveLens) Draw(dst *ebiten.Image, scene kit.Effect) {
	if l.options.Opacity == 0 {
		scene.Draw(dst)
		return
	}
	l.source.Fill(color.Black)
	scene.Draw(l.source)
	dst.DrawImage(l.source, nil)
	l.magnifier.Draw(dst, l.source, l.options)
	l.geometry = [4]float32{float32(l.options.CenterX), float32(l.options.CenterY), float32(l.options.Radius), float32(l.options.Opacity)}
	r := l.options.Radius + 3
	x, y := math.Max(0, math.Floor(l.options.CenterX-r)), math.Max(0, math.Floor(l.options.CenterY-r))
	w, h := math.Min(Width, math.Ceil(l.options.CenterX+r))-x, math.Min(Height, math.Ceil(l.options.CenterY+r))-y
	if w > 0 && h > 0 {
		l.boxOrigin = [2]float32{float32(x), float32(y)}
		op := ebiten.DrawRectShaderOptions{Uniforms: l.uniforms}
		op.GeoM.Translate(x, y)
		dst.DrawRectShader(int(w), int(h), l.rim, &op)
	}
}
func (l *liveLens) Close() {
	if l.magnifier != nil {
		l.magnifier.Close()
		l.magnifier = nil
	}
	if l.rim != nil {
		l.rim.Deallocate()
		l.rim = nil
	}
	if l.source != nil {
		l.source.Deallocate()
		l.source = nil
	}
}

// DCK owns image refraction. This separate Go/Kage pass adds a thin translucent
// rim and upper-left glass reflection without embedding artwork or a backdrop.
// Rectangle-local UVs keep the glass attached to the lens when an atlas or an
// export intermediate changes the destination's underlying framebuffer origin.
const lensGlassShader = `//kage:unit pixels
package main
var Geometry vec4
var BoxOrigin vec2
func Fragment(position vec4,uv vec2,color vec4)vec4{
 p:=(uv+BoxOrigin-Geometry.xy)/Geometry.z
 r:=length(p)
 if r>1.04{return vec4(0)}
 rim:=exp(-pow((r-.992)/.008,2))*.20
 light:=max(0,dot(p/max(r,.001),vec2(-.6,-.8)))
 crescent:=exp(-pow((r-.91)/.025,2))*pow(light,14)*.62
 glint:=exp(-pow((p.x+.31)/.12,2)-pow((p.y+.43)/.15,2))*.13
 if r>1{glint=0}
 alpha:=min(.8,(rim+crescent+glint)*Geometry.w)
 return vec4(vec3(.72,.87,1)*alpha,alpha)
}
`
