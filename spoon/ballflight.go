package spoon

import (
	"math"

	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/motion"
)

var ballTravel = motion.HarmonicTranslation{
	X: []motion.HarmonicTerm{{Amplitude: 210, Rate: 1.05}, {Amplitude: 75, Rate: .41}},
	Y: []motion.HarmonicTerm{{Amplitude: 150, Rate: 1.21}, {Amplitude: 65, Rate: .53, Phase: .4}},
}
var ballDepthRoll = motion.HarmonicTranslation{
	X: []motion.HarmonicTerm{{Amplitude: 580, Rate: 1.4}, {Amplitude: 140, Rate: .61}},
	Y: []motion.HarmonicTerm{{Amplitude: .2, Rate: .85}},
}
var ballHeading = motion.HarmonicTranslation{
	X: []motion.HarmonicTerm{{Amplitude: .38, Rate: .93}},
	Y: []motion.HarmonicTerm{{Amplitude: .6, Rate: .71}},
}

type ballFrame struct {
	position                geometry.Vec3
	rotation, spoonRotation geometry.Rotation
	spoonPosition           geometry.Vec3
	gather                  float64
}

func ballFrameAt(seconds float64) ballFrame {
	flightSeconds := seconds
	if seconds > RevealAt {
		// Integrate a smooth braking envelope while the spoon gathers. Once
		// stopped, depth recycling cannot pop a half-assembled source ball.
		u := min(1, seconds-RevealAt)
		flightSeconds = RevealAt + u - u*u*u + .5*u*u*u*u
	}
	travel, depth, heading := ballTravel.At(flightSeconds), ballDepthRoll.At(flightSeconds), ballHeading.At(flightSeconds)
	pose := spoonPose(seconds)
	return ballFrame{position: geometry.Vec3{X: travel.X, Y: travel.Y, Z: depth.X},
		rotation:      geometry.RotateXYZ(geometry.Vec3{X: heading.X, Y: heading.Y, Z: depth.Y}),
		spoonRotation: geometry.RotateXYZ(pose.Rotation), spoonPosition: pose.Position, gather: smooth(RevealAt, GatherAt, seconds)}
}

func (f ballFrame) point(index int, target geometry.Vec3) (geometry.Vec3, float64) {
	// Recycle the surrounding volume on all axes so lateral travel cannot
	// leave its finite source box behind. Edge fog hides each recycled copy.
	x := motion.Wrap(noise(index*5+1)*700-f.position.X, 700) - 350
	y := motion.Wrap(noise(index*5+2)*480-f.position.Y, 480) - 240
	z := 75 + motion.Wrap(noise(index*5+3)*1150-f.position.Z, 1150)
	free := f.rotation.Apply(geometry.Vec3{X: x, Y: y, Z: z})
	assembled := f.spoonRotation.Apply(target).Add(f.spoonPosition)
	fog := smooth(75, 155, z) * (1 - smooth(1060, 1225, z)) *
		(1 - smooth(305, 350, math.Abs(x))) * (1 - smooth(205, 240, math.Abs(y)))
	return geometry.Lerp(free, assembled, f.gather), fog + (1-fog)*math.Pow(f.gather, 6)
}
