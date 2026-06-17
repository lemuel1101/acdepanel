package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/novapanel/novapanel/internal/auth"
	"github.com/novapanel/novapanel/internal/config"
	"github.com/novapanel/novapanel/internal/db"
	"github.com/novapanel/novapanel/internal/system"
)

type BackupHandler struct {
	cfg   *config.Config
	audit *system.AuditLogger
}

func NewBackupHandler(cfg *config.Config) *BackupHandler {
	return &BackupHandler{cfg: cfg, audit: system.NewAuditLogger()}
}

func (h *BackupHandler) List(c *gin.Context) {
	var backups []db.Backup
	query := db.DB.Order("created_at DESC")

	role := auth.Role(c.GetString("role"))
	userID := c.GetUint("user_id")

	if role == auth.RoleClient {
		query = query.Where("user_id = ?", userID)
	}

	query.Find(&backups)
	c.JSON(http.StatusOK, gin.H{"backups": backups})
}

type CreateBackupRequest struct {
	Type     string `json:"type" binding:"required,oneof=full site database"`
	DomainID *uint  `json:"domain_id"`
	DestType string `json:"dest_type"`
}

func (h *BackupHandler) Create(c *gin.Context) {
	var req CreateBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DestType == "" {
		req.DestType = "local"
	}

	userID := c.GetUint("user_id")
	filename := fmt.Sprintf("backup_%s_%d.tar.gz", req.Type, time.Now().Unix())
	backupPath := filepath.Join(h.cfg.Backup.Dir, filename)

	os.MkdirAll(h.cfg.Backup.Dir, 0755)

	backup := db.Backup{
		UserID:   &userID,
		DomainID: req.DomainID,
		Type:     req.Type,
		Filename: filename,
		Status:   "running",
		DestType: req.DestType,
	}

	db.DB.Create(&backup)

	go h.runBackup(&backup, backupPath, req)

	c.JSON(http.StatusAccepted, backup)
}

func (h *BackupHandler) runBackup(backup *db.Backup, path string, req CreateBackupRequest) {
	var result *system.ExecResult

	switch req.Type {
	case "full":
		result = system.Execute("tar", "-czf", path,
			"/etc/nginx", "/etc/novapanel", "/var/lib/novapanel",
			"/home", "/var/log/novapanel")
	case "site":
		if req.DomainID != nil {
			var domain db.Domain
			if db.DB.First(&domain, *req.DomainID).Error == nil {
				result = system.Execute("tar", "-czf", path, domain.DocumentRoot)
			}
		}
	case "database":
		result = system.ExecuteBash(fmt.Sprintf("mysqldump --all-databases > %s", path))
	}

	now := time.Now()
	if result != nil && result.Success {
		info, _ := os.Stat(path)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		db.DB.Model(backup).Updates(map[string]interface{}{
			"status":       "completed",
			"size":         size,
			"completed_at": &now,
		})
	} else {
		db.DB.Model(backup).Updates(map[string]interface{}{
			"status": "failed",
		})
	}
}

func (h *BackupHandler) Download(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup ID"})
		return
	}

	var backup db.Backup
	if err := db.DB.First(&backup, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
		return
	}

	backupPath := filepath.Join(h.cfg.Backup.Dir, backup.Filename)
	c.File(backupPath)
}

func (h *BackupHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup ID"})
		return
	}

	var backup db.Backup
	if err := db.DB.First(&backup, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
		return
	}

	backupPath := filepath.Join(h.cfg.Backup.Dir, backup.Filename)
	os.Remove(backupPath)
	db.DB.Delete(&backup)

	c.JSON(http.StatusOK, gin.H{"message": "Backup deleted"})
}

func (h *BackupHandler) Settings(c *gin.Context) {
	var req struct {
		RetentionDays int    `json:"retention_days"`
		S3Bucket      string `json:"s3_bucket"`
		S3Region      string `json:"s3_region"`
		S3AccessKey   string `json:"s3_access_key"`
		S3SecretKey   string `json:"s3_secret_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.RetentionDays > 0 {
		h.cfg.Backup.RetentionDays = req.RetentionDays
	}
	if req.S3Bucket != "" {
		h.cfg.Backup.S3Bucket = req.S3Bucket
	}
	if req.S3Region != "" {
		h.cfg.Backup.S3Region = req.S3Region
	}
	if req.S3AccessKey != "" {
		h.cfg.Backup.S3AccessKey = req.S3AccessKey
	}
	if req.S3SecretKey != "" {
		h.cfg.Backup.S3SecretKey = req.S3SecretKey
	}

	username := c.GetString("username")
	userID := c.GetUint("user_id")
	h.audit.LogSuccess(&userID, username, "update_backup_settings", "backups", "Backup settings updated", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"message": "Backup settings updated"})
}
