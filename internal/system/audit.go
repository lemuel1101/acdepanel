package system

import (
	"github.com/novapanel/novapanel/internal/db"
)

type AuditLogger struct{}

func NewAuditLogger() *AuditLogger {
	return &AuditLogger{}
}

func (a *AuditLogger) Log(userID *uint, username, action, resource, detail, ip, status string) {
	entry := db.AuditLog{
		UserID:   userID,
		Username: username,
		Action:   action,
		Resource: resource,
		Detail:   detail,
		IP:       ip,
		Status:   status,
	}

	if db.DB != nil {
		db.DB.Create(&entry)
	}
}

func (a *AuditLogger) LogSuccess(userID *uint, username, action, resource, detail, ip string) {
	a.Log(userID, username, action, resource, detail, ip, "success")
}

func (a *AuditLogger) LogFailure(userID *uint, username, action, resource, detail, ip string) {
	a.Log(userID, username, action, resource, detail, ip, "failure")
}
