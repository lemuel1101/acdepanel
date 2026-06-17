package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/novapanel/novapanel/internal/auth"
	"github.com/novapanel/novapanel/internal/config"
	"github.com/novapanel/novapanel/internal/db"
	"github.com/novapanel/novapanel/internal/hosting"
	"github.com/novapanel/novapanel/internal/system"
)

type DomainHandler struct {
	cfg   *config.Config
	nginx *hosting.NginxManager
	ssl   *hosting.SSLManager
	audit *system.AuditLogger
}

func NewDomainHandler(cfg *config.Config) *DomainHandler {
	return &DomainHandler{
		cfg:   cfg,
		nginx: hosting.NewNginxManager(cfg),
		ssl:   hosting.NewSSLManager(cfg),
		audit: system.NewAuditLogger(),
	}
}

func (h *DomainHandler) List(c *gin.Context) {
	var domains []db.Domain
	query := db.DB.Order("created_at DESC")

	role := auth.Role(c.GetString("role"))
	userID := c.GetUint("user_id")

	if role == auth.RoleClient {
		query = query.Where("user_id = ?", userID)
	}

	query.Find(&domains)
	c.JSON(http.StatusOK, gin.H{"domains": domains})
}

func (h *DomainHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	var domain db.Domain
	if err := db.DB.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	c.JSON(http.StatusOK, domain)
}

type CreateDomainRequest struct {
	DomainName    string `json:"domain_name" binding:"required,fqdn"`
	PHPVersion    string `json:"php_version"`
	NginxTemplate string `json:"nginx_template"`
}

func (h *DomainHandler) Create(c *gin.Context) {
	var req CreateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.DomainName = strings.ToLower(strings.TrimSpace(req.DomainName))
	userID := c.GetUint("user_id")

	var existing db.Domain
	if db.DB.Where("domain_name = ?", req.DomainName).First(&existing).RowsAffected > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Domain already exists"})
		return
	}

	docRoot := filepath.Join(h.cfg.System.HomeDirPrefix, c.GetString("username"), "public_html", req.DomainName)
	docRoot = strings.ReplaceAll(docRoot, "//", "/")

	if req.PHPVersion == "" {
		req.PHPVersion = "8.1"
	}

	domain := db.Domain{
		UserID:        userID,
		DomainName:    req.DomainName,
		DocumentRoot:  docRoot,
		PHPVersion:    req.PHPVersion,
		NginxTemplate: req.NginxTemplate,
		Status:        "active",
	}

	if err := db.DB.Create(&domain).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create domain"})
		return
	}

	if err := os.MkdirAll(docRoot, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create document root"})
		return
	}

	indexHTML := fmt.Sprintf(`<!DOCTYPE html><html><head><title>%s</title></head><body><h1>Welcome to %s</h1><p>Site is being set up.</p></body></html>`, req.DomainName, req.DomainName)
	os.WriteFile(filepath.Join(docRoot, "index.html"), []byte(indexHTML), 0644)

	if err := h.nginx.CreateVHost(&domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to configure nginx: %s", err.Error())})
		return
	}

	username := c.GetString("username")
	h.audit.LogSuccess(&userID, username, "create_domain", "domains", "Created domain: "+req.DomainName, c.ClientIP())

	c.JSON(http.StatusCreated, domain)
}

func (h *DomainHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	var domain db.Domain
	if err := db.DB.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	username := c.GetString("username")
	userID := c.GetUint("user_id")

	h.nginx.RemoveVHost(&domain)
	db.DB.Delete(&domain)

	h.audit.LogSuccess(&userID, username, "delete_domain", "domains", "Deleted domain: "+domain.DomainName, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"message": "Domain deleted"})
}

func (h *DomainHandler) EnableSSL(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	var domain db.Domain
	if err := db.DB.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	domainName := domain.DomainName
	if err := h.ssl.Obtain(domainName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("SSL failed: %s", err.Error())})
		return
	}

	db.DB.Model(&domain).Updates(map[string]interface{}{
		"ssl_enabled": true,
		"ssl_status":  "active",
	})

	h.nginx.EnableSSL(&domain)

	username := c.GetString("username")
	userID := c.GetUint("user_id")
	h.audit.LogSuccess(&userID, username, "enable_ssl", "domains", "Enabled SSL for: "+domainName, c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"message": "SSL enabled successfully", "domain": domain})
}

func (h *DomainHandler) SetPHP(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	var req struct {
		PHPVersion string `json:"php_version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var domain db.Domain
	if err := db.DB.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	db.DB.Model(&domain).Update("php_version", req.PHPVersion)
	h.nginx.UpdatePHP(&domain, req.PHPVersion)

	c.JSON(http.StatusOK, gin.H{"message": "PHP version updated"})
}

type RedirectRequest struct {
	URL  string `json:"url" binding:"required"`
	Type int    `json:"type"`
}

func (h *DomainHandler) SetRedirect(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	var req RedirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.Type == 0 {
		req.Type = 301
	}

	var domain db.Domain
	if err := db.DB.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	db.DB.Model(&domain).Updates(map[string]interface{}{
		"redirect_url":  req.URL,
		"redirect_type": req.Type,
	})

	h.nginx.SetRedirect(&domain, req.URL, req.Type)
	c.JSON(http.StatusOK, gin.H{"message": "Redirect configured"})
}

func (h *DomainHandler) GetLogs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	var domain db.Domain
	if err := db.DB.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	lines := 100
	accessLog := fmt.Sprintf("/var/log/nginx/%s.access.log", domain.DomainName)
	errorLog := fmt.Sprintf("/var/log/nginx/%s.error.log", domain.DomainName)

	accessData := system.TailFile(accessLog, lines)
	errorData := system.TailFile(errorLog, lines)

	c.JSON(http.StatusOK, gin.H{
		"domain": domain.DomainName,
		"access": accessData,
		"error":  errorData,
	})
}
