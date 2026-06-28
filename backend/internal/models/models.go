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
	Projects     []Project   `gorm:"foreignKey:UserID" json:"projects,omitempty"`
}

type Project struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Name      string    `gorm:"not null" json:"name"`
	RepoURL   string    `json:"repo_url"`
	Branch    string    `json:"branch"`
	Workspace string    `json:"workspace"`
	SandboxID uint      `json:"sandbox_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	User      User      `gorm:"foreignKey:UserID" json:"-"`
	Sandbox   *Sandbox  `gorm:"foreignKey:ProjectID" json:"sandbox,omitempty"`
}

type Sandbox struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	UserID        uint      `gorm:"not null;index" json:"user_id"`
	ProjectID     uint      `gorm:"not null;uniqueIndex" json:"project_id"`
	ContainerID   string    `gorm:"uniqueIndex;not null" json:"container_id"`
	Name          string    `gorm:"not null" json:"name"`
	Image         string    `gorm:"not null" json:"image"`
	Status        string    `json:"status"`
	WorkspacePath string    `json:"workspace_path"`
	VolumeName    string    `json:"volume_name"`
	NetworkName   string    `json:"network_name"`
	CPULimit      float64   `json:"cpu_limit"`
	MemoryLimit   int64     `json:"memory_limit"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	User          User      `gorm:"foreignKey:UserID" json:"-"`
	Project       *Project  `gorm:"foreignKey:ProjectID" json:"-"`
}

type AgentSession struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	ProjectID uint       `gorm:"not null;index" json:"project_id"`
	Name      string     `json:"name"`
	Status    string     `gorm:"not null;index" json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	User      User       `gorm:"foreignKey:UserID" json:"-"`
	Project   *Project   `gorm:"foreignKey:ProjectID" json:"-"`
	ToolCalls []ToolCall `gorm:"foreignKey:AgentSessionID" json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID             uint          `gorm:"primarykey" json:"id"`
	UserID         uint          `gorm:"not null;index" json:"user_id"`
	ProjectID      uint          `gorm:"not null;index" json:"project_id"`
	AgentSessionID uint          `gorm:"index" json:"agent_session_id"`
	ToolName       string        `gorm:"not null;index" json:"tool_name"`
	Status         string        `gorm:"not null;index" json:"status"`
	Input          string        `json:"input"`
	Output         string        `json:"output"`
	ExitCode       int           `json:"exit_code"`
	Error          string        `json:"error"`
	CreatedAt      time.Time     `json:"created_at"`
	User           User          `gorm:"foreignKey:UserID" json:"-"`
	Project        *Project      `gorm:"foreignKey:ProjectID" json:"-"`
	AgentSession   *AgentSession `gorm:"foreignKey:AgentSessionID" json:"-"`
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

func (Project) TableName() string {
	return "projects"
}

func (Sandbox) TableName() string {
	return "sandboxes"
}

func (AgentSession) TableName() string {
	return "agent_sessions"
}

func (ToolCall) TableName() string {
	return "tool_calls"
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

func (p *Project) BeforeCreate(tx *gorm.DB) error {
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	return nil
}

func (s *Sandbox) BeforeCreate(tx *gorm.DB) error {
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	return nil
}

func (s *AgentSession) BeforeCreate(tx *gorm.DB) error {
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	return nil
}

func (t *ToolCall) BeforeCreate(tx *gorm.DB) error {
	t.CreatedAt = time.Now()
	return nil
}

func (a *OperationAudit) BeforeCreate(tx *gorm.DB) error {
	a.CreatedAt = time.Now()
	return nil
}
