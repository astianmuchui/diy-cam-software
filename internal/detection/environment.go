package detection

import (
	"time"

	"github.com/astianmuchui/mobilesonar/external/nasa"
)

type EnvironmentScore struct {
	Overall        float64            `json:"overall"`
	Factors        map[string]float64 `json:"factors"`
	Recommendation string             `json:"recommendation"`
}

func ScoreEnvironment(insights []nasa.WeatherInsight, ts time.Time) EnvironmentScore {
	factors := map[string]float64{
		"time_of_day": scoreTimeOfDay(ts),
		"temperature": 0.5,
		"solar":       0.5,
		"wind":        0.5,
		"rainfall":    0.5,
	}

	if len(insights) > 0 {

		latest := insights[len(insights)-1]
		factors["temperature"] = scoreTemperature(latest.TemperatureC)
		factors["solar"] = scoreSolar(latest.SolarRadiation)
		factors["wind"] = scoreWind(latest.WindSpeedMS)
		factors["rainfall"] = scoreRainfall(latest.RainfallMM)
	}

	overall := factors["time_of_day"]*0.30 +
		factors["temperature"]*0.25 +
		factors["solar"]*0.20 +
		factors["wind"]*0.15 +
		factors["rainfall"]*0.10

	return EnvironmentScore{
		Overall:        clamp01(overall),
		Factors:        factors,
		Recommendation: envRecommendation(overall),
	}
}

func scoreTimeOfDay(ts time.Time) float64 {
	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		loc = time.UTC
	}
	local := ts.In(loc)
	h := local.Hour()
	m := local.Minute()
	minuteOfDay := h*60 + m

	dawn := [2]int{5 * 60, 7 * 60}
	earlyMorn := [2]int{7 * 60, 9 * 60}
	midday := [2]int{10 * 60, 14 * 60}
	lateAfter := [2]int{15*60 + 30, 17*60 + 30}
	dusk := [2]int{17*60 + 30, 19 * 60}

	switch {
	case minuteOfDay >= dawn[0] && minuteOfDay < dawn[1]:
		return 1.0
	case minuteOfDay >= dusk[0] && minuteOfDay < dusk[1]:
		return 1.0
	case minuteOfDay >= earlyMorn[0] && minuteOfDay < earlyMorn[1]:
		return 0.7
	case minuteOfDay >= lateAfter[0] && minuteOfDay < lateAfter[1]:
		return 0.7
	case minuteOfDay >= midday[0] && minuteOfDay < midday[1]:
		return 0.3
	default:

		return 0.4
	}
}

func scoreTemperature(tempC float64) float64 {
	switch {
	case tempC >= 22 && tempC <= 28:
		return 1.0
	case (tempC >= 18 && tempC < 22) || (tempC > 28 && tempC <= 32):
		return 0.6
	default:
		return 0.2
	}
}

func scoreSolar(solarWm2 float64) float64 {
	if solarWm2 <= 0 {
		return 0.5
	}
	normalised := (solarWm2 - 200) / 600
	normalised = clamp01(normalised)

	return 0.8 - normalised*0.4
}

func scoreWind(windMS float64) float64 {
	windKPH := windMS * 3.6
	switch {
	case windKPH >= 5 && windKPH <= 15:
		return 0.8
	case windKPH < 5:
		return 0.5
	default:
		return 0.3
	}
}

func scoreRainfall(rainfallMM float64) float64 {
	switch {
	case rainfallMM == 0:
		return 0.5
	case rainfallMM <= 10:
		return 0.7
	default:
		return 0.3
	}
}

func envRecommendation(overall float64) string {
	switch {
	case overall >= 0.75:
		return "prime"
	case overall >= 0.55:
		return "good"
	case overall >= 0.35:
		return "moderate"
	default:
		return "poor"
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
