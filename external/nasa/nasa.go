package nasa

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"
)

const baseURL = "https://power.larc.nasa.gov/api/temporal/daily/point"

var client = &http.Client{
	Timeout: 15 * time.Second,
}

type WeatherInsight struct {
	Date           string  `json:"date"`
	TemperatureC   float64 `json:"temperature_c"`
	RainfallMM     float64 `json:"rainfall_mm"`
	WindSpeedMS    float64 `json:"wind_speed_ms"`
	SolarRadiation float64 `json:"solar_radiation"`
	FishingScore   int     `json:"fishing_score"`
	Recommendation string  `json:"recommendation"`
}

type powerResponse struct {
	Properties struct {
		Parameter map[string]map[string]float64 `json:"parameter"`
	} `json:"properties"`
}

const nasaFillValue = -999.0

func Fetch(lat, lon float64) ([]WeatherInsight, error) {
	end := time.Now().AddDate(0, 0, -4).Format("20060102")
	start := time.Now().AddDate(0, 0, -10).Format("20060102")

	url := fmt.Sprintf(
		"%s?parameters=T2M,PRECTOTCORR,WS2M,ALLSKY_SFC_SW_DWN&community=RE&longitude=%.4f&latitude=%.4f&start=%s&end=%s&format=JSON",
		baseURL, lon, lat, start, end,
	)

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("nasa: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nasa: unexpected status %d", resp.StatusCode)
	}

	var pr powerResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("nasa: decode failed: %w", err)
	}

	params := pr.Properties.Parameter
	t2m := params["T2M"]
	prec := params["PRECTOTCORR"]
	ws2m := params["WS2M"]
	solar := params["ALLSKY_SFC_SW_DWN"]

	dates := make([]string, 0, len(t2m))
	for d := range t2m {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	insights := make([]WeatherInsight, 0, len(dates))
	for _, d := range dates {

		if t2m[d] <= nasaFillValue || prec[d] <= nasaFillValue || ws2m[d] <= nasaFillValue {
			continue
		}
		score, rec := fishingScore(t2m[d], prec[d], ws2m[d])
		insights = append(insights, WeatherInsight{
			Date:           d,
			TemperatureC:   round(t2m[d], 1),
			RainfallMM:     round(prec[d], 2),
			WindSpeedMS:    round(ws2m[d], 1),
			SolarRadiation: round(solar[d], 1),
			FishingScore:   score,
			Recommendation: rec,
		})
	}

	return insights, nil
}

func fishingScore(tempC, rainfallMM, windMS float64) (int, string) {
	score := 10

	if rainfallMM > 20 {
		score -= 3
	} else if rainfallMM > 5 {
		score -= 1
	}

	if windMS > 8 {
		score -= 3
	} else if windMS > 5 {
		score -= 1
	}

	if tempC < 16 || tempC > 34 {
		score -= 2
	}

	if score < 1 {
		score = 1
	}

	switch {
	case score >= 9:
		return score, "Excellent conditions — fish all day"
	case score >= 7:
		return score, "Good conditions — early morning recommended"
	case score >= 5:
		return score, "Fair conditions — try dawn or dusk"
	default:
		return score, "Poor conditions — consider waiting"
	}
}

func round(f float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(f*p) / p
}
