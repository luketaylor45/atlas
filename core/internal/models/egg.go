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
	Available   bool   `gorm:"default:true;index" json:"available"`
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

	// Ports is a JSON list of EggPort: the additional ports this game needs
	// besides its primary one (query, RCON, ...).
	Ports string `gorm:"type:text" json:"ports"`

	// Source is "catalog" for eggs loaded from the eggs directory and
	// "import" for eggs added through the panel.
	Source string `gorm:"size:20;default:''" json:"source"`
	// SeedHash is the hash of the catalog file this egg was last loaded from.
	SeedHash string `gorm:"size:64;default:''" json:"-"`
	// Customized is set when an admin edits a catalog egg. The seeder then
	// leaves it alone instead of overwriting the edit on the next restart.
	Customized bool `gorm:"default:false" json:"customized"`

	// Constraint: Deleting an Egg deletes all its Variables
	Variables []EggVariable `json:"variables" gorm:"foreignKey:EggID;constraint:OnDelete:CASCADE"`

	// Services are never removed by deleting an egg: an egg that is still in
	// use cannot be deleted at all.
	Servers []Service `json:"servers" gorm:"foreignKey:EggID;constraint:OnDelete:RESTRICT"`

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
