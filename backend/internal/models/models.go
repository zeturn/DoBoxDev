package models

import (
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID           uint        `gorm:"primarykey" json:"id"`
	Username     string      `gorm:"uniqueIndex;not null" json:"username"`
	Email        string      `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string      `gorm:"not null" json:"-"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	Containers   []Container `gorm:"foreignKey:UserID" json:"containers,omitempty"`
}

type Container struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	UserID        uint      `gorm:"not null;index" json:"user_id"`
	ContainerID   string    `gorm:"uniqueIndex;not null" json:"container_id"` // Docker container ID
	Name          string    `gorm:"not null" json:"name"`
	Image         string    `gorm:"not null" json:"image"`
	Status        string    `json:"status"`   // running, stopped, paused, etc.
	Ports         string    `json:"ports"`    // JSON string of port mappings
	EnvVars       string    `json:"env_vars"` // JSON string of environment variables
	Volumes       string    `json:"volumes"`  // JSON string of volume bindings hostPath:containerPath[:ro|rw]
	Command       string    `json:"command"`  // JSON string array of command args
	WorkingDir    string    `json:"working_dir"`
	RestartPolicy string    `json:"restart_policy"` // no, always, unless-stopped, on-failure
	NetworkMode   string    `json:"network_mode"`   // bridge, host, none, container:<name|id>
	CPULimit      float64   `json:"cpu_limit"`      // CPU limit in cores
	MemoryLimit   int64     `json:"memory_limit"`   // Memory limit in bytes
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	User          User      `gorm:"foreignKey:UserID" json:"-"`
}

type OperationAudit struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	ContainerID uint      `gorm:"not null;index" json:"container_id"`
	Action      string    `gorm:"not null;index" json:"action"`
	Status      string    `gorm:"not null;index" json:"status"` // success | failed
	Detail      string    `json:"detail"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName overrides the table name
func (User) TableName() string {
	return "users"
}

func (Container) TableName() string {
	return "containers"
}

func (OperationAudit) TableName() string {
	return "operation_audits"
}

// BeforeCreate hook
func (u *User) BeforeCreate(tx *gorm.DB) error {
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	return nil
}

func (c *Container) BeforeCreate(tx *gorm.DB) error {
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	return nil
}

func (a *OperationAudit) BeforeCreate(tx *gorm.DB) error {
	a.CreatedAt = time.Now()
	return nil
}
