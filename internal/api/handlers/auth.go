package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/novapanel/novapanel/internal/auth"
	"github.com/novapanel/novapanel/internal/config"
	"github.com/novapanel/novapanel/internal/db"
	"github.com/novapanel/novapanel/internal/system"
)

type AuthHandler struct {
	cfg   *config.Config
	audit *system.AuditLogger
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		cfg:   cfg,
		audit: system.NewAuditLogger(),
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token *auth.TokenPair `json:"token"`
	User  UserResponse    `json:"user"`
}

type UserResponse struct {
	ID           uint       `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	MaxDomains   int        `json:"max_domains"`
	MaxDatabases int        `json:"max_databases"`
	MaxEmails    int        `json:"max_emails"`
	DiskLimit    int64      `json:"disk_limit"`
	TwoFactor    bool       `json:"two_factor"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
}

func userToResponse(u *db.User) UserResponse {
	return UserResponse{
		ID:           u.ID,
		Username:     u.Username,
		Email:        u.Email,
		Role:         u.Role,
		Status:       u.Status,
		MaxDomains:   u.MaxDomains,
		MaxDatabases: u.MaxDatabases,
		MaxEmails:    u.MaxEmails,
		DiskLimit:    u.DiskLimit,
		TwoFactor:    u.TwoFactor,
		LastLogin:    u.LastLogin,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)

	var user db.User
	if err := db.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		h.audit.LogFailure(nil, req.Username, "login", "auth", "User not found", c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if user.Status != "active" {
		h.audit.LogFailure(&user.ID, user.Username, "login", "auth", "Account inactive", c.ClientIP())
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is inactive"})
		return
	}

	if !auth.CheckPassword(req.Password, user.Password) {
		h.audit.LogFailure(&user.ID, user.Username, "login", "auth", "Invalid password", c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	tokenPair, err := auth.GenerateTokenPair(&h.cfg.JWT, user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	now := time.Now()
	db.DB.Model(&user).Update("last_login", &now)

	h.audit.LogSuccess(&user.ID, user.Username, "login", "auth", "Successful login", c.ClientIP())

	c.JSON(http.StatusOK, LoginResponse{
		Token: tokenPair,
		User:  userToResponse(&user),
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	tokenPair, err := auth.RefreshAccessToken(req.RefreshToken, &h.cfg.JWT)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenPair})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetUint("user_id")

	var user db.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, userToResponse(&user))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	userID := c.GetUint("user_id")
	username := c.GetString("username")
	h.audit.LogSuccess(&userID, username, "logout", "auth", "User logged out", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.GetUint("user_id")
	username := c.GetString("username")

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var user db.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if !auth.CheckPassword(req.CurrentPassword, user.Password) {
		h.audit.LogFailure(&userID, username, "change_password", "auth", "Incorrect current password", c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is incorrect"})
		return
	}

	hashedPassword, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	db.DB.Model(&user).Update("password", hashedPassword)
	h.audit.LogSuccess(&userID, username, "change_password", "auth", "Password changed", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}
