package jelly

import (
	"github.com/olivierh59500/democonstructionkit/motion"
	"math"
)

const flockSide = 64
const flockCount = flockSide * flockSide
const flockCell, flockPadding = 32, 64
const flockColumns, flockRows = (Width + 2*flockPadding + flockCell - 1) / flockCell, (Height + 2*flockPadding + flockCell - 1) / flockCell
const flockNeighbors = 7

type bird struct{ x, y, z, vx, vy, vz, ax, ay, az, ux, uy, uz float64 }
type flock struct {
	birds   [flockCount]bird
	buckets [flockColumns * flockRows][8]int
	counts  [flockColumns * flockRows]int
	forces  [flockCount][3]float64
	tick    int
}

func (f *flock) reset() {
	f.tick = 0
	clear(f.forces[:])
	for i := range f.birds {
		x := (float64(i%flockSide) + .5) * Width / flockSide
		y := (float64(i/flockSide) + .5) * Height / flockSide
		angle := -.3 + .35*math.Sin(x*.008) + .25*math.Sin(y*.012) + (random(i+53)-.5)*.25
		speed := 55 + 20*random(i+71)
		uy, ux := math.Sincos(angle)
		uz, plane := math.Sincos((random(i+311)-.5)*.8 + .25*math.Sin(x*.01))
		ux *= plane
		uy *= plane
		f.birds[i] = bird{x: x, y: y, z: (random(i+312) - .5) * 240, vx: ux * speed, vy: uy * speed, vz: uz * speed, ux: ux, uy: uy, uz: uz}
	}
}
func targetPoint(i int) (float64, float64) {
	return Width/2 - 130 + (float64(i%flockSide)+.5)*260/flockSide, 10 + (float64(i/flockSide)+.5)*260/flockSide
}
func (f *flock) seek(tick int) {
	if tick < f.tick {
		f.reset()
	}
	for f.tick < tick {
		f.step()
	}
}
func flockBucket(b bird) (int, int) {
	return min(flockColumns-1, max(0, int(b.x+flockPadding)/flockCell)), min(flockRows-1, max(0, int(b.y+flockPadding)/flockCell))
}

// Every nearby cell contributes a small reservoir. Selecting the seven nearest
// perceived birds from all directions avoids a bias toward one grid edge.
func (f *flock) steering(i int, b bird, age float64) (float64, float64, float64) {
	cx, cy := flockBucket(b)
	var ids [flockNeighbors]int
	var distances [flockNeighbors]float64
	count, worst := 0, 0
	sx, sy, sz := 0.0, 0.0, 0.0
	speed2 := b.vx*b.vx + b.vy*b.vy + b.vz*b.vz
	for yy := max(0, cy-1); yy <= min(flockRows-1, cy+1); yy++ {
		for xx := max(0, cx-1); xx <= min(flockColumns-1, cx+1); xx++ {
			cell := yy*flockColumns + xx
			n := min(8, f.counts[cell])
			if n == 0 {
				continue
			}
			slot := (i + cell*3) % n
			for sample := 0; sample < min(4, n); sample++ {
				j := f.buckets[cell][slot]
				slot++
				if slot == n {
					slot = 0
				}
				if j == i {
					continue
				}
				other := f.birds[j]
				dx, dy := other.x-b.x, other.y-b.y
				dz := other.z - b.z
				d2 := dx*dx + dy*dy + dz*dz
				if d2 < .01 || d2 > 28*28 {
					continue
				}
				if d2 < 7*7 {
					sx -= dx / (d2 + 2)
					sy -= dy / (d2 + 2)
					sz -= dz / (d2 + 2)
				}
				dot := dx*b.vx + dy*b.vy + dz*b.vz
				if dot < 0 && dot*dot > speed2*d2*.5 {
					continue
				}
				if count < flockNeighbors {
					ids[count] = j
					distances[count] = d2
					count++
				} else {
					if d2 >= distances[worst] {
						continue
					}
					ids[worst] = j
					distances[worst] = d2
				}
				worst = 0
				for k := 1; k < count; k++ {
					if distances[k] > distances[worst] {
						worst = k
					}
				}
			}
		}
	}
	fx, fy, fz := sx*90, sy*90, sz*90
	if count > 0 {
		var vx, vy, vz, px, py, pz float64
		for k := 0; k < count; k++ {
			other := f.birds[ids[k]]
			vx += other.vx
			vy += other.vy
			vz += other.vz
			px += other.x
			py += other.y
			pz += other.z
		}
		n := float64(count)
		fx += (vx/n-b.vx)*1.25 + (px/n-b.x)*.12
		fy += (vy/n-b.vy)*1.25 + (py/n-b.y)*.12
		fz += (vz/n-b.vz)*1.25 + (pz/n-b.z)*.12
	}
	// Correlated wander changes slowly. It steers across the current heading,
	// while predictive containment leaves enough room for a gradual banked turn.
	wander := wanderAt(i, age) * 12
	fx += -b.uy * wander
	fy += b.ux * wander
	climb := wanderAt(i+413, age) * 16
	fx -= b.ux * b.uz * climb
	fy -= b.uy * b.uz * climb
	fz += (1 - b.uz*b.uz) * climb
	lookX, lookY, lookZ := (b.x+b.vx-320)/255, (b.y+b.vy-200)/155, (b.z+b.vz)/155
	outside := max(0, lookX*lookX+lookY*lookY+lookZ*lookZ-.8)
	wx, wy, wz := -lookX*outside*85, -lookY*outside*85, -lookZ*outside*85
	forward := wx*b.ux + wy*b.uy + wz*b.uz
	if forward < -10 {
		// Keep lift while banking back into the volume instead of stopping
		// and reversing at a wall. Depth provides room for crossing flocks.
		correction := -10 - forward
		wx += b.ux * correction
		wy += b.uy * correction
		wz += b.uz * correction
		wz += math.Copysign(min(40, -forward*.4), .1-b.z)
	}
	return fx + wx, fy + wy, fz + wz
}

