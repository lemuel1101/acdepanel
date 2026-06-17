package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/novapanel/novapanel/internal/db"
	"github.com/novapanel/novapanel/internal/system"
)

type SystemHandler struct {
	upgrader websocket.Upgrader
}

func NewSystemHandler() *SystemHandler {
	return &SystemHandler{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *SystemHandler) Stats(c *gin.Context) {
	cpu := system.GetCPUUsage()
	mem := system.GetMemoryInfo()
	diskTotal, diskUsed, diskFree, _ := system.DiskUsage("/")
	osInfo := system.GetOSInfo()

	var totalDomains, activeDomains, totalUsers int64
	db.DB.Model(&db.Domain{}).Count(&totalDomains)
	db.DB.Model(&db.Domain{}).Where("status = ?", "active").Count(&activeDomains)
	db.DB.Model(&db.User{}).Count(&totalUsers)

	uptime := system.Execute("uptime")
	loadAvg := ""
	if uptime.Success {
		parts := strings.Split(uptime.Stdout, "load average:")
		if len(parts) == 2 {
			loadAvg = strings.TrimSpace(parts[1])
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"cpu": gin.H{
			"usage_percent": cpu,
			"load_average":  loadAvg,
		},
		"memory": mem,
		"disk": gin.H{
			"total": diskTotal,
			"used":  diskUsed,
			"free":  diskFree,
		},
		"os": osInfo,
		"counts": gin.H{
			"total_domains":  totalDomains,
			"active_domains": activeDomains,
			"total_users":    totalUsers,
		},
		"uptime": uptime.Stdout,
	})
}

func (h *SystemHandler) Services(c *gin.Context) {
	services := []string{"nginx", "mysql", "postfix", "dovecot", "ufw", "certbot", "php8.1-fpm", "php8.2-fpm", "php8.3-fpm"}

	var result []gin.H
	for _, svc := range services {
		running := system.ServiceStatus(svc)
		unitResult := system.Execute("systemctl", "show", "-p", "ActiveEnterTimestamp,Description", svc, "--no-pager")

		desc := svc
		startTime := "N/A"
		if unitResult.Success {
			for _, line := range strings.Split(unitResult.Stdout, "\n") {
				if strings.HasPrefix(line, "Description=") {
					desc = strings.TrimPrefix(line, "Description=")
				}
				if strings.HasPrefix(line, "ActiveEnterTimestamp=") {
					startTime = strings.TrimPrefix(line, "ActiveEnterTimestamp=")
				}
			}
		}

		result = append(result, gin.H{
			"name":        svc,
			"running":     running,
			"status":      map[bool]string{true: "running", false: "stopped"}[running],
			"started_at":  startTime,
			"description": desc,
		})
	}

	c.JSON(http.StatusOK, gin.H{"services": result})
}

func (h *SystemHandler) ServiceAction(c *gin.Context) {
	name := c.Param("name")
	action := c.Param("action")

	validActions := map[string]bool{"start": true, "stop": true, "restart": true, "reload": true}
	if !validActions[action] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
		return
	}

	result := system.ServiceAction(name, action)
	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Stderr})
		return
	}

	username := c.GetString("username")
	userID := c.GetUint("user_id")
	system.NewAuditLogger().LogSuccess(&userID, username, "service_action", "services",
		fmt.Sprintf("%s %s", action, name), c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("%s %sed successfully", name, action)})
}

func (h *SystemHandler) Logs(c *gin.Context) {
	service := c.DefaultQuery("service", "novapanel")
	lines := 100

	result := system.Execute("journalctl", "-u", service, "--no-pager", "-n", fmt.Sprintf("%d", lines), "-q")
	logs := ""
	if result.Success {
		logs = result.Stdout
	}

	c.JSON(http.StatusOK, gin.H{
		"service": service,
		"logs":    logs,
		"lines":   lines,
	})
}

func (h *SystemHandler) Processes(c *gin.Context) {
	result := system.ExecuteBash("ps aux --sort=-%cpu | head -50")
	processes := ""
	if result.Success {
		processes = result.Stdout
	}

	c.JSON(http.StatusOK, gin.H{"processes": processes})
}

func (h *SystemHandler) Websocket(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cpu := system.GetCPUUsage()
			mem := system.GetMemoryInfo()
			diskTotal, diskUsed, diskFree, _ := system.DiskUsage("/")

			msg := gin.H{
				"type": "stats",
				"data": gin.H{
					"cpu":  cpu,
					"mem":  mem,
					"disk": gin.H{"total": diskTotal, "used": diskUsed, "free": diskFree},
					"time": time.Now().Unix(),
				},
			}

			if err := conn.WriteJSON(msg); err != nil {
				return
			}
		}
	}
}

func (h *SystemHandler) DiskUsage(c *gin.Context) {
	path := c.DefaultQuery("path", "/home")
	result := system.ExecuteBash(fmt.Sprintf("du -sh %s/* 2>/dev/null | sort -rh | head -20", path))

	var usage []gin.H
	if result.Success {
		for _, line := range strings.Split(result.Stdout, "\n") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				usage = append(usage, gin.H{
					"size": parts[0],
					"path": strings.Join(parts[1:], " "),
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"usage": usage})
}

func (h *SystemHandler) AuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	offset := (page - 1) * pageSize

	var logs []db.AuditLog
	db.DB.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs)

	var total int64
	db.DB.Model(&db.AuditLog{}).Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"logs":        logs,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (int(total) + pageSize - 1) / pageSize,
	})
}
