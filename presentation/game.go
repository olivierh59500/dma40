// Package presentation runs DMA40 as an automatic sequence without a menu.
package presentation

import (
	"fmt"
	"image/color"
	"math"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/timeline"
	"github.com/olivierh59500/dma40/internal/displayclock"
	"github.com/olivierh59500/dma40/internal/gesture"
	"github.com/olivierh59500/dma40/internal/introassets"
	"github.com/olivierh59500/dma40/internal/partassets"
	"github.com/olivierh59500/dma40/internal/sequence"
	"github.com/olivierh59500/dma40/internal/startup"
	"github.com/olivierh59500/dma40/jelly"
	"github.com/olivierh59500/dma40/opening"
	"github.com/olivierh59500/dma40/quad"
	"github.com/olivierh59500/dma40/spoon"
)

const Width, Height, FPS = 640, 400, startup.FPS

type Options struct {
	Mute, DisableAudio, Deterministic bool
	StartPart                         string
	StartTime                         time.Duration
	QuadDuration, NonameDuration      time.Duration
	OnState                           func(State)
}

type State struct {
	Stage     string `json:"stage"`
	Effect    string `json:"effect"`
	Tick      int    `json:"tick"`
	LocalTick int    `json:"local_tick"`
	Paused    bool   `json:"paused"`
	Muted     bool   `json:"muted"`
}

type scene interface {
	Update() error
	Draw(*ebiten.Image)
	Tick() uint64
	Close()
}

type Game struct {
	art           *introassets.Set
	media         *partassets.Set
	clock         *timeline.CueClock
	plan          *sequence.Plan
	cue           sequence.Cue
	opening       *opening.Scene
	loader        *opening.Loader
	part          scene
	partID        string
	black         *ebiten.Image
	visualClock   displayclock.Clock
	audio         soundtrack
	loaderAudio   soundtrack
	options       Options
	paused, muted bool
	nonameSeconds float64
	tracks        map[string][]byte
	musicOffset   int
	fourFadeBase  int
	advanceTap    gesture.DoubleTap
	touches       []ebiten.TouchID
	contacts      []gesture.Contact
}

func New(options Options) (_ *Game, err error) {
	if options.QuadDuration == 0 {
		options.QuadDuration = quad.PresentationSeconds * time.Second
	}
	if options.NonameDuration == 0 {
		options.NonameDuration = time.Duration(spoon.DefaultNonameSeconds) * time.Second
	}
	if options.QuadDuration < time.Second/FPS || options.QuadDuration > time.Hour {
		return nil, fmt.Errorf("four-panel duration must be positive and fit within an hour")
	}
	if options.NonameDuration < time.Second/FPS || options.NonameDuration > 50*time.Minute {
		return nil, fmt.Errorf("Noname duration must be between one frame and fifty minutes")
	}
	nonameTicks := (options.NonameDuration*FPS + time.Second - 1) / time.Second
	g := &Game{options: options, muted: options.Mute, nonameSeconds: float64(nonameTicks) / FPS, tracks: make(map[string][]byte)}
	defer func() {
		if err != nil {
			g.Close()
		}
	}()
	g.art, err = introassets.Load()
	if err != nil {
		return nil, err
	}
	g.media, err = partassets.Load()
	if err != nil {
		return nil, err
	}
	g.opening, err = opening.NewSinglePass(g.art)
	if err != nil {
		return nil, err
	}
	openingTicks := int(math.Ceil((g.opening.ScrollEnd() + 2) * FPS))
	spoonTicks := int(math.Round(spoon.FinaleAt()*FPS)) + int(math.Round(g.nonameSeconds*FPS)) + int(math.Round(spoon.LogoHandoffSeconds*FPS))
	g.plan, err = sequence.New(openingTicks, int(options.QuadDuration*FPS/time.Second), spoonTicks)
	if err != nil {
		return nil, err
	}
	g.clock, err = timeline.NewCueClock(timeline.CueClockConfig{Rate: FPS})
	if err != nil {
		return nil, err
	}
	g.black = ebiten.NewImage(1, 1)
	g.black.Fill(color.Black)
	if err = g.Seek(options.StartPart, options.StartTime); err != nil {
		return nil, err
	}
	return g, nil
}

// Seek enters a part directly for review, then resumes the ordinary sequence.
// Preview aliases retain every preceding effect in normal playback.
func (g *Game) Seek(part string, position time.Duration) error {
	if part == "" {
		part = sequence.Opening
	}
	if position < 0 || position > time.Hour {
		return fmt.Errorf("preview time must be within 0-3600 seconds")
	}
	switch part {
	case "quad":
		part = sequence.Four
	case "noname":
		part, position = sequence.Spoon, position+time.Duration(math.Round(spoon.FinaleAt()*FPS))*time.Second/FPS
	case "handoff":
		part, position = sequence.Spoon, position+time.Duration(math.Round((spoon.FinaleAt()+g.nonameSeconds)*FPS))*time.Second/FPS
	case "flight":
		part, position = sequence.Spoon, position+time.Duration(spoon.FlightAt*FPS)*time.Second/FPS
	case "city":
		part, position = sequence.Spoon, position+time.Duration(math.Round(spoon.CityAt()*FPS))*time.Second/FPS
	case "circle":
		part, position = sequence.Spoon, position+time.Duration(math.Round(spoon.CircleAt*FPS))*time.Second/FPS
	case "boids":
		part, position = sequence.Jelly, position+time.Duration(jelly.BoidsAt*FPS-jelly.EntryTick)*time.Second/FPS
	case "portrait":
		part, position = sequence.Jelly, position+time.Duration(jelly.PortraitAt*FPS-jelly.EntryTick)*time.Second/FPS
	case "greeting":
		part, position = sequence.Jelly, position+time.Duration(jelly.GreetingAt*FPS-jelly.EntryTick)*time.Second/FPS
	case "pi":
		part, position = sequence.Jelly, position+time.Duration(int(math.Round(jelly.PiAt*FPS))-jelly.EntryTick)*time.Second/FPS
	case "pi-page":
		part, position = sequence.Jelly, position+time.Duration(int(math.Round(jelly.PiPageAt*FPS))-jelly.EntryTick)*time.Second/FPS
	}
	tick, err := g.plan.Start(part, int(position*FPS/time.Second))
	if err != nil {
		return err
	}
	g.audio.close()
	g.loaderAudio.close()
	g.closeScenes(part == sequence.Opening)
	g.cue = sequence.Cue{}
	g.musicOffset = 0
	g.fourFadeBase = g.plan.Length(sequence.Four)
	g.advanceTap.Reset()
	g.clock.Reset()
	for range tick {
		g.clock.Step()
	}
	if err := g.sample(g.plan.At(tick)); err != nil {
		return err
	}
	g.publish()
	return nil
}

