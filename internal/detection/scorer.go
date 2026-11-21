package detection

import (
	"math"

	"github.com/astianmuchui/mobilesonar/internal/models"
)

type Detection struct {
	FishDetected   bool               `json:"fish_detected"`
	Confidence     float64            `json:"confidence"`
	Classification string             `json:"classification"`
	CorrectedDepth float64            `json:"corrected_depth_m"`
	Baseline       float64            `json:"baseline_m"`
	Factors        map[string]float64 `json:"factors"`
	GateRejected   bool               `json:"gate_rejected"`
	RejectReasons  []string           `json:"reject_reasons,omitempty"`
}

func Score(
	correctedDepth, baseline float64,
	reading *models.SensorReading,
	envScore EnvironmentScore,
	prevCorrDepth, nextCorrDepth *float64,
) Detection {
	factors := make(map[string]float64)

	deviation := baseline - correctedDepth
	var deviationScore float64
	if deviation > 0 && baseline > 0 {
		deviationScore = clamp01(deviation / (baseline * 0.015))
	}
	factors["depth_deviation"] = deviationScore

	gyroMag := math.Sqrt(
		float64(reading.GyroX)*float64(reading.GyroX) +
			float64(reading.GyroY)*float64(reading.GyroY) +
			float64(reading.GyroZ)*float64(reading.GyroZ),
	)
	var gyroPenalty float64
	if gyroMag > 30 {
		gyroPenalty = math.Min((gyroMag-30)/60, 0.3)
	}
	factors["gyro_penalty"] = -gyroPenalty

	accelMag := math.Sqrt(
		float64(reading.AccelX)*float64(reading.AccelX) +
			float64(reading.AccelY)*float64(reading.AccelY) +
			float64(reading.AccelZ)*float64(reading.AccelZ),
	)
	var accelBonus float64
	if math.Abs(accelMag-1.0) <= 0.15 && deviationScore > 0.2 {
		accelBonus = 0.15
	}
	factors["accel_bonus"] = accelBonus

	var transienceAdj float64
	if prevCorrDepth != nil && nextCorrDepth != nil && baseline > 0 {
		prevDev := baseline - *prevCorrDepth
		nextDev := baseline - *nextCorrDepth

		neighboursNearBaseline := prevDev < deviation*0.3 && nextDev < deviation*0.3
		if neighboursNearBaseline && deviationScore > 0 {
			transienceAdj = 0.15
		} else if prevDev > deviation*0.5 && nextDev > deviation*0.5 {
			transienceAdj = -0.05
		}
	}
	factors["transience"] = transienceAdj

	var tiltBonus float64
	if float64(reading.TiltAngle) < 5.0 && deviationScore > 0.2 {
		tiltBonus = 0.10
	}
	factors["tilt_bonus"] = tiltBonus

	rawScore := deviationScore - gyroPenalty + accelBonus + transienceAdj + tiltBonus
	rawScore = clamp01(rawScore)

	envWeight := 0.6 + envScore.Overall*0.8
	factors["env_weight"] = envWeight

	finalConfidence := clamp01(rawScore * envWeight)
	factors["raw_score"] = rawScore
	factors["final_confidence"] = finalConfidence

	classification := classify(finalConfidence)

	return Detection{
		FishDetected:   finalConfidence > 0.45,
		Confidence:     finalConfidence,
		Classification: classification,
		CorrectedDepth: correctedDepth,
		Baseline:       baseline,
		Factors:        factors,
	}
}

func classify(confidence float64) string {
	switch {
	case confidence > 0.45:
		return "likely_fish"
	case confidence > 0.25:
		return "possible"
	case confidence > 0.10:
		return "unlikely"
	default:
		return "no_detection"
	}
}
