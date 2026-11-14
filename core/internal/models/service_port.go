package models

import "time"

// PrimaryPortEnv is the variable that carries a service's primary port.
const PrimaryPortEnv = "SERVER_PORT"

// ServicePort is one host port allocated to a service. Every port a service
// publishes, including its primary one, has a row here, and the unique index
// makes it impossible for two services on a node to hold the same port.
type ServicePort struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	ServiceID uint `gorm:"not null;index" json:"service_id"`
	NodeID    uint `gorm:"not null;uniqueIndex:idx_service_ports_node_port" json:"node_id"`
	Port      int  `gorm:"not null;uniqueIndex:idx_service_ports_node_port" json:"port"`

	// Env is the environment variable the game reads this port from.
	Env      string `gorm:"size:255;not null" json:"env"`
	Name     string `gorm:"size:255" json:"name"`
	Protocol string `gorm:"size:10;not null;default:'both'" json:"protocol"` // tcp, udp or both
	// IsPrimary marks the port stored on the service itself and exposed as SERVER_PORT.
	IsPrimary bool `gorm:"not null;default:false" json:"primary"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EggPort is an additional port an egg needs besides the primary one, such as
// a query or RCON port. Eggs store a JSON list of these.
type EggPort struct {
	Name     string `json:"name"`
	Env      string `json:"env"`
	Default  int    `json:"default"`
	Protocol string `json:"protocol"`
}
