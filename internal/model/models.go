package model

import "time"

type Endpoint struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	URL             string    `json:"url"`
	IntervalSeconds int       `json:"interval_seconds"`
	FailThreshold   int       `json:"fail_threshold"`
	Paused          bool      `json:"paused"`
	CreatedAt       time.Time `json:"created_at"`
	NotifierIDs     []int64   `json:"notifier_ids"` // For API request/response
}

type Check struct {
	ID             int64     `json:"id"`
	EndpointID     int64     `json:"endpoint_id"`
	StatusCode     *int      `json:"status_code"`
	ResponseTimeMs int       `json:"response_time_ms"`
	IsUp           bool      `json:"is_up"`
	CheckedAt      time.Time `json:"checked_at"`
}

type EndpointWithStats struct {
	Endpoint
	StatusCode     *int       `json:"status_code"`
	ResponseTimeMs *int       `json:"response_time_ms"`
	IsUp           *bool      `json:"is_up"`
	CheckedAt      *time.Time `json:"checked_at"`
	Uptime30d      *float64   `json:"uptime_30d"`
}

type Notifier struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`        // e.g., 'TELEGRAM', 'PUSHOVER'
	ConfigJSON string    `json:"config_json"` // JSON string containing credentials/config
	CreatedAt  time.Time `json:"created_at"`
}

type EndpointNotifier struct {
	EndpointID int64 `json:"endpoint_id"`
	NotifierID int64 `json:"notifier_id"`
}
