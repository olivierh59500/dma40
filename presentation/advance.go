package presentation

import (
	"github.com/olivierh59500/dma40/internal/displayclock"
	"github.com/olivierh59500/dma40/internal/sequence"
	"github.com/olivierh59500/dma40/jelly"
)

// Advance shortens one scene without rebuilding its running part. Screen exits
// keep the ordinary fades and cards; intra-part jumps seek the same soundtrack.
func (g *Game) Advance() error {
	next, ok := g.nextCue()
	if !ok {
		return nil
	}
	target, err := g.plan.Start(next.Stage, next.Tick)
	if err != nil {
		return err
	}
	previousStage, previousMusic := g.cue.Stage, g.musicCue()
	if previousStage == sequence.Four && next.Stage == sequence.FourFade {
		// Shortening a hold must not simulate its skipped native frames.
		g.fourFadeBase = int(g.part.Tick())
	}
	for g.clock.Tick() < target {
		g.clock.Step()
	}
	if err := g.sample(next); err != nil {
		return err
	}
	currentMusic := g.musicCue()
	if previousMusic.name == currentMusic.name {
		if previousStage != next.Stage {
			// Keep the outgoing song continuous during a shortened screen exit.
			g.musicOffset += previousMusic.tick - currentMusic.tick
		} else {
			g.musicOffset = 0
			g.audio.audible = false
		}
	}
	g.paused = false
	g.visualClock = displayclock.Clock{}
	if err := g.syncAudio(); err != nil {
		return err
	}
	g.publish()
	return nil
}

func (g *Game) nextCue() (sequence.Cue, bool) {
	if g.cue.Stage == g.partID {
		if part, ok := g.part.(interface{ NextTick() (int, bool) }); ok {
			if tick, ok := part.NextTick(); ok {
				if g.cue.Stage == sequence.Jelly {
					tick -= jelly.EntryTick
				}
				return sequence.Cue{Stage: g.cue.Stage, Tick: tick}, true
			}
		}
	}
	return g.plan.Next(g.cue)
}
