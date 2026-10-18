// Package sequence schedules the uninterrupted DMA40 presentation through DCK.
package sequence

import (
	"fmt"
	"time"

	"github.com/olivierh59500/democonstructionkit/timeline"
	"github.com/olivierh59500/dma40/internal/startup"
)

const (
	Opening     = "opening"
	FourLoader  = "four-loader"
	Four        = "four"
	FourFade    = "four-fade"
	SpoonLoader = "spoon-loader"
	Spoon       = "spoon"
	Jelly       = "jelly"
)

const FadeTicks = 2 * startup.FPS

var stages = [...]string{Opening, FourLoader, Four, FourFade, SpoonLoader, Spoon, Jelly}

type Cue struct {
	Stage string
	Tick  int
}

type Plan struct {
	sequence *timeline.Sequence
	starts   [7]int
	lengths  [6]int
}

func New(openingTicks, fourTicks, spoonTicks int) (*Plan, error) {
	p := &Plan{lengths: [6]int{openingTicks, startup.TransitionTicks, fourTicks, FadeTicks, startup.TransitionTicks, spoonTicks}}
	durations := make([]time.Duration, len(p.lengths))
	for i, ticks := range p.lengths {
		if ticks <= 0 || ticks > 3600*startup.FPS {
			return nil, fmt.Errorf("sequence: invalid duration for %s", stages[i])
		}
		p.starts[i+1] = p.starts[i] + ticks
		durations[i] = time.Duration(ticks) * time.Second / startup.FPS
	}
	var err error
	p.sequence, err = timeline.NewSequence(durations, false)
	return p, err
}

// At uses half-open cues; the final part owns its completion signal.
func (p *Plan) At(tick int) Cue {
	tick = max(0, tick)
	index, local, ok := p.sequence.At(time.Duration(tick) * time.Second / startup.FPS)
	if !ok {
		return Cue{Stage: Jelly, Tick: tick - p.starts[6]}
	}
	return Cue{Stage: stages[index], Tick: int(local * startup.FPS / time.Second)}
}

func (p *Plan) Start(stage string, tick int) (int, error) {
	if tick < 0 || tick > 3600*startup.FPS {
		return 0, fmt.Errorf("sequence: invalid preview time")
	}
	for i, name := range stages {
		if name != stage {
			continue
		}
		if i < len(p.lengths) && tick >= p.lengths[i] {
			return 0, fmt.Errorf("sequence: preview exceeds %s", stage)
		}
		return p.starts[i] + tick, nil
	}
	return 0, fmt.Errorf("sequence: unknown part %q", stage)
}

func (p *Plan) Length(stage string) int {
	for i, name := range stages[:len(p.lengths)] {
		if name == stage {
			return p.lengths[i]
		}
	}
	return 0
}

func (p *Plan) JellyStart() int { return p.starts[6] }

// Next preserves the ordinary order when a viewer shortens a screen's hold.
func (p *Plan) Next(cue Cue) (Cue, bool) {
	for index, stage := range stages {
		if cue.Stage == stage && index+1 < len(stages) {
			return Cue{Stage: stages[index+1]}, true
		}
	}
	return Cue{}, false
}
