package models

import (
	"time"

	"gorm.io/gorm"
)

// Nest defines a category for eggs (e.g., "Source Engine", "Minecraft")
type Nest struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UUID        string `gorm:"uniqueIndex;size:36;not null" json:"uuid"`
	Name        string `gorm:"size:255;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Eggs        []Egg  `json:"eggs" gorm:"foreignKey:NestID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Egg defines a game configuration (e.g., "Garry's Mod", "Minecraft")
type Egg struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	UUID           string `gorm:"uniqueIndex;size:36;not null" json:"uuid"`
	NestID         uint   `gorm:"not null" json:"nest_id"`
	Nest           Nest   `json:"nest" gorm:"foreignKey:NestID"`
	Name           string `gorm:"size:255;not null" json:"name"`
	Description    string `gorm:"type:text" json:"description"`
	DockerImages   string `gorm:"type:text" json:"docker_images"` // JSON string: ["image1", "image2"]
	StartupCommand string `gorm:"type:text;not null" json:"startup_command"`
	StopCommand    string `gorm:"size:255" json:"stop_command"`
	ConfigFiles    string `gorm:"type:text" json:"config_files"` // JSON string: [{"path": "...", "search": "...", "replace": "..."}]

	InstallScript     string `gorm:"type:text" json:"install_script"`
	InstallContainer  string `gorm:"size:255" json:"install_container"`
	InstallEntrypoint string `gorm:"size:255" json:"install_entrypoint"`

	Variables []EggVariable `json:"variables" gorm:"foreignKey:EggID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EggVariable defines a configurable variable for an egg
type EggVariable struct {
	ID                  uint   `gorm:"primaryKey" json:"id"`
	EggID               uint   `gorm:"not null" json:"egg_id"`
	Name                string `gorm:"size:255;not null" json:"name"`
	Description         string `gorm:"type:text" json:"description"`
	EnvironmentVariable string `gorm:"size:255;not null" json:"environment_variable"`
	DefaultValue        string `gorm:"type:text" json:"default_value"`
	UserViewable        bool   `gorm:"default:true" json:"user_viewable"`
	UserEditable        bool   `gorm:"default:true" json:"user_editable"`
	Rules               string `gorm:"type:text" json:"rules"`
	InputType           string `gorm:"size:50;default:'text'" json:"input_type"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Service defines a deployed instance (game server, app, etc.)
type Service struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	UUID string `gorm:"uniqueIndex;size:36;not null" json:"uuid"`
	Name string `gorm:"size:255;not null" json:"name"`

	UserID uint `gorm:"not null" json:"user_id"`
	User   User `json:"user" gorm:"foreignKey:UserID"`

	NodeID uint `gorm:"not null" json:"node_id"`
	Node   Node `json:"node" gorm:"foreignKey:NodeID"`

	EggID uint `gorm:"not null" json:"egg_id"`
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
	Environment          string `gorm:"type:text" json:"environment"` // JSON string of overrides

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
