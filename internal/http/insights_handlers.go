package http

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/astianmuchui/mobilesonar/external/nasa"
	"github.com/astianmuchui/mobilesonar/internal/ml"
	rdb "github.com/astianmuchui/mobilesonar/internal/redis"
)

func InsightsPageHandler(c *fiber.Ctx) error {
	data := commonData(c)
	data["Title"] = "Fishing Insights"
	data["active"] = "insights"
	return c.Render("dashboard/insights", data)
}

type mlEnvironment struct {
	Overall        float64            `json:"overall"`
	Recommendation string             `json:"recommendation"`
	Factors        map[string]float64 `json:"factors"`
}

type mlTimeWindow struct {
	Start          string   `json:"start"`
	End            string   `json:"end"`
	AvgScore       float64  `json:"avg_score"`
	PeakScore      float64  `json:"peak_score"`
	Recommendation string   `json:"recommendation"`
	SpeciesActive  []string `json:"species_active"`
}

type mlLocation struct {
	Lat             float64 `json:"lat"`
	Lon             float64 `json:"lon"`
	TotalCatches    int     `json:"total_catches"`
	RecentWeight    float64 `json:"recent_weight"`
	DominantSpecies string  `json:"dominant_species"`
}

type mlSpeciesPred struct {
	Species    string             `json:"species"`
	Confidence float64            `json:"confidence"`
	Votes      map[string]float64 `json:"votes"`
	SampleSize int                `json:"sample_size"`
}

type mlExpectedSpecies struct {
	Now          *mlSpeciesPred `json:"now"`
	AtPeakWindow *mlSpeciesPred `json:"at_peak_window"`
}

type mlCatchTrend struct {
	Direction string  `json:"direction"`
	Recent    int     `json:"recent"`
	Previous  int     `json:"previous"`
	ChangePct float64 `json:"change_pct"`
}

type insightWindow struct {
	Label          string   `json:"label"`
	Recommendation string   `json:"recommendation"`
	Score          int      `json:"score"`
	Species        []string `json:"species"`
}

type insightSpecies struct {
	Name       string `json:"name"`
	Confidence int    `json:"confidence"`
	SampleSize int    `json:"sample_size"`
}

type insightZone struct {
	Lat             float64 `json:"lat"`
	Lon             float64 `json:"lon"`
	DominantSpecies string  `json:"dominant_species"`
	TotalCatches    int     `json:"total_catches"`
}

type insightTrend struct {
	Direction string  `json:"direction"`
	ChangePct float64 `json:"change_pct"`
	Recent    int     `json:"recent"`
	Previous  int     `json:"previous"`
}

type insightsResponse struct {
	Headline    string               `json:"headline"`
	Subtext     string               `json:"subtext"`
	Verdict     string               `json:"verdict"`
	Score       int                  `json:"score"`
	Now         string               `json:"now"`
	Weather     *nasa.WeatherInsight `json:"weather"`
	Advisory    string               `json:"advisory"`
	Windows     []insightWindow      `json:"windows"`
	Species     []insightSpecies     `json:"species"`
	Zones       []insightZone        `json:"zones"`
	Trend       *insightTrend        `json:"trend"`
	SampleSize  int                  `json:"sample_size"`
	GeneratedAt string               `json:"generated_at"`
}

