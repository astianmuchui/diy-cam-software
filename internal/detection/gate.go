package detection

import (
	"fmt"
	"math"

	"github.com/astianmuchui/mobilesonar/internal/models"
)

type GateResult struct {
	Valid         bool
	RejectReasons []string
}

func Gate(r *models.SensorReading) GateResult {
	var reasons []string

	if r.TiltAngle > 25.0 {
		reasons = append(reasons, fmt.Sprintf("tilt %.1f° exceeds 25° limit", r.TiltAngle))
	}

	if r.WaterDepthM < 1.0 {
		reasons = append(reasons, fmt.Sprintf("water depth %.2fm below 1m minimum", r.WaterDepthM))
	}

	if r.DistanceCM < 100.0 {
		reasons = append(reasons, fmt.Sprintf("distance %.1fcm below 100cm minimum", r.DistanceCM))
	}

	accelMag := math.Sqrt(
		float64(r.AccelX)*float64(r.AccelX) +
			float64(r.AccelY)*float64(r.AccelY) +
			float64(r.AccelZ)*float64(r.AccelZ),
	)
	if math.Abs(accelMag-1.0) > 0.4 {
		reasons = append(reasons, fmt.Sprintf("accel magnitude %.3fg deviates >0.4g from 1g", accelMag))
	}

	return GateResult{
		Valid:         len(reasons) == 0,
		RejectReasons: reasons,
	}
}
