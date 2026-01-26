package models

import (
	"time"
)

// Nest defines a category for eggs (e.g., "Game Servers" or "Garry's Mod")
type Nest struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UUID        string `gorm:"uniqueIndex;size:36;not null" json:"uuid"`
	Code        string `gorm:"size:50" json:"code"` // slug: "source", "minecraft"
	Name        string `gorm:"size:255;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`

	// Sub-Nest Support
	ParentID *uint `json:"parent_id"`
	Parent   *Nest `json:"parent,omitempty" gorm:"foreignKey:ParentID"`

	// Constraint: Deleting a Nest deletes all its sub-nests and Eggs
	SubNests []Nest `json:"sub_nests" gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE"`
	Eggs     []Egg  `json:"eggs" gorm:"foreignKey:NestID;constraint:OnDelete:CASCADE"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Egg defines a game configuration (e.g., "Garry's Mod", "Minecraft")
type Egg struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UUID        string `gorm:"uniqueIndex;size:36;not null" json:"uuid"`
	NestID      uint   `gorm:"not null" json:"nest_id"`
	Nest        Nest   `json:"nest" gorm:"foreignKey:NestID"`
	Author      string `gorm:"size:255;not null" json:"author"`
	Name        string `gorm:"size:255;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`

	DockerImage     string `gorm:"type:text;not null" json:"docker_image"`
	DockerImages    string `gorm:"type:text" json:"docker_images"` // JSON List of available images
	StartupCommand  string `gorm:"type:text;not null" json:"startup_command"`
	StopCommand     string `gorm:"size:255" json:"stop_command"`
	Config          string `gorm:"type:text" json:"config"` // JSON string for config file replacements
	ScriptInstall   string `gorm:"type:text" json:"script_install"`
	ScriptEntry     string `gorm:"size:255" json:"script_entry"`
	ScriptContainer string `gorm:"size:255" json:"script_container"`
	ScriptCovers    bool   `gorm:"default:false" json:"script_covers"` // If true, script runs on every start? (Legacy compat)

	// Constraint: Deleting an Egg deletes all its Variables
	Variables []EggVariable `json:"variables" gorm:"foreignKey:EggID;constraint:OnDelete:CASCADE"`

	// Constraint: Deleting an Egg should probably NOT delete Servers automatically for safety,
	// but the user asked to fix deletion. Let's make it CASCADE for now to satisfy "Delete Nest cleans everything"
	Servers []Service `json:"servers" gorm:"foreignKey:EggID;constraint:OnDelete:CASCADE"`

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
	Rules               string `gorm:"type:text" json:"rules"` // Validation rules (e.g. required|string)
	InputType           string `gorm:"size:50;default:'text'" json:"input_type"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
