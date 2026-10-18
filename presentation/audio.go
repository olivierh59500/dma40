package presentation

import (
	"errors"
	"io"
	"io/fs"
	"time"

	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/sound"
	playback "github.com/olivierh59500/democonstructionkit/sound/ebiten"
	"github.com/olivierh59500/democonstructionkit/sound/output"
	"github.com/olivierh59500/dma40/internal/sequence"
	"github.com/olivierh59500/dma40/internal/startup"
	"github.com/olivierh59500/dma40/jelly"
)

const musicVolume = .65

type soundtrack struct {
	player  *playback.Player
	key     string
	audible bool
}

// sync uses DCK's one output context for live playback and offline mixing.
// Muting advances the visual timeline; unmuting seeks the same musical cue.
func (s *soundtrack) sync(key, name string, data []byte, tick int, gain float64, silent, loop bool) error {
	created := s.player == nil || s.key != key
	if created {
		s.close()
		var err error
		s.player, err = playback.Open(output.CurrentContext(), name, data, sound.Options{PCMFormat: sound.PCM16, Loop: loop})
		if err != nil {
			return err
		}
		s.key = key
	}
	audible := !silent && gain > 0
	if (created || audible && !s.audible) && tick > 0 {
		err := s.player.Seek(time.Duration(tick) * time.Second / FPS)
		if errors.Is(err, io.EOF) {
			audible = false
		} else if err != nil {
			return err
		}
	}
	if err := s.player.SetVolume(musicVolume * max(0, min(1, gain))); err != nil {
		return err
	}
	if audible {
		s.player.Play()
	} else {
		s.player.Pause()
	}
	s.audible = audible
	return nil
}

func (s *soundtrack) close() {
	if s.player != nil {
		s.player.Close()
		s.player = nil
	}
	s.key = ""
	s.audible = false
}

type musicCue struct {
	name       string
	tick       int
	gain       float64
	loaderTick int
}

func (g *Game) musicCue() musicCue {
	c := musicCue{tick: g.cue.Tick, gain: 1, loaderTick: -1}
	switch g.cue.Stage {
	case sequence.Opening:
		c.name = "leonard.ym"
	case sequence.FourLoader:
		c.name = "leonard.ym"
		c.tick += g.plan.Length(sequence.Opening)
		c.gain = 1 - motion.Smooth(float64(g.cue.Tick)/startup.MusicFadeTicks)
		c.loaderTick = g.cue.Tick - startup.MusicFadeTicks
	case sequence.Four:
		c.name = "mega-apocalypse.ym"
	case sequence.FourFade:
		c.name = "mega-apocalypse.ym"
		c.tick += g.plan.Length(sequence.Four)
		c.gain = 1 - motion.Smooth(float64(g.cue.Tick)/sequence.FadeTicks)
	case sequence.SpoonLoader:
		c.name = "mega-apocalypse.ym"
		c.tick += g.plan.Length(sequence.Four) + sequence.FadeTicks
		c.gain = 0
		c.loaderTick = g.cue.Tick - startup.MusicFadeTicks
	case sequence.Spoon:
		c.name = "madness.mod"
		c.gain = 1 - motion.Smooth(float64(g.cue.Tick-g.plan.Length(sequence.Spoon)+FPS)/FPS)
	case sequence.Jelly:
		c.name = "tnt3.ym"
		ending := float64(g.cue.Tick+jelly.EntryTick-jelly.PiSettledTick()) / float64(jelly.PiHoldSeconds*FPS)
		c.gain *= 1 - motion.Smooth(ending)
	}
	c.tick += g.musicOffset
	return c
}

func (g *Game) track(name string) ([]byte, error) {
	if data, ok := g.tracks[name]; ok {
		return data, nil
	}
	var data []byte
	var err error
	switch name {
	case "leonard.ym":
		data = g.art.Music
	case "mega-apocalypse.ym":
		data = g.media.Music
	case "madness.mod":
		data, err = fs.ReadFile(g.media.Files, "spoon/music.mod")
	case "tnt3.ym":
		data, err = fs.ReadFile(g.media.Files, "jelly/music.ym")
	}
	if err == nil {
		g.tracks[name] = data
	}
	return data, err
}

func (g *Game) syncAudio() error {
	if g.options.DisableAudio {
		return nil
	}
	cue := g.musicCue()
	data, err := g.track(cue.name)
	if err != nil {
		return err
	}
	silent := g.muted || g.paused
	if err := g.audio.sync(cue.name, cue.name, data, cue.tick, cue.gain, silent, true); err != nil {
		return err
	}
	if cue.loaderTick >= 0 {
		return g.loaderAudio.sync(g.cue.Stage, "loader.ogg", g.art.LoaderSound, cue.loaderTick, 1, silent, false)
	}
	g.loaderAudio.close()
	return nil
}
