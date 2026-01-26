package models

import (
	"time"
)

// ServiceUser represents a sub-user with access to a specific service
type ServiceUser struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	ServiceID uint `gorm:"not null;index:idx_service_user" json:"service_id"`
	UserID    uint `gorm:"not null;index:idx_service_user" json:"user_id"`

	// Permissions
	CanViewConsole  bool `gorm:"default:true" json:"can_view_console"`
	CanSendCommands bool `gorm:"default:false" json:"can_send_commands"`
	CanManageFiles  bool `gorm:"default:false" json:"can_manage_files"`
	CanEditStartup  bool `gorm:"default:false" json:"can_edit_startup"`
	CanControlPower bool `gorm:"default:false" json:"can_control_power"`
	CanAccessSFTP   bool `gorm:"default:false" json:"can_access_sftp"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Service Service `json:"service,omitempty" gorm:"foreignKey:ServiceID"`
	User    User    `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// ActivityLog tracks all service-related actions for auditing
type ActivityLog struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	ServiceID uint `gorm:"not null;index" json:"service_id"`
	UserID    uint `gorm:"not null;index" json:"user_id"`

	// Action details
	Action      string `gorm:"size:50;not null;index" json:"action"` // power, sftp, file, startup, etc.
	Resource    string `gorm:"type:text" json:"resource"`            // File path, variable name, etc.
	Description string `gorm:"type:text" json:"description"`
	IPAddress   string `gorm:"size:45" json:"ip_address"`
	UserAgent   string `gorm:"type:text" json:"user_agent"`

	// Additional metadata (JSON)
	Metadata string `gorm:"type:text" json:"metadata"`

	CreatedAt time.Time `gorm:"index" json:"created_at"`

	// Relations
	Service Service `json:"service,omitempty" gorm:"foreignKey:ServiceID"`
	User    User    `json:"user,omitempty" gorm:"foreignKey:UserID"`
}
