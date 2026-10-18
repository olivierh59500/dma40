package rastermaster

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

const jinxWidth, jinxHeight = 256, 30

type jinxPose struct {
	x           [jinxHeight]float64
	left, right float64
}

// jinxPose retains the native row wave without its screen-edge exit/reentry.
// A bank spans 640 pixels and its sixteen phases shift by one pixel each.
// Decode the signed row position, unwrap neighboring rows, then center the
// complete silhouette on the traveling object's own trajectory.
func (c *Clock) jinxPose(ink [jinxHeight]image.Rectangle) jinxPose {
	var pose jinxPose
	left, right := math.Inf(1), math.Inf(-1)
	for row := range jinxHeight {
		offset := c.Logo[c.Tick%1542+row]
		phase := max(0, min(15, (offset+160)/320))
		x := float64(phase - (offset-phase*320)/8*16)
		if row > 0 {
			for x-pose.x[row-1] > 320 {
				x -= 640
			}
			for x-pose.x[row-1] < -320 {
				x += 640
			}
		}
		pose.x[row] = x
		if !ink[row].Empty() {
			left = min(left, x+float64(ink[row].Min.X))
			right = max(right, x+float64(ink[row].Max.X))
		}
	}
	if math.IsInf(left, 1) {
		return pose
	}
	center := (left + right - Width) / 2
	for row := range pose.x {
		pose.x[row] -= center
	}
	pose.left, pose.right = left-center, right-center
	return pose
}

// fit keeps the complete wave inside the composition, including wide poses
// near its outer edges. Internal panel separators impose no clipping.
func (p jinxPose) fit(parent ebiten.GeoM, bounds image.Rectangle) ebiten.GeoM {
	left, top, right, bottom := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, corner := range [4][2]float64{{p.left, 0}, {p.right, 0}, {p.right, jinxHeight}, {p.left, jinxHeight}} {
		x, y := parent.Apply(corner[0], corner[1])
		left, right, top, bottom = min(left, x), max(right, x), min(top, y), max(bottom, y)
	}
	dx := max(0, float64(bounds.Min.X)-left) - max(0, right-float64(bounds.Max.X))
	dy := max(0, float64(bounds.Min.Y)-top) - max(0, bottom-float64(bounds.Max.Y))
	parent.Translate(dx, dy)
	return parent
}
