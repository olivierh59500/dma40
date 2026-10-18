package spoon

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/dma40/internal/displayclock"
)

const voxelWidth, voxelHeight, voxelSamples = 320, 200, 128

// voxelFlight adapts yosshin's volumetric lattice into an authored fly-through.
// DCK owns the retained low-resolution surface, scaling and timed composition.
// Reference: https://x.com/yosshin4004/status/1470413906741198848
type voxelFlight struct {
	volume *voxelVolume
	pass   *composite.Pass
}

type voxelVolume struct {
	shader                     *ebiten.Shader
	options                    ebiten.DrawRectShaderOptions
	origin, right, up, forward [3]float32
	seconds, liveSeconds       float64
	live                       bool
	clock                      displayclock.Clock
}

func newVoxelFlight() (*voxelFlight, error) {
	shader, err := ebiten.NewShader([]byte(voxelShader))
	if err != nil {
		return nil, err
	}
	v := &voxelVolume{shader: shader}
	v.options.Uniforms = map[string]any{"Time": float32(0), "Origin": v.origin[:], "Right": v.right[:], "Up": v.up[:], "Forward": v.forward[:]}
	pass, err := composite.Layer(v, voxelWidth, voxelHeight, func(kit.Frame) ebiten.DrawImageOptions {
		var options ebiten.DrawImageOptions
		options.Filter = ebiten.FilterLinear
		options.GeoM.Scale(Width/float64(voxelWidth), Height/float64(voxelHeight))
		return options
	})
	if err != nil {
		shader.Deallocate()
		return nil, err
	}
	return &voxelFlight{volume: v, pass: pass}, nil
}

func (v *voxelFlight) Update(f kit.Frame) error { return v.pass.Update(f) }
func (v *voxelFlight) Draw(dst *ebiten.Image)   { v.pass.Draw(dst) }
func (v *voxelFlight) Close() error             { return v.pass.Close() }

func (v *voxelFlight) liveAt(now time.Time, paused bool) {
	v.volume.live = true
	if paused {
		v.volume.liveSeconds = v.volume.clock.Hold(now, v.volume.seconds)
	} else {
		v.volume.liveSeconds = v.volume.clock.Sample(now, v.volume.seconds)
	}
}

func (v *voxelVolume) Update(f kit.Frame) error { v.seconds = f.Time; return nil }
func (v *voxelVolume) Close() error             { v.shader.Deallocate(); return nil }

func (v *voxelVolume) Draw(dst *ebiten.Image) {
	seconds := v.seconds
	if v.live {
		seconds = v.liveSeconds
	}
	t := (seconds + 7) * .65
	v.options.Uniforms["Time"] = float32(t)
	v.origin = [3]float32{0, float32(99 * math.Sin(t)), float32(-495 * t)}
	// Normalize the HSV-derived axis before applying Rodrigues' rotation.
	// The reference's sample-dependent HSV value cancels during normalization.
	component := func(offset float64) float64 {
		h := t*.1 + offset
		return max(0, min(1, math.Abs((h-math.Floor(h))*6-3)-1))
	}
	axis := geometry.Vec3{X: component(0), Y: component(2.0 / 3), Z: component(1.0 / 3)}
	axis = axis.Scale(1 / math.Sqrt(axis.Dot(axis)))
	rotate := func(p geometry.Vec3) geometry.Vec3 {
		return p.Scale(math.Cos(1)).Add(axis.Cross(p).Scale(math.Sin(1))).Add(axis.Scale(axis.Dot(p) * (1 - math.Cos(1))))
	}
	set := func(dst *[3]float32, p geometry.Vec3) {
		dst[0], dst[1], dst[2] = float32(p.X), float32(p.Y), float32(p.Z)
	}
	set(&v.right, rotate(geometry.Vec3{X: 1}))
	set(&v.up, rotate(geometry.Vec3{Y: 1}))
	set(&v.forward, rotate(geometry.Vec3{Z: 1}))
	dst.DrawRectShader(voxelWidth, voxelHeight, v.shader, &v.options)
}

// Fixed stratification preserves depth coverage without the reference's 1,000
// samples per fragment. Multiplication replaces three powers inside the loop.
const voxelShader = `//kage:unit pixels
package main
var Time float
var Origin, Right, Up, Forward vec3
func Fragment(position vec4, source vec2, color vec4) vec4 {
 uv:=(position.xy-vec2(160,100))/200
 ray:=normalize(vec3(uv.x,-uv.y,-.85))
 direction:=Right*ray.x+Up*ray.y+Forward*ray.z
 jitter:=fract(dot(position.xy,vec2(.754877666,.569840296)))*.6
 sum:=vec3(0)
 for step:=0;step<128;step++ {
  distance:=(float(step)+.5+jitter)*(1000.0/128.0)
  p:=Origin+direction*distance
  surface:=min(p.y,length(p.xy)*sign(cos(p.z/3000)))
  band:=abs((fract(surface/200)-.5)*p.y)*5+1
  ux,uz:=int(p.x),int(p.z)
  pattern:=((uz-ux)^(uz+ux))&(uz/999)
  q:=fract(float(pattern)/83-Time*.01)
  q2:=q*q
  q4:=q2*q2
  q8:=q4*q4
  q16:=q8*q8
  q32:=q16*q16
  green:=q32*q16*q2
  blue:=green*green
  red:=blue*green
  sum+=vec3(red,green,blue)/band*(1000.0/128.0)
 }
 mapped:=sum/(vec3(1)+sum)
 mapped=pow(mapped,vec3(.6))*(1-.2*dot(uv,uv))
 return vec4(clamp(mapped,vec3(0),vec3(1)),1)
}
`
