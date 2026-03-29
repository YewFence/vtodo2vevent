package model

import "time"

type Event struct {
	Path          string
	ETag          string
	UID           string
	Summary       string
	Description   string
	StartAt       time.Time
	EndAt         time.Time
	SourceTodoUID string
	Raw           string
}
