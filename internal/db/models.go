package db

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Email        string         `gorm:"uniqueIndex;size:255" json:"email"`
	Password     string         `gorm:"size:255;not null" json:"-"`
	Role         string         `gorm:"size:32;not null;default:client" json:"role"`
	Status       string         `gorm:"size:32;not null;default:active" json:"status"`
	ResellerID   *uint          `json:"reseller_id,omitempty"`
	MaxDomains   int            `gorm:"default:-1" json:"max_domains"`
	MaxDatabases int            `gorm:"default:-1" json:"max_databases"`
	MaxEmails    int            `gorm:"default:-1" json:"max_emails"`
	DiskLimit    int64          `gorm:"default:-1" json:"disk_limit"`
	LastLogin    *time.Time     `json:"last_login,omitempty"`
	TwoFactor    bool           `gorm:"default:false" json:"two_factor"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type Domain struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"index;not null" json:"user_id"`
	DomainName    string         `gorm:"uniqueIndex;size:255;not null" json:"domain_name"`
	DocumentRoot  string         `gorm:"size:512" json:"document_root"`
	NginxTemplate string         `gorm:"size:64;default:default" json:"nginx_template"`
	PHPVersion    string         `gorm:"size:16;default:8.1" json:"php_version"`
	SSLEnabled    bool           `gorm:"default:false" json:"ssl_enabled"`
	SSLStatus     string         `gorm:"size:32;default:pending" json:"ssl_status"`
	RedirectURL   string         `gorm:"size:512" json:"redirect_url,omitempty"`
	RedirectType  int            `gorm:"default:0" json:"redirect_type"`
	Status        string         `gorm:"size:32;default:active" json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

type DomainAlias struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	DomainID  uint           `gorm:"index;not null" json:"domain_id"`
	Alias     string         `gorm:"uniqueIndex;size:255;not null" json:"alias"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Database struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"index;not null" json:"user_id"`
	DomainID  *uint          `json:"domain_id,omitempty"`
	Name      string         `gorm:"uniqueIndex;size:255;not null" json:"name"`
	DBType    string         `gorm:"size:32;default:mysql" json:"db_type"`
	Charset   string         `gorm:"size:64;default:utf8mb4" json:"charset"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type DatabaseUser struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	DatabaseID uint           `gorm:"index;not null" json:"database_id"`
	Username   string         `gorm:"size:255;not null" json:"username"`
	Password   string         `gorm:"size:255;not null" json:"-"`
	Privileges string         `gorm:"size:64;default:ALL" json:"privileges"`
	CreatedAt  time.Time      `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

type EmailAccount struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	DomainID     uint           `gorm:"index;not null" json:"domain_id"`
	Email        string         `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Password     string         `gorm:"size:255;not null" json:"-"`
	Quota        int64          `gorm:"default:0" json:"quota"`
	ForwardTo    string         `gorm:"size:512" json:"forward_to,omitempty"`
	Autoresponder bool          `gorm:"default:false" json:"autoresponder"`
	ARSubject    string         `gorm:"size:255" json:"ar_subject,omitempty"`
	ARBody       string         `gorm:"type:text" json:"ar_body,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type FirewallRule struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Port        int       `gorm:"not null" json:"port"`
	Protocol    string    `gorm:"size:8;default:tcp" json:"protocol"`
	Action      string    `gorm:"size:16;default:allow" json:"action"`
	Source      string    `gorm:"size:64;default:any" json:"source"`
	Description string    `gorm:"size:255" json:"description"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Backup struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      *uint      `json:"user_id,omitempty"`
	DomainID    *uint      `json:"domain_id,omitempty"`
	Type        string     `gorm:"size:32;not null" json:"type"`
	Filename    string     `gorm:"size:512;not null" json:"filename"`
	Size        int64      `json:"size"`
	Status      string     `gorm:"size:32;default:pending" json:"status"`
	DestType    string     `gorm:"size:32;default:local" json:"dest_type"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type CronJob struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     uint           `gorm:"index;not null" json:"user_id"`
	DomainID   *uint          `json:"domain_id,omitempty"`
	Command    string         `gorm:"type:text;not null" json:"command"`
	Schedule   string         `gorm:"size:128;not null" json:"schedule"`
	Enabled    bool           `gorm:"default:true" json:"enabled"`
	LastRun    *time.Time     `json:"last_run,omitempty"`
	LastOutput string         `gorm:"type:text" json:"last_output,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

type SSLRequest struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	DomainID     uint           `gorm:"index;not null" json:"domain_id"`
	Domains      string         `gorm:"type:text;not null" json:"domains"`
	Status       string         `gorm:"size:32;default:pending" json:"status"`
	ErrorMessage string         `gorm:"type:text" json:"error_message,omitempty"`
	ExpiresAt    *time.Time     `json:"expires_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    *uint     `json:"user_id,omitempty"`
	Username  string    `gorm:"size:128" json:"username"`
	Action    string    `gorm:"size:255;not null" json:"action"`
	Resource  string    `gorm:"size:255" json:"resource"`
	Detail    string    `gorm:"type:text" json:"detail,omitempty"`
	IP        string    `gorm:"size:64" json:"ip"`
	Status    string    `gorm:"size:32" json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Setting struct {
	Key   string `gorm:"primaryKey;size:255" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}
