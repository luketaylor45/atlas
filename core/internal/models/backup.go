package models

import "time"

// Backup statuses.
const (
	BackupCreating  = "creating"
	BackupCompleted = "completed"
	BackupFailed    = "failed"
	BackupRestoring = "restoring"
)

// Backup is an archive of a service's files, stored on the service's node.
type Backup struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UUID      string `gorm:"uniqueIndex;size:36;not null" json:"uuid"`
	ServiceID uint   `gorm:"not null;index" json:"service_id"`
	Name      string `gorm:"size:255;not null" json:"name"`

	Status   string `gorm:"size:20;not null;index" json:"status"`
	Size     int64  `gorm:"not null;default:0" json:"size"` // bytes
	Checksum string `gorm:"size:80" json:"checksum"`        // "sha256:<hex>"
	// Error explains a failed backup, or a restore of it that failed.
	Error string `gorm:"type:text" json:"error"`
	// Automated backups are made by schedules and are rotated: when the
	// limit is reached the oldest one makes room for the next.
	Automated bool `gorm:"not null;default:false" json:"automated"`

	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// Schedule actions.
const (
	ScheduleActionPower   = "power"
	ScheduleActionCommand = "command"
	ScheduleActionBackup  = "backup"
)

// Schedule runs one action on a service at the times a cron expression names.
type Schedule struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ServiceID uint   `gorm:"not null;index" json:"service_id"`
	Name      string `gorm:"size:255;not null" json:"name"`

	// Cron is a five-field expression: minute, hour, day of month, month,
	// day of week. It is read in Core's time zone.
	Cron string `gorm:"size:120;not null" json:"cron"`

	// Action is "power", "command" or "backup". Payload is the power action
	// ("restart"), the console command, or the name for the backup.
	Action  string `gorm:"size:20;not null" json:"action"`
	Payload string `gorm:"type:text" json:"payload"`

	// No database default: "false" must be storable on insert.
	Enabled bool `gorm:"not null" json:"enabled"`
	// OnlyWhenOnline skips a run while the server is not running.
	OnlyWhenOnline bool `gorm:"not null;default:false" json:"only_when_online"`

	NextRunAt   *time.Time `gorm:"index" json:"next_run_at"`
	LastRunAt   *time.Time `json:"last_run_at"`
	LastStatus  string     `gorm:"size:20" json:"last_status"` // success, failed or skipped
	LastMessage string     `gorm:"type:text" json:"last_message"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
