package spoon

import (
	"bytes"
	"fmt"
	"image"
	"io/fs"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/render"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/democonstructionkit/sprites"
)

type finaleTextPage struct {
	heading string
	lines   [8]string
}

var finaleText = [...]finaleTextPage{
	{heading: "MEMBERS OF DMA", lines: [8]string{
		"", "ALDO   BILIZIR", "CHONCHON CIGALE COCO", "CORWIN   DWORKIN",
		"GRUMPY   JINX", "LARCOCIEL LOCKBUSTER", "MAD   MAD GILLOU", "METALIKE PDM TWISTER",
	}},
	{heading: "GREETINGS", lines: [8]string{
		"", "", "TO ALL DEMOSCENE", "ENTHUSIASTS", "AND YOU !", "", "", "",
	}},
	{lines: [8]string{
		"", "", "RENDEZ-VOUS", "TOUS LES MERCREDIS", "A 17H30", "POUR REGARDER", "LATEAM", "",
	}},
}

// finalePages keeps the original twenty-column, eight-row choreography. Small
// headings stay separate, and the greetings reserve a blank row below theirs.
func finalePages() ([]string, error) {
	var pages []string
	for _, text := range finaleText {
		var page strings.Builder
		for _, line := range text.lines {
			length := utf8.RuneCountInString(line)
			if length > 20 {
				return nil, fmt.Errorf("finale page line exceeds twenty columns: %q", line)
			}
			left := (20 - length) / 2
			page.WriteString(strings.Repeat(" ", left) + line + strings.Repeat(" ", 20-length-left))
		}
		pages = append(pages, page.String())
	}
	return pages, nil
}

func finalePageConfig(pages []string) (motion.GlyphPageConfig, error) {
	config, err := presets.NonamenoGlyphPages(pages, 0)
	if err != nil {
		return config, err
	}
	config.Origin.Y = 86
	config.Center.Y = Height / 2
	config.DelayStepMS = 20
	// Preserve each two-second elastic entrance, then hold the settled letters
	// for 3.5 seconds before DCK's completion barrier starts the outgoing wave.
	enter, ease := config.EnterDurationMS, config.EnterEase
	config.EnterDurationMS += 3500
	duration := config.EnterDurationMS
	config.EnterEase = func(t float64) float64 { return ease(min(1, t*duration/enter)) }
	return config, nil
}

type nonameFinale struct {
	textures           []*ebiten.Image
	logo               *bilizirLogo
	font               *scrolling.Atlas
	headings           []*scrolling.Scrolling
	headingAlpha       float64
	pageStart, pageEnd float64
	stars              *starFlight
	pages              *motion.GlyphPageCycle
	painter            *sprites.GlyphPages
	config             motion.GlyphPageConfig
	frame              int64
	meter              *finaleEqualizer
	background         *ebiten.Image
	seconds, settleAt  float64
}

func newNonameFinale(files fs.FS, logo image.Image, settleAt float64) (_ *nonameFinale, err error) {
	n := &nonameFinale{settleAt: settleAt}
	defer func() {
		if err != nil {
			n.Close()
		}
	}()
	loadFont := func(name, preset string) (*scrolling.Atlas, error) {
		raw, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, err
		}
		im, _, err := image.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		texture := ebiten.NewImageFromImage(im)
		n.textures = append(n.textures, texture)
		return presets.FontAtlas(preset, texture)
	}
	n.font, err = loadFont("noname-font.png", "nonameno-demo")
	if err != nil {
		return nil, err
	}
	face := n.font.Face()
	face.ScaleX, face.ScaleY = .5, .5
	n.headings = make([]*scrolling.Scrolling, len(finaleText))
	for i, text := range finaleText {
		if text.heading == "" {
			continue
		}
		n.headings[i], err = scrolling.New(scrolling.Config{
			Text: text.heading, Font: "noname", Fonts: map[string]scrolling.Face{"noname": face},
			X: (Width - float64(utf8.RuneCountInString(text.heading))*16) / 2, Y: 84,
			Map: func(_ scrolling.Sample, options *ebiten.DrawImageOptions) bool {
				options.ColorScale.ScaleAlpha(float32(n.headingAlpha))
				return true
			},
		})
		if err != nil {
			return nil, err
		}
	}
	pages, err := finalePages()
	if err != nil {
		return nil, err
	}
	n.config, err = finalePageConfig(pages)
	if err != nil {
		return nil, err
	}
	n.meter, err = newFinaleEqualizer(files)
	if err != nil {
		return nil, err
	}
	n.logo, err = newBilizirLogo(logo)
	if err != nil {
		return nil, err
	}
	n.logo.settleAt = settleAt
	n.background = render.NewSurface(Width, Height)
	if err = n.reset(); err != nil {
		return nil, err
	}
	return n, nil
}

