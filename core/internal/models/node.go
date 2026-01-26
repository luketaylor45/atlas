package models

import (
	"time"

	"gorm.io/gorm"
)

type Node struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"size:255;not null" json:"name"`
	Address  string `gorm:"not null" json:"address"` // IP or Domain
	Port     string `gorm:"default:'8081'" json:"port"`
	SFTPPort string `gorm:"default:'2022'" json:"sftp_port"`
	Token    string `gorm:"uniqueIndex;not null" json:"-"`

	// Resources
	TotalRAM  uint64  `gorm:"not null;default:0" json:"total_ram"`  // In MB
	TotalDisk uint64  `gorm:"not null;default:0" json:"total_disk"` // In MB
	UsedRAM   uint64  `gorm:"default:0" json:"used_ram"`
	UsedCPU   float64 `gorm:"default:0" json:"used_cpu"`

	// Location
	Location string `gorm:"size:255;default:'Unknown'" json:"location"` // Ensure simple string for now, maybe JSON later

	IsOnline      bool           `gorm:"default:false" json:"is_online"`
	LastHeartbeat time.Time      `json:"last_heartbeat"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}
