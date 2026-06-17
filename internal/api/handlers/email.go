package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/novapanel/novapanel/internal/auth"
	"github.com/novapanel/novapanel/internal/db"
	"github.com/novapanel/novapanel/internal/system"
)

type EmailHandler struct {
	audit *system.AuditLogger
}

func NewEmailHandler() *EmailHandler {
	return &EmailHandler{audit: system.NewAuditLogger()}
}

func (h *EmailHandler) List(c *gin.Context) {
	domainID := c.Param("domain_id")

	var accounts []db.EmailAccount
	query := db.DB.Order("email ASC")

	if domainID != "" {
		id, err := strconv.ParseUint(domainID, 10, 32)
		if err == nil {
			var domain db.Domain
			if db.DB.First(&domain, id).Error == nil {
				role := auth.Role(c.GetString("role"))
				userID := c.GetUint("user_id")
				if role == auth.RoleClient && domain.UserID != userID {
					c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
					return
				}
				query = query.Where("domain_id = ?", id)
			}
		}
	}

	query.Find(&accounts)
	c.JSON(http.StatusOK, gin.H{"accounts": accounts})
}

type CreateEmailRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	DomainID uint   `json:"domain_id" binding:"required"`
	Quota    int64  `json:"quota"`
	Forward  string `json:"forward_to"`
}

func (h *EmailHandler) Create(c *gin.Context) {
	var req CreateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetUint("user_id")

	var existing db.EmailAccount
	if db.DB.Where("email = ?", req.Email).First(&existing).RowsAffected > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Email account already exists"})
		return
	}

	account := db.EmailAccount{
		DomainID:  req.DomainID,
		Email:     req.Email,
		Password:  req.Password,
		Quota:     req.Quota,
		ForwardTo: req.Forward,
	}

	if err := db.DB.Create(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create email account"})
		return
	}

	localPart := strings.Split(req.Email, "@")[0]
	domain := strings.Split(req.Email, "@")[1]

	postfixCmd := fmt.Sprintf("echo '%s: %s' >> /etc/postfix/virtual", req.Email, localPart)
	system.ExecuteBash(postfixCmd)
	system.Execute("postmap", "/etc/postfix/virtual")

	dovecotCmd := fmt.Sprintf("echo '%s:%s:%s:5000:5000:/home/%s:/bin/false' >> /etc/dovecot/users",
		req.Email, req.Password, localPart, domain)
	system.ExecuteBash(dovecotCmd)

	system.ServiceAction("postfix", "reload")
	system.ServiceAction("dovecot", "reload")

	username := c.GetString("username")
	h.audit.LogSuccess(&userID, username, "create_email", "email", "Created: "+req.Email, c.ClientIP())

	c.JSON(http.StatusCreated, account)
}

func (h *EmailHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email ID"})
		return
	}

	var account db.EmailAccount
	if err := db.DB.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}

	parts := strings.Split(account.Email, "@")
	if len(parts) == 2 {
		system.ExecuteBash(fmt.Sprintf("sed -i '/^%s:/d' /etc/postfix/virtual", account.Email))
		system.Execute("postmap", "/etc/postfix/virtual")
		system.ExecuteBash(fmt.Sprintf("sed -i '/^%s:/d' /etc/dovecot/users", account.Email))
		system.ServiceAction("postfix", "reload")
		system.ServiceAction("dovecot", "reload")
	}

	db.DB.Delete(&account)
	c.JSON(http.StatusOK, gin.H{"message": "Email account deleted"})
}

func (h *EmailHandler) UpdatePassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email ID"})
		return
	}

	var req struct {
		Password string `json:"password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var account db.EmailAccount
	if err := db.DB.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}

	db.DB.Model(&account).Update("password", req.Password)

	parts := strings.Split(account.Email, "@")
	localPart := parts[0]
	system.ExecuteBash(fmt.Sprintf("sed -i 's/^%s:.*/%s:%s/' /etc/dovecot/users",
		localPart, localPart, req.Password))
	system.ServiceAction("dovecot", "reload")

	c.JSON(http.StatusOK, gin.H{"message": "Password updated"})
}
