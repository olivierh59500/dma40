package spoon

import (
	"github.com/olivierh59500/dma40/city"
	"math"
)

// NextTick selects the next authored entrance, keeping the logo's final hold.
func (s *Scene) NextTick() (int, bool) { return nextSceneTick(int(s.tick), s.nonameSeconds) }

func nextSceneTick(tick int, nonameSeconds float64) (int, bool) {
	for _, seconds := range [...]float64{
		RevealAt, GatherAt, SpoonAt, SpoonSolidAt, BendAt, FlightAt, MatrixAt,
		SecretsAt, PacmanAt, FadeAt, CircleAt, CircleLogoAt, CircleLensAt,
		CityAt(), CityAt() + city.FormedAt, CityAt() + city.AskAt, CityAt() + city.ReplyAt,
		CityAt() + city.ByeAt, CityAt() + city.LaunchAt, CityAt() + city.OverheadAt,
		CityAt() + city.PortalAt, FinaleAt(), FinaleAt() + nonameSeconds,
		FinaleAt() + nonameSeconds + LogoSettleSeconds,
	} {
		next := int(math.Round(seconds * FPS))
		if next > tick {
			return next, true
		}
	}
	return 0, false
}
