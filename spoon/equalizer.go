package spoon

import (
	"encoding/binary"
	"image/color"
	"io"
	"io/fs"
	"math"
	"math/bits"
	"math/cmplx"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/modulation"
	"github.com/olivierh59500/democonstructionkit/sound"
)

const meterColumns, meterHeight = 80, 72
const meterRate, meterWindow = 16000, 1024
const meterFrames = meterRate / FPS

// pcmSpectrum maps stereo PCM to logarithmic bands without cancelling opposing
// left/right phases. FFT storage, the Hann window and band limits are reused.
type pcmSpectrum struct {
	history [2][meterWindow]float64
	window  [meterWindow]float64
	phase   [meterWindow / 2]complex128
	work    [meterWindow]complex128
	power   [meterWindow / 2]float64
	ranges  [meterColumns][2]int
	charges [meterColumns]modulation.PeakCharge
	peaks   *modulation.PeakBank
	cursor  int
}

func newPCMSpectrum() (*pcmSpectrum, error) {
	s := &pcmSpectrum{}
	var err error
	s.peaks, err = modulation.NewPeakBank(modulation.PeakBankConfig{Count: meterColumns, Ceiling: meterHeight, Decay: 3})
	if err != nil {
		return nil, err
	}
	for i := range s.window {
		s.window[i] = .5 - .5*math.Cos(2*math.Pi*float64(i)/(meterWindow-1))
	}
	for i := range s.phase {
		s.phase[i] = cmplx.Rect(1, -2*math.Pi*float64(i)/meterWindow)
	}
	for i := range s.ranges {
		low := 60 * math.Pow(125, float64(i)/meterColumns)
		high := 60 * math.Pow(125, float64(i+1)/meterColumns)
		s.ranges[i] = [2]int{max(1, int(math.Floor(low*meterWindow/meterRate))), min(meterWindow/2, int(math.Ceil(high*meterWindow/meterRate)))}
	}
	return s, nil
}

func (s *pcmSpectrum) reset() {
	clear(s.history[0][:])
	clear(s.history[1][:])
	s.cursor = 0
	s.peaks.Reset()
}

func (s *pcmSpectrum) step(pcm []byte) error {
	for frame := 0; frame+7 < len(pcm); frame += 8 {
		for channel := 0; channel < 2; channel++ {
			value := math.Float32frombits(binary.LittleEndian.Uint32(pcm[frame+channel*4:]))
			s.history[channel][s.cursor] = float64(value)
		}
		s.cursor = (s.cursor + 1) % meterWindow
	}
	clear(s.power[:])
	for channel := 0; channel < 2; channel++ {
		for i := range s.work {
			reversed := bits.Reverse16(uint16(i)) >> 6
			s.work[reversed] = complex(s.history[channel][(s.cursor+i)%meterWindow]*s.window[i], 0)
		}
		// An in-place radix-two transform leaves all frequency bins available
		// for the reusable DCK peak bank. No audio device is read or modified.
		for size := 2; size <= meterWindow; size *= 2 {
			for start := 0; start < meterWindow; start += size {
				for i := 0; i < size/2; i++ {
					a := s.work[start+i]
					b := s.work[start+i+size/2] * s.phase[i*meterWindow/size]
					s.work[start+i], s.work[start+i+size/2] = a+b, a-b
				}
			}
		}
		for bin := range s.power {
			re, im := real(s.work[bin]), imag(s.work[bin])
			s.power[bin] += re*re + im*im
		}
	}
	for column, span := range s.ranges {
		peak := 0.0
		for bin := span[0]; bin < span[1]; bin++ {
			peak = max(peak, s.power[bin])
		}
		amplitude := math.Sqrt(peak/2) * 4 / meterWindow
		level := max(0, min(1, (20*math.Log10(max(amplitude, 1e-8))+64)/56))
		s.charges[column] = modulation.PeakCharge{Index: column, Level: meterHeight * level}
	}
	return s.peaks.Step(s.charges[:], 1)
}

// finaleEqualizer shares Leonard's eighty-column red/yellow presentation.
// A silent DCK decoder follows the part clock; the audible MOD player remains
// untouched. Advancing during earlier effects avoids a long seek at the finale.
type finaleEqualizer struct {
	stream   *sound.Stream
	spectrum *pcmSpectrum
	bars     *composite.GradientBars
	pcm      [meterFrames * 8]byte
	tick     uint64
}

func newFinaleEqualizer(files fs.FS) (_ *finaleEqualizer, err error) {
	m := &finaleEqualizer{}
	defer func() {
		if err != nil {
			m.Close()
		}
	}()
	m.spectrum, err = newPCMSpectrum()
	if err != nil {
		return nil, err
	}
	m.stream, err = sound.OpenFS(files, "music.mod", sound.Options{SampleRate: meterRate, BlockFrames: meterFrames, Loop: true})
	if err != nil {
		return nil, err
	}
	m.bars, err = composite.NewGradientBars(composite.GradientBarsConfig{
		Columns: meterColumns, Baseline: Height - 1, Step: 8, Width: 7,
		Level: m.spectrum.peaks.Level,
		Top:   color.NRGBA{R: 255, G: 255, A: 255}, Bottom: color.NRGBA{R: 255, A: 255},
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *finaleEqualizer) update(tick uint64) error {
	if tick < m.tick || tick-m.tick > FPS {
		// One second restores both the FFT history and the decaying peaks.
		start := max(int64(0), int64(tick)-FPS)
		if _, err := m.stream.Seek(start*meterFrames*8, io.SeekStart); err != nil {
			return err
		}
		m.spectrum.reset()
		m.tick = uint64(start)
	}
	for m.tick < tick {
		if _, err := io.ReadFull(m.stream, m.pcm[:]); err != nil {
			return err
		}
		if err := m.spectrum.step(m.pcm[:]); err != nil {
			return err
		}
		m.tick++
	}
	return nil
}
func (m *finaleEqualizer) Draw(dst *ebiten.Image) { m.bars.Draw(dst) }
func (m *finaleEqualizer) Close() {
	if m.bars != nil {
		m.bars.Close()
	}
	if m.stream != nil {
		m.stream.Close()
	}
}
