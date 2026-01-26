package models

import (
	"time"

	"gorm.io/gorm"
)

// [Deleted Nest, Egg, and EggVariable structs - moved to egg.go]

// Service defines a deployed instance (game server, app, etc.)
type Service struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	UUID string `gorm:"uniqueIndex;size:36;not null" json:"uuid"`
	Name string `gorm:"size:255;not null" json:"name"`

	UserID uint `gorm:"not null" json:"user_id"`
	User   User `json:"user" gorm:"foreignKey:UserID"`

	NodeID uint `gorm:"not null" json:"node_id"`
	Node   Node `json:"node" gorm:"foreignKey:NodeID"`

	// Link to Egg
	NestID uint `gorm:"default:null" json:"nest_id"` // Optional for legacy support initially
	Nest   Nest `json:"nest" gorm:"foreignKey:NestID"`

	EggID uint `gorm:"default:null" json:"egg_id"` // Optional for legacy support initially
	Egg   Egg  `json:"egg" gorm:"foreignKey:EggID"`

	// Resources
	Memory uint64 `gorm:"not null" json:"memory"` // MB
	Disk   uint64 `gorm:"not null" json:"disk"`   // MB
	Cpu    uint64 `gorm:"not null" json:"cpu"`    // % (100 = 1 core)

	// Network
	Port int `gorm:"not null" json:"port"`

	DockerImage string `gorm:"size:255" json:"docker_image"`

	IsSuspended          bool   `gorm:"default:false" json:"is_suspended"`
	Status               string `gorm:"default:'installing'" json:"status"` // installing, running, offline
	InstallationStage    string `gorm:"size:255;default:''" json:"installation_stage"`
	InstallationProgress int    `gorm:"default:0" json:"installation_progress"`

	Environment    string `gorm:"type:text" json:"environment"`     // JSON string of overrides
	VariableValues string `gorm:"type:text" json:"variable_values"` // JSON key-value pair of variable values

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
