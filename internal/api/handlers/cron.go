package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/novapanel/novapanel/internal/auth"
	"github.com/novapanel/novapanel/internal/db"
	"github.com/novapanel/novapanel/internal/system"
)

type CronHandler struct {
	audit *system.AuditLogger
}

func NewCronHandler() *CronHandler {
	return &CronHandler{audit: system.NewAuditLogger()}
}

func (h *CronHandler) List(c *gin.Context) {
	var jobs []db.CronJob
	query := db.DB.Order("created_at DESC")

	role := auth.Role(c.GetString("role"))
	userID := c.GetUint("user_id")

	if role == auth.RoleClient {
		query = query.Where("user_id = ?", userID)
	}

	query.Find(&jobs)
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

type CreateCronRequest struct {
	Command  string `json:"command" binding:"required"`
	Schedule string `json:"schedule" binding:"required"`
	DomainID *uint  `json:"domain_id"`
}

func (h *CronHandler) Create(c *gin.Context) {
	var req CreateCronRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetUint("user_id")
	username := c.GetString("username")

	job := db.CronJob{
		UserID:   userID,
		Command:  req.Command,
		Schedule: req.Schedule,
		DomainID: req.DomainID,
		Enabled:  true,
	}

	if err := db.DB.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cron job"})
		return
	}

	h.writeCrontab(username)
	h.audit.LogSuccess(&userID, username, "create_cron", "cron",
		fmt.Sprintf("Schedule: %s, Command: %s", req.Schedule, req.Command), c.ClientIP())

	c.JSON(http.StatusCreated, job)
}

func (h *CronHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cron job ID"})
		return
	}

	var job db.CronJob
	if err := db.DB.First(&job, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cron job not found"})
		return
	}

	var req struct {
		Command  string `json:"command"`
		Schedule string `json:"schedule"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	updates := map[string]interface{}{}
	if req.Command != "" {
		updates["command"] = req.Command
	}
	if req.Schedule != "" {
		updates["schedule"] = req.Schedule
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	db.DB.Model(&job).Updates(updates)
	h.writeCrontab(c.GetString("username"))

	c.JSON(http.StatusOK, job)
}

func (h *CronHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cron job ID"})
		return
	}

	var job db.CronJob
	if err := db.DB.First(&job, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cron job not found"})
		return
	}

	username := c.GetString("username")
	db.DB.Delete(&job)
	h.writeCrontab(username)

	c.JSON(http.StatusOK, gin.H{"message": "Cron job deleted"})
}

func (h *CronHandler) writeCrontab(username string) {
	var jobs []db.CronJob
	var user db.User
	if err := db.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return
	}

	db.DB.Where("user_id = ? AND enabled = ?", user.ID, true).Find(&jobs)

	cronContent := "# NovaPanel Cron Jobs\n"
	for _, job := range jobs {
		cronContent += fmt.Sprintf("%s %s\n", job.Schedule, job.Command)
	}

	cronFile := fmt.Sprintf("/var/spool/cron/crontabs/%s", username)
	os.MkdirAll(filepath.Dir(cronFile), 0755)
	os.WriteFile(cronFile, []byte(cronContent), 0600)
	system.Execute("crontab", cronFile)
}
