package http

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AddDeviceRequest struct {
	DeviceID        string `json:"device_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	PlainKey        string `json:"device_key"`
	FirmwareVersion string `json:"firmware_version"`
}

type RegisterRequest struct {
	FullName        string `json:"full_name" form:"full_name"`
	Email           string `json:"email" form:"email"`
	Password        string `json:"password" form:"password"`
	ConfirmPassword string `json:"confirm_password" form:"confirm_password"`
}

type AcknowledgeEventRequest struct {
	EventID string `json:"event_id"`
}

type FishProbabilityRequest struct {
	Lat    float64 `query:"lat"`
	Lon    float64 `query:"lon"`
	Radius float64 `query:"radius"`
	Days   int     `query:"days"`
}

type DensityMapRequest struct {
	Days int `query:"days"`
}

type TrendsRequest struct {
	Days int `query:"days"`
}

type WeatherRequest struct {
	Lat float64 `query:"lat"`
	Lon float64 `query:"lon"`
}
