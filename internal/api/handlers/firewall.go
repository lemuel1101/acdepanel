package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/novapanel/novapanel/internal/db"
	"github.com/novapanel/novapanel/internal/system"
)

type FirewallHandler struct {
	audit *system.AuditLogger
}

func NewFirewallHandler() *FirewallHandler {
	return &FirewallHandler{audit: system.NewAuditLogger()}
}

func (h *FirewallHandler) List(c *gin.Context) {
	var rules []db.FirewallRule
	db.DB.Order("port ASC").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

type CreateRuleRequest struct {
	Port        int    `json:"port" binding:"required,min=1,max=65535"`
	Protocol    string `json:"protocol"`
	Action      string `json:"action"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

func (h *FirewallHandler) Create(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Protocol == "" {
		req.Protocol = "tcp"
	}
	if req.Action == "" {
		req.Action = "allow"
	}
	if req.Source == "" {
		req.Source = "any"
	}

	rule := db.FirewallRule{
		Port:        req.Port,
		Protocol:    req.Protocol,
		Action:      req.Action,
		Source:      req.Source,
		Description: req.Description,
	}

	if err := db.DB.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create rule"})
		return
	}

	h.applyRule(&rule)
	username := c.GetString("username")
	userID := c.GetUint("user_id")
	h.audit.LogSuccess(&userID, username, "create_firewall_rule", "firewall",
		fmt.Sprintf("Port %d/%s - %s", req.Port, req.Protocol, req.Action), c.ClientIP())

	c.JSON(http.StatusCreated, rule)
}

func (h *FirewallHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule ID"})
		return
	}

	var rule db.FirewallRule
	if err := db.DB.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}

	h.removeRule(&rule)
	db.DB.Delete(&rule)
	c.JSON(http.StatusOK, gin.H{"message": "Rule deleted"})
}

func (h *FirewallHandler) Toggle(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule ID"})
		return
	}

	var rule db.FirewallRule
	if err := db.DB.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}

	rule.Enabled = !rule.Enabled
	db.DB.Save(&rule)

	if rule.Enabled {
		h.applyRule(&rule)
	} else {
		h.removeRule(&rule)
	}

	c.JSON(http.StatusOK, rule)
}

func (h *FirewallHandler) applyRule(rule *db.FirewallRule) {
	if rule.Action == "allow" {
		if rule.Source != "any" {
			system.Execute("ufw", "allow", "from", rule.Source, "to", "any", "port",
				strconv.Itoa(rule.Port), "proto", rule.Protocol)
		} else {
			system.Execute("ufw", "allow", strconv.Itoa(rule.Port)+"/"+rule.Protocol)
		}
	} else {
		if rule.Source != "any" {
			system.Execute("ufw", "deny", "from", rule.Source, "to", "any", "port",
				strconv.Itoa(rule.Port), "proto", rule.Protocol)
		} else {
			system.Execute("ufw", "deny", strconv.Itoa(rule.Port)+"/"+rule.Protocol)
		}
	}
}

func (h *FirewallHandler) removeRule(rule *db.FirewallRule) {
	if rule.Source != "any" {
		system.Execute("ufw", "delete", rule.Action, "from", rule.Source,
			"to", "any", "port", strconv.Itoa(rule.Port), "proto", rule.Protocol)
	} else {
		system.Execute("ufw", "delete", rule.Action, strconv.Itoa(rule.Port)+"/"+rule.Protocol)
	}
}

func (h *FirewallHandler) Status(c *gin.Context) {
	result := system.Execute("ufw", "status", "verbose")
	status := "inactive"
	if result.Success {
		status = result.Stdout
	}

	c.JSON(http.StatusOK, gin.H{
		"status": status,
	})
}

func (h *FirewallHandler) ToggleFirewall(c *gin.Context) {
	username := c.GetString("username")
	userID := c.GetUint("user_id")

	var req struct {
		Enabled bool `json:"enabled"`
	}
	c.ShouldBindJSON(&req)

	if req.Enabled {
		system.ExecuteBash("echo y | ufw enable")
		h.audit.LogSuccess(&userID, username, "enable_firewall", "firewall", "Firewall enabled", c.ClientIP())
		c.JSON(http.StatusOK, gin.H{"message": "Firewall enabled"})
	} else {
		system.Execute("ufw", "disable")
		h.audit.LogSuccess(&userID, username, "disable_firewall", "firewall", "Firewall disabled", c.ClientIP())
		c.JSON(http.StatusOK, gin.H{"message": "Firewall disabled"})
	}
}