func (n *nonameFinale) reset() error {
	if n.stars != nil {
		n.stars.Close()
	}
	var err error
	n.stars, err = newStarFlight()
	if err != nil {
		return err
	}
	n.pages, err = motion.NewGlyphPageCycle(n.config)
	if err != nil {
		return err
	}
	n.painter, err = sprites.NewGlyphPages(sprites.GlyphPagesConfig{Cycle: n.pages, Font: n.font})
	n.frame = 0
	n.pageStart, n.pageEnd, n.headingAlpha = 0, -1, 0
	return err
}

// Replay the source's 60 Hz choreography on the host's 50 Hz timeline. Seeking
// reconstructs its completion barriers rather than jumping a stateful tween.
func (n *nonameFinale) Update(f kit.Frame) error {
	n.seconds = f.Time
	target := int64(math.Floor(f.Time*60 + 1e-7))
	if target < n.frame {
		if err := n.reset(); err != nil {
			return err
		}
	}
	for n.frame < target {
		n.frame++
		page, phase := n.pages.PageIndex(), n.pages.Phase()
		// Keep the last, fully readable page until the logo handoff. DCK has
		// reached its completion barrier but has not moved any outgoing glyph.
		if page == len(n.config.Pages)-1 && phase == motion.GlyphPageExiting {
			continue
		}
		if err := n.pages.UpdateAt(float64(n.frame) * 1000 / 60); err != nil {
			return err
		}
		if n.pages.PageIndex() != page {
			n.pageStart, n.pageEnd = float64(n.frame)/60, -1
		} else if phase != n.pages.Phase() {
			n.pageEnd = float64(n.frame) / 60
		}
	}
	n.stars.setTime(f.Time)
	n.headingAlpha = smooth(0, 1, f.Time-n.pageStart)
	if n.pageEnd >= 0 {
		n.headingAlpha *= 1 - smooth(0, .5, f.Time-n.pageEnd)
	}
	if heading := n.headings[n.pages.PageIndex()]; heading != nil {
		if err := heading.Update(f); err != nil {
			return err
		}
	}
	return n.logo.Update(f)
}

func (n *nonameFinale) drawText(dst *ebiten.Image) {
	if heading := n.headings[n.pages.PageIndex()]; heading != nil {
		heading.Draw(dst)
	}
	n.painter.Draw(dst)
}

func (n *nonameFinale) Draw(dst *ebiten.Image) {
	if age := n.seconds - n.settleAt; age >= 0 {
		if age < LogoFadeSeconds {
			n.background.Clear()
			n.stars.Draw(n.background)
			n.meter.Draw(n.background)
			n.drawText(n.background)
			var options ebiten.DrawImageOptions
			options.ColorScale.ScaleAlpha(float32(1 - motion.Smooth(age/LogoFadeSeconds)))
			dst.DrawImage(n.background, &options)
		}
		n.logo.Draw(dst)
		return
	}
	n.stars.Draw(dst)
	n.meter.Draw(dst)
	n.logo.Draw(dst)
	n.drawText(dst)
}

func (n *nonameFinale) Close() error {
	for _, heading := range n.headings {
		if heading != nil {
			heading.Close()
		}
	}
	n.headings = nil
	if n.background != nil {
		n.background.Deallocate()
		n.background = nil
	}
	if n.logo != nil {
		n.logo.Close()
		n.logo = nil
	}
	if n.meter != nil {
		n.meter.Close()
		n.meter = nil
	}
	if n.stars != nil {
		n.stars.Close()
		n.stars = nil
	}
	for _, texture := range n.textures {
		texture.Deallocate()
	}
	n.textures = nil
	return nil
}
