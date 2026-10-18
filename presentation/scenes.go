package presentation

import (
	"image"
	"io/fs"

	"github.com/olivierh59500/dma40/internal/portraitassets"
	"github.com/olivierh59500/dma40/internal/sequence"
	"github.com/olivierh59500/dma40/jelly"
	"github.com/olivierh59500/dma40/opening"
	"github.com/olivierh59500/dma40/quad"
	"github.com/olivierh59500/dma40/spoon"
)

func (g *Game) closeScenes(keepOpening bool) {
	if !keepOpening && g.opening != nil {
		g.opening.Close()
		g.opening = nil
	}
	if g.loader != nil {
		g.loader.Close()
		g.loader = nil
	}
	if g.part != nil {
		g.part.Close()
		g.part = nil
	}
	g.partID = ""
}

func (g *Game) ensurePart(id string) error {
	if g.part != nil && g.partID == id {
		return nil
	}
	if g.part != nil {
		g.part.Close()
		g.part = nil
	}
	var err error
	switch id {
	case sequence.Four:
		g.part, err = quad.New(g.media)
	case sequence.Spoon:
		var files fs.FS
		files, err = fs.Sub(g.media.Files, "spoon")
		if err == nil {
			g.part, err = spoon.NewTimed(files, g.art.FontGrey, g.art.Logo, g.nonameSeconds)
		}
	case sequence.Jelly:
		var files fs.FS
		files, err = fs.Sub(g.media.Files, "jelly")
		if err == nil {
			var portrait image.Image
			portrait, err = portraitassets.Load()
			if err == nil {
				g.part, err = jelly.New(files, g.art.Logo, portrait, g.art.FontGrey)
			}
		}
	}
	if err != nil {
		g.part = nil
		g.partID = ""
		return err
	}
	g.partID = id
	return nil
}

func (g *Game) sample(cue sequence.Cue) error {
	previousMusic := g.musicCue().name
	if cue.Stage != g.cue.Stage {
		g.advanceTap.Reset()
		if cue.Stage != sequence.Opening && g.opening != nil {
			g.opening.Close()
			g.opening = nil
		}
		if g.loader != nil {
			g.loader.Close()
			g.loader = nil
		}
		switch cue.Stage {
		case sequence.Opening:
			if g.opening == nil {
				var err error
				g.opening, err = opening.NewSinglePass(g.art)
				if err != nil {
					return err
				}
			}
		case sequence.FourLoader, sequence.SpoonLoader:
			id, title := sequence.Four, quad.Title
			if cue.Stage == sequence.SpoonLoader {
				id, title = sequence.Spoon, spoon.Title
			}
			if err := g.ensurePart(id); err != nil {
				return err
			}
			var err error
			g.loader, err = opening.NewLoader(g.art.UnionFont, title)
			if err != nil {
				return err
			}
		case sequence.Four, sequence.FourFade:
			if err := g.ensurePart(sequence.Four); err != nil {
				return err
			}
		case sequence.Spoon:
			if err := g.ensurePart(sequence.Spoon); err != nil {
				return err
			}
		case sequence.Jelly:
			if err := g.ensurePart(sequence.Jelly); err != nil {
				return err
			}
		}
	}
	g.cue = cue
	if g.musicCue().name != previousMusic {
		g.musicOffset = 0
	}
	switch cue.Stage {
	case sequence.Opening:
		return g.opening.Update(float64(cue.Tick) / FPS)
	case sequence.FourLoader, sequence.SpoonLoader:
		return g.loader.Update(cue.Tick)
	}
	target := cue.Tick
	if cue.Stage == sequence.FourFade {
		target += g.fourFadeBase
	}
	if cue.Stage == sequence.Jelly {
		target += jelly.EntryTick
	}
	if seekable, ok := g.part.(interface{ Seek(int) error }); ok && int(g.part.Tick())+1 != target {
		return seekable.Seek(target)
	}
	for int(g.part.Tick()) < target {
		if err := g.part.Update(); err != nil {
			return err
		}
	}
	return nil
}
