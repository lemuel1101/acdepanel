package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/novapanel/novapanel/internal/auth"
	"github.com/novapanel/novapanel/internal/db"
	"github.com/novapanel/novapanel/internal/system"
)

type UserHandler struct {
	audit *system.AuditLogger
}

func NewUserHandler() *UserHandler {
	return &UserHandler{audit: system.NewAuditLogger()}
}

type CreateUserRequest struct {
	Username     string `json:"username" binding:"required,min=3,max=64"`
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
	Role         string `json:"role" binding:"required,oneof=admin reseller client"`
	MaxDomains   int    `json:"max_domains"`
	MaxDatabases int   `json:"max_databases"`
	MaxEmails    int    `json:"max_emails"`
	DiskLimit    int64  `json:"disk_limit"`
}

func (h *UserHandler) List(c *gin.Context) {
	var users []db.User
	query := db.DB.Order("created_at DESC")

	role := c.GetString("role")
	userID := c.GetUint("user_id")

	if auth.Role(role) != auth.RoleAdmin {
		query = query.Where("reseller_id = ? OR id = ?", userID, userID)
	}

	query.Find(&users)

	var resp []UserResponse
	for _, u := range users {
		resp = append(resp, userToResponse(&u))
	}

	c.JSON(http.StatusOK, gin.H{"users": resp})
}

func (h *UserHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user db.User
	if err := db.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, userToResponse(&user))
}

func (h *UserHandler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing db.User
	if db.DB.Where("username = ?", req.Username).First(&existing).RowsAffected > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := db.User{
		Username:     req.Username,
		Email:        req.Email,
		Password:     hashedPassword,
		Role:         req.Role,
		Status:       "active",
		MaxDomains:   req.MaxDomains,
		MaxDatabases: req.MaxDatabases,
		MaxEmails:    req.MaxEmails,
		DiskLimit:    req.DiskLimit,
	}

	if req.Role == string(auth.RoleClient) {
		resellerID := c.GetUint("user_id")
		user.ResellerID = &resellerID
	}

	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	username := c.GetString("username")
	h.audit.LogSuccess(nil, username, "create_user", "users", "Created user: "+req.Username, c.ClientIP())

	c.JSON(http.StatusCreated, userToResponse(&user))
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user db.User
	if err := db.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		Email        string `json:"email"`
		Role         string `json:"role"`
		Status       string `json:"status"`
		MaxDomains   *int   `json:"max_domains"`
		MaxDatabases *int  `json:"max_databases"`
		MaxEmails    *int   `json:"max_emails"`
		DiskLimit    *int64 `json:"disk_limit"`
		Password     string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	updates := map[string]interface{}{}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Role != "" {
		updates["role"] = req.Role
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.MaxDomains != nil {
		updates["max_domains"] = *req.MaxDomains
	}
	if req.MaxDatabases != nil {
		updates["max_databases"] = *req.MaxDatabases
	}
	if req.MaxEmails != nil {
		updates["max_emails"] = *req.MaxEmails
	}
	if req.DiskLimit != nil {
		updates["disk_limit"] = *req.DiskLimit
	}
	if req.Password != "" {
		hashedPassword, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		updates["password"] = hashedPassword
	}

	db.DB.Model(&user).Updates(updates)
	c.JSON(http.StatusOK, userToResponse(&user))
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user db.User
	if err := db.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	db.DB.Delete(&user)
	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}
