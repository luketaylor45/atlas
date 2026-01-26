package models

import (
	"time"
)

type News struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Title     string    `gorm:"size:255;not null" json:"title"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	Type      string    `gorm:"size:50;not null" json:"type"` // e.g. "Release", "Status", "Maintenance"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
