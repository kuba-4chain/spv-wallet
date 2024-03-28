// Package common is a package that contains common models used by all other packages.
package common

import "time"

// Model is a common model that contains common fields for all models.
type Model struct {
	// CreatedAt is a time when outer model was created.
	CreatedAt time.Time `json:"created_at" example:"2024-02-26T11:00:28.069911Z"`
	// UpdatedAt is a time when outer model was updated.
	UpdatedAt time.Time `json:"updated_at" example:"2024-02-26T11:01:28.069911Z"`
	// DeletedAt is a time when outer model was deleted.
	DeletedAt time.Time `json:"deleted_at" example:"2024-02-26T11:02:28.069911Z"`
	// Metadata is a metadata map of outer model.
	Metadata map[string]interface{} `json:"metadata" swaggertype:"object,string" example:"key:value,key2:value2"`
}

type TimeRange struct {
	// From represents the start time of the time range.
	From time.Time `json:"from" example:"2024-02-26T11:00:28.069911Z"`
	// To represents the end time of the time range.
	To time.Time `json:"to" example:"2024-02-26T11:00:28.069911Z"`
}