func (g *Game) Update() error {
	if runtime.GOOS != "android" && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		g.muted = !g.muted
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.paused = !g.paused
	}
	advance := inpututil.IsKeyJustPressed(ebiten.KeySpace)
	if runtime.GOOS == "android" {
		g.touches = ebiten.AppendTouchIDs(g.touches[:0])
		g.contacts = g.contacts[:0]
		for _, id := range g.touches {
			x, y := ebiten.TouchPosition(id)
			g.contacts = append(g.contacts, gesture.Contact{ID: int(id), X: float64(x), Y: float64(y)})
		}
		advance = g.advanceTap.Update(time.Now(), g.contacts) || advance
	}
	if advance {
		if err := g.Advance(); err != nil {
			return err
		}
	}
	return g.Step()
}

// Step advances only the DCK simulation clock; drawing never advances a cue.
func (g *Game) Step() error {
	// Audio backends are installed after Android package initialization. Open
	// the first track here, at its initial cue, before advancing the visual tick.
	if !g.options.DisableAudio && g.audio.player == nil {
		if err := g.syncAudio(); err != nil {
			return err
		}
	}
	if !g.paused {
		g.clock.Step()
		if err := g.sample(g.plan.At(g.clock.Tick())); err != nil {
			return err
		}
	}
	if err := g.syncAudio(); err != nil {
		return err
	}
	g.publish()
	if part, ok := g.part.(interface{ Done() bool }); ok && part.Done() {
		g.audio.close()
		g.loaderAudio.close()
		return ebiten.Termination
	}
	return nil
}

func (g *Game) Snapshot() State {
	effect := g.cue.Stage
	if source, ok := g.part.(interface{ Phase() string }); ok && g.cue.Stage != sequence.FourLoader && g.cue.Stage != sequence.SpoonLoader {
		effect = source.Phase()
	}
	return State{Stage: g.cue.Stage, Effect: effect, Tick: g.clock.Tick(), LocalTick: g.cue.Tick, Paused: g.paused, Muted: g.muted}
}
func (g *Game) publish() {
	if g.options.OnState != nil {
		g.options.OnState(g.Snapshot())
	}
}
func (g *Game) SetPaused(paused bool)      { g.paused = paused }
func (g *Game) SetMuted(muted bool)        { g.muted = muted }
func (g *Game) JellyStart() int            { return g.plan.JellyStart() }
func (g *Game) Layout(int, int) (int, int) { return Width, Height }

func (g *Game) Draw(dst *ebiten.Image) {
	dst.Fill(color.Black)
	opacity := 1.0
	switch g.cue.Stage {
	case sequence.Opening:
		seconds := float64(g.cue.Tick) / FPS
		if !g.options.Deterministic {
			if g.paused {
				seconds = g.visualClock.Hold(time.Now(), seconds)
			} else {
				seconds = g.visualClock.Sample(time.Now(), seconds)
			}
		}
		g.opening.DrawAt(dst, seconds)
		opacity = 1 - motion.Smooth(float64(g.cue.Tick-g.plan.Length(sequence.Opening)+FPS)/FPS)
	case sequence.FourLoader, sequence.SpoonLoader:
		g.loader.Draw(dst)
		opacity = 1 - motion.Smooth(float64(g.cue.Tick-startup.TransitionTicks+20)/20)
	case sequence.Four, sequence.FourFade:
		if live, ok := g.part.(interface {
			DrawLive(*ebiten.Image, time.Time, bool)
		}); ok && !g.options.Deterministic {
			live.DrawLive(dst, time.Now(), g.paused)
		} else {
			g.part.Draw(dst)
		}
		if g.cue.Stage == sequence.Four {
			opacity = motion.Smooth(float64(g.cue.Tick) / 40)
		} else {
			opacity = 1 - motion.Smooth(float64(g.cue.Tick)/sequence.FadeTicks)
		}
	default:
		if g.part != nil {
			if live, ok := g.part.(interface {
				DrawLive(*ebiten.Image, time.Time, bool)
			}); ok && !g.options.Deterministic {
				live.DrawLive(dst, time.Now(), g.paused)
			} else {
				g.part.Draw(dst)
			}
		}
	}
	if opacity < 1 {
		var options ebiten.DrawImageOptions
		options.GeoM.Scale(Width, Height)
		options.ColorScale.ScaleAlpha(float32(1 - max(0, opacity)))
		dst.DrawImage(g.black, &options)
	}
}

func (g *Game) Close() {
	g.audio.close()
	g.loaderAudio.close()
	g.closeScenes(false)
	if g.black != nil {
		g.black.Deallocate()
		g.black = nil
	}
}
