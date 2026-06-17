package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/novapanel/novapanel/internal/auth"
	"github.com/novapanel/novapanel/internal/db"
	"github.com/novapanel/novapanel/internal/system"
)

type DatabaseHandler struct {
	audit *system.AuditLogger
}

func NewDatabaseHandler() *DatabaseHandler {
	return &DatabaseHandler{audit: system.NewAuditLogger()}
}

func (h *DatabaseHandler) List(c *gin.Context) {
	var databases []db.Database
	query := db.DB.Order("created_at DESC")

	role := auth.Role(c.GetString("role"))
	userID := c.GetUint("user_id")

	if role == auth.RoleClient {
		query = query.Where("user_id = ?", userID)
	}

	query.Find(&databases)
	c.JSON(http.StatusOK, gin.H{"databases": databases})
}

type CreateDBRequest struct {
	Name     string `json:"name" binding:"required"`
	DBType   string `json:"db_type"`
	Charset  string `json:"charset"`
	DomainID *uint  `json:"domain_id"`
}

func (h *DatabaseHandler) Create(c *gin.Context) {
	var req CreateDBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Name = strings.ToLower(strings.ReplaceAll(req.Name, "-", "_"))
	if req.DBType == "" {
		req.DBType = "mysql"
	}
	if req.Charset == "" {
		req.Charset = "utf8mb4"
	}

	userID := c.GetUint("user_id")

	database := db.Database{
		UserID:   userID,
		Name:     req.Name,
		DBType:   req.DBType,
		Charset:  req.Charset,
		DomainID: req.DomainID,
	}

	if err := db.DB.Create(&database).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Database name already exists"})
		return
	}

	result := system.Execute("mysql", "-e",
		"CREATE DATABASE IF NOT EXISTS `"+req.Name+"` CHARACTER SET "+req.Charset)
	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create database: " + result.Stderr})
		return
	}

	username := c.GetString("username")
	h.audit.LogSuccess(&userID, username, "create_database", "databases", "Created: "+req.Name, c.ClientIP())

	c.JSON(http.StatusCreated, database)
}

func (h *DatabaseHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid database ID"})
		return
	}

	var database db.Database
	if err := db.DB.First(&database, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Database not found"})
		return
	}

	system.Execute("mysql", "-e", "DROP DATABASE IF EXISTS `"+database.Name+"`")
	db.DB.Delete(&database)

	c.JSON(http.StatusOK, gin.H{"message": "Database deleted"})
}

func (h *DatabaseHandler) CreateUser(c *gin.Context) {
	dbID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid database ID"})
		return
	}

	var req struct {
		Username   string `json:"username" binding:"required"`
		Password   string `json:"password" binding:"required,min=8"`
		Privileges string `json:"privileges"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Privileges == "" {
		req.Privileges = "ALL"
	}

	dbUser := db.DatabaseUser{
		DatabaseID: uint(dbID),
		Username:   req.Username,
		Password:   req.Password,
		Privileges: req.Privileges,
	}

	if err := db.DB.Create(&dbUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	system.Execute("mysql", "-e",
		"CREATE USER IF NOT EXISTS '"+req.Username+"'@'localhost' IDENTIFIED BY '"+req.Password+"'")
	system.Execute("mysql", "-e",
		"GRANT "+req.Privileges+" ON `"+req.Username+"`.* TO '"+req.Username+"'@'localhost'")
	system.Execute("mysql", "-e", "FLUSH PRIVILEGES")

	c.JSON(http.StatusCreated, dbUser)
}

func (h *DatabaseHandler) DeleteUser(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("userId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var dbUser db.DatabaseUser
	if err := db.DB.First(&dbUser, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Database user not found"})
		return
	}

	system.Execute("mysql", "-e", "DROP USER IF EXISTS '"+dbUser.Username+"'@'localhost'")
	system.Execute("mysql", "-e", "FLUSH PRIVILEGES")
	db.DB.Delete(&dbUser)

	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}
