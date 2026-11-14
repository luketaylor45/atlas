package models

import "time"

// Setting stores instance-wide values that Atlas manages itself, such as the
// generated signing secret.
type Setting struct {
	Key       string `gorm:"primaryKey;size:100" json:"key"`
	Value     string `gorm:"type:text;not null" json:"-"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
