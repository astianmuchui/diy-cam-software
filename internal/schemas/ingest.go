package schemas

import "errors"

type IngestReadingRequest struct {
	DeviceID   string            `json:"device_id"`
	Timestamp  int64             `json:"timestamp"`
	GPS        GPSPayload        `json:"gps"`
	Ultrasonic UltrasonicPayload `json:"ultrasonic"`
	MPU6050    MPUPayload        `json:"mpu6050"`
	Sonar      SonarPayload      `json:"sonar"`
	System     SystemPayload     `json:"system"`
	Camera     CameraPayload     `json:"camera"`
}

type CameraPayload struct {
	ReadingID string `json:"reading_id"`
}

type GPSPayload struct {
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Accuracy float32 `json:"accuracy"`
}

type UltrasonicPayload struct {
	DistanceCM float32 `json:"distance_cm"`
	Valid      bool    `json:"valid"`
}

type MPUPayload struct {
	AccelX float32 `json:"accel_x"`
	AccelY float32 `json:"accel_y"`
	AccelZ float32 `json:"accel_z"`
	GyroX  float32 `json:"gyro_x"`
	GyroY  float32 `json:"gyro_y"`
	GyroZ  float32 `json:"gyro_z"`
}

type SonarPayload struct {
	EchoAmplitude float32 `json:"echo_amplitude"`
	FishDetected  bool    `json:"fish_detected"`
	FishCount     int     `json:"fish_count"`
	FishType      string  `json:"fish_type"`
}

type SystemPayload struct {
	BatteryVoltage   float32 `json:"battery_voltage"`
	SignalStrength   int     `json:"signal_strength"`
	TransmissionType string  `json:"transmission_type"`
}

type HeartbeatRequest struct {
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
}

func (r *IngestReadingRequest) Validate() error {
	if r.DeviceID == "" {
		return errors.New("device_id is required")
	}
	if r.Timestamp == 0 {
		return errors.New("timestamp is required")
	}
	return nil
}