func GetInsightsHandler(c *fiber.Ctx) error {
	lat := c.QueryFloat("lat", 0)
	lon := c.QueryFloat("lon", 0)
	if lat == 0 && lon == 0 {
		lat, lon = -1.0, 33.0
	}
	radius := c.QueryFloat("radius_m", 2000)

	loc, err := time.LoadLocation("Africa/Nairobi")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)

	var latest *nasa.WeatherInsight
	var mlWeather *ml.MLWeather
	cacheKey := fmt.Sprintf("weather:%.2f:%.2f", lat, lon)
	if raw, wErr := rdb.GetOrSet(c.UserContext(), cacheKey, time.Hour, func() (any, error) {
		return nasa.Fetch(lat, lon)
	}); wErr == nil {
		var insights []nasa.WeatherInsight
		if json.Unmarshal(raw, &insights) == nil && len(insights) > 0 {
			latest = &insights[len(insights)-1]
			mlWeather = &ml.MLWeather{
				Temperature:    latest.TemperatureC,
				SolarRadiation: latest.SolarRadiation,
				WindSpeed:      latest.WindSpeedMS * 3.6,
				Precipitation:  latest.RainfallMM,
			}
		}
	}

	resp := insightsResponse{
		Verdict:     "unknown",
		Weather:     latest,
		GeneratedAt: now.Format("Mon 3:04 PM"),
		Windows:     []insightWindow{},
		Species:     []insightSpecies{},
		Zones:       []insightZone{},
	}

	ins, mlErr := ml.GetInsights(lat, lon, radius, mlWeather)
	if mlErr != nil {
		resp.Headline = "Could not load live insights"
		resp.Subtext = "The insight service is unreachable; showing weather only."
		if latest != nil {
			resp.Score = latest.FishingScore
		}
		return c.JSON(resp)
	}

	resp.Advisory = ins.Advisory
	resp.SampleSize = ins.SampleSize

	var env mlEnvironment
	if json.Unmarshal(ins.CurrentEnvironment, &env) == nil {
		resp.Score = int(env.Overall*10 + 0.5)
		resp.Now = humanRecommendation(env.Recommendation)
		switch env.Recommendation {
		case "prime", "good":
			resp.Verdict = "good"
		case "moderate":
			resp.Verdict = "fair"
		case "poor":
			resp.Verdict = "poor"
		}
	}

	var windows []mlTimeWindow
	if json.Unmarshal(ins.BestTimeWindows, &windows) == nil {
		for _, w := range windows {
			resp.Windows = append(resp.Windows, insightWindow{
				Label:          windowLabel(w.Start, w.End, loc),
				Recommendation: w.Recommendation,
				Score:          int(w.PeakScore*100 + 0.5),
				Species:        w.SpeciesActive,
			})
		}
	}

	var exp mlExpectedSpecies
	if json.Unmarshal(ins.ExpectedSpecies, &exp) == nil && exp.Now != nil {
		for name, vote := range exp.Now.Votes {
			resp.Species = append(resp.Species, insightSpecies{
				Name:       name,
				Confidence: int(vote*100 + 0.5),
				SampleSize: exp.Now.SampleSize,
			})
		}
		sortSpeciesDesc(resp.Species)
	}

	var zones []mlLocation
	if json.Unmarshal(ins.BestLocations, &zones) == nil {
		for _, z := range zones {
			resp.Zones = append(resp.Zones, insightZone{
				Lat:             z.Lat,
				Lon:             z.Lon,
				DominantSpecies: z.DominantSpecies,
				TotalCatches:    z.TotalCatches,
			})
		}
	}

	var trend mlCatchTrend
	if json.Unmarshal(ins.CatchTrend, &trend) == nil && trend.Direction != "" {
		resp.Trend = &insightTrend{
			Direction: trend.Direction,
			ChangePct: trend.ChangePct,
			Recent:    trend.Recent,
			Previous:  trend.Previous,
		}
	}

	resp.Headline, resp.Subtext = buildHeadline(resp)

	return c.JSON(resp)
}

func humanRecommendation(rec string) string {
	switch rec {
	case "prime":
		return "Prime conditions right now."
	case "good":
		return "Good conditions right now."
	case "moderate":
		return "Moderate conditions right now."
	case "poor":
		return "Poor conditions right now."
	default:
		return ""
	}
}

func windowLabel(start, end string, loc *time.Location) string {
	s, errS := time.Parse(time.RFC3339, start)
	e, errE := time.Parse(time.RFC3339, end)
	if errS != nil || errE != nil {
		return "—"
	}
	sLocal := s.In(loc)
	eLocal := e.In(loc)
	if !eLocal.After(sLocal) || sLocal.Hour() == eLocal.Hour() {
		eLocal = time.Date(sLocal.Year(), sLocal.Month(), sLocal.Day(),
			sLocal.Hour()+1, 0, 0, 0, loc)
	}
	return fmt.Sprintf("%s – %s", clockLabel(sLocal), clockLabel(eLocal))
}

func clockLabel(t time.Time) string {
	h := t.Hour()
	switch {
	case h == 0:
		return "12 AM"
	case h < 12:
		return fmt.Sprintf("%d AM", h)
	case h == 12:
		return "12 PM"
	default:
		return fmt.Sprintf("%d PM", h-12)
	}
}

func sortSpeciesDesc(s []insightSpecies) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1].Confidence < s[j].Confidence; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func buildHeadline(r insightsResponse) (string, string) {
	headline := "Not enough history yet — try dawn (around 6 AM) or dusk (around 6 PM)"
	if len(r.Species) > 0 && len(r.Windows) > 0 {
		headline = fmt.Sprintf("Most likely to catch %s — best window %s",
			r.Species[0].Name, r.Windows[0].Label)
	} else if len(r.Windows) > 0 {
		headline = fmt.Sprintf("Best fishing window today: %s", r.Windows[0].Label)
	} else if len(r.Species) > 0 {
		headline = fmt.Sprintf("Most likely to catch %s here", r.Species[0].Name)
	}

	subtext := r.Advisory
	if subtext == "" {
		switch r.Verdict {
		case "good":
			subtext = fmt.Sprintf("Conditions look good today (score %d/10).", r.Score)
		case "fair":
			subtext = fmt.Sprintf("Conditions are fair today (score %d/10).", r.Score)
		case "poor":
			subtext = fmt.Sprintf("Conditions are poor today (score %d/10).", r.Score)
		default:
			subtext = "Based on recent catches and live weather."
		}
	}
	return headline, subtext
}