func (f *flock) step() {
	clear(f.counts[:])
	for i, b := range f.birds {
		x, y := flockBucket(b)
		cell := y*flockColumns + x
		n := f.counts[cell]
		f.counts[cell]++
		slot := n
		if n >= 8 {
			slot = int(randomHash(uint32(i)*977+uint32(f.tick/8)*7919) % uint32(n+1))
		}
		if slot < 8 {
			f.buckets[cell][slot] = i
		}
	}
	age := float64(f.tick+1) / FPS
	gather := ease(FlockGatherAt-BoidsAt, PortraitAt-BoidsAt-.6, age)
	// Four staggered perception groups keep the costly nearest-neighbor query
	// bounded. Acceleration, heading and position still integrate at 50 Hz.
	for i := f.tick & 3; i < flockCount; i += 4 {
		b := f.birds[i]
		fx, fy, fz := f.steering(i, b, age)
		tx, ty := targetPoint(i)
		fx = fx*(1-gather) + ((tx-b.x)*8-b.vx*4)*gather
		fy = fy*(1-gather) + ((ty-b.y)*8-b.vy*4)*gather
		fz = fz*(1-gather) + (-b.z*8-b.vz*4)*gather
		limit := 75 + gather*400
		length2 := fx*fx + fy*fy + fz*fz
		if length2 > limit*limit {
			scale := limit / math.Sqrt(length2)
			fx *= scale
			fy *= scale
			fz *= scale
		}
		f.forces[i] = [3]float64{fx, fy, fz}
	}
	for i := range f.birds {
		b := &f.birds[i]
		b.ax += (f.forces[i][0] - b.ax) * .18
		b.ay += (f.forces[i][1] - b.ay) * .18
		b.az += (f.forces[i][2] - b.az) * .18
		b.vx += b.ax / FPS
		b.vy += b.ay / FPS
		b.vz += b.az / FPS
		speed := math.Sqrt(b.vx*b.vx + b.vy*b.vy + b.vz*b.vz)
		if speed > 1e-6 {
			b.ux, b.uy, b.uz = b.vx/speed, b.vy/speed, b.vz/speed
			cruise := 55 + 20*random(i+71)
			desired := speed + (cruise-speed)*.04*(1-gather)
			desired = min(95+gather*185, desired)
			b.vx = b.ux * desired
			b.vy = b.uy * desired
			b.vz = b.uz * desired
		}
		b.x = min(Width+flockPadding, max(-flockPadding, b.x+b.vx/FPS))
		b.y = min(Height+flockPadding, max(-flockPadding, b.y+b.vy/FPS))
		b.z = min(260, max(-260, b.z+b.vz/FPS))
	}
	f.tick++
}

func wanderAt(i int, age float64) float64 {
	time := age / 2
	whole := math.Floor(time)
	u := motion.Smooth(time - whole)
	a := random(i*37 + int(whole)*7919)
	b := random(i*37 + int(whole+1)*7919)
	return (a+(b-a)*u)*2 - 1
}
func randomHash(n uint32) uint32 {
	n ^= n >> 16
	n *= 0x7feb352d
	n ^= n >> 15
	n *= 0x846ca68b
	return n ^ (n >> 16)
}
func random(i int) float64               { return float64(randomHash(uint32(i))) / 4294967296 }
func ease(start, end, t float64) float64 { return motion.Smooth((t - start) / (end - start)) }
