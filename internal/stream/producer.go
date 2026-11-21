package stream

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	rdb "github.com/astianmuchui/mobilesonar/internal/redis"
	"github.com/astianmuchui/mobilesonar/internal/schemas"
)

const (
	StreamReadings = "sonar:readings"
	StreamEvents   = "sonar:events"
)

func PublishReading(ctx context.Context, req *schemas.IngestReadingRequest) error {
	return rdb.Client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamReadings,
		Values: map[string]any{
			"device_id":         req.DeviceID,
			"timestamp":         req.Timestamp,
			"lat":               req.GPS.Lat,
			"lon":               req.GPS.Lon,
			"gps_accuracy":      req.GPS.Accuracy,
			"distance_cm":       req.Ultrasonic.DistanceCM,
			"ultrasonic_valid":  fmt.Sprintf("%v", req.Ultrasonic.Valid),
			"accel_x":           req.MPU6050.AccelX,
			"accel_y":           req.MPU6050.AccelY,
			"accel_z":           req.MPU6050.AccelZ,
			"gyro_x":            req.MPU6050.GyroX,
			"gyro_y":            req.MPU6050.GyroY,
			"gyro_z":            req.MPU6050.GyroZ,
			"echo_amplitude":    req.Sonar.EchoAmplitude,
			"fish_detected":     fmt.Sprintf("%v", req.Sonar.FishDetected),
			"fish_count":        req.Sonar.FishCount,
			"fish_type":         req.Sonar.FishType,
			"battery_voltage":   req.System.BatteryVoltage,
			"signal_strength":   req.System.SignalStrength,
			"transmission_type": req.System.TransmissionType,
			"camera_reading_id": req.Camera.ReadingID,
		},
	}).Err()
}

func PublishEvent(ctx context.Context, values map[string]any) error {
	return rdb.Client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamEvents,
		Values: values,
	}).Err()
}
