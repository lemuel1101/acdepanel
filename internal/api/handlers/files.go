package handlers

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/novapanel/novapanel/internal/system"
)

type FileHandler struct{}

func NewFileHandler() *FileHandler {
	return &FileHandler{}
}

func (h *FileHandler) getBasePath(c *gin.Context) string {
	username := c.GetString("username")
	role := c.GetString("role")

	basePath := "/home/" + username
	if role == "admin" {
		basePath = c.DefaultQuery("path", "/home")
	}

	return basePath
}

func (h *FileHandler) checkPath(basePath, reqPath string) (string, error) {
	fullPath := filepath.Join(basePath, reqPath)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, basePath) {
		return "", fmt.Errorf("path traversal detected")
	}

	return fullPath, nil
}

type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

func (h *FileHandler) List(c *gin.Context) {
	basePath := h.getBasePath(c)
	reqPath := c.DefaultQuery("path", "/")

	fullPath, err := h.checkPath(basePath, reqPath)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Directory not found"})
		return
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(reqPath, entry.Name()),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode().Perm().String(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(http.StatusOK, gin.H{"files": files, "path": reqPath})
}

func (h *FileHandler) Read(c *gin.Context) {
	basePath := h.getBasePath(c)
	reqPath := c.Query("path")

	fullPath, err := h.checkPath(basePath, reqPath)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": string(content),
		"path":    reqPath,
	})
}

func (h *FileHandler) Write(c *gin.Context) {
	basePath := h.getBasePath(c)

	var req struct {
		Path    string `json:"path" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	fullPath, err := h.checkPath(basePath, req.Path)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directories"})
		return
	}

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write file"})
		return
	}

	userID := c.GetUint("user_id")
	username := c.GetString("username")
	system.NewAuditLogger().LogSuccess(&userID, username, "write_file", "files", "Wrote: "+req.Path, c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"message": "File saved"})
}

func (h *FileHandler) Delete(c *gin.Context) {
	basePath := h.getBasePath(c)

	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	fullPath, err := h.checkPath(basePath, req.Path)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := os.RemoveAll(fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

func (h *FileHandler) Upload(c *gin.Context) {
	basePath := h.getBasePath(c)
	destPath := c.DefaultPostForm("path", "/")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	fullPath, err := h.checkPath(basePath, filepath.Join(destPath, header.Filename))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File uploaded"})
}

func (h *FileHandler) CreateDir(c *gin.Context) {
	basePath := h.getBasePath(c)

	var req struct {
		Path string `json:"path" binding:"required"`
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	fullPath, err := h.checkPath(basePath, filepath.Join(req.Path, req.Name))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Directory created"})
}

func (h *FileHandler) Chmod(c *gin.Context) {
	basePath := h.getBasePath(c)

	var req struct {
		Path string `json:"path" binding:"required"`
		Mode string `json:"mode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	fullPath, err := h.checkPath(basePath, req.Path)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	mode, err := strconv.ParseInt(req.Mode, 8, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid mode"})
		return
	}

	if err := os.Chmod(fullPath, os.FileMode(mode)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permissions updated"})
}

func (h *FileHandler) Zip(c *gin.Context) {
	basePath := h.getBasePath(c)

	var req struct {
		Path    string   `json:"path" binding:"required"`
		ZipName string   `json:"zip_name"`
		Files   []string `json:"files"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.ZipName == "" {
		req.ZipName = "archive.zip"
	}

	zipPath, err := h.checkPath(basePath, filepath.Join(req.Path, req.ZipName))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	zipFile, err := os.Create(zipPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create zip file"})
		return
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	for _, file := range req.Files {
		srcPath, err := h.checkPath(basePath, filepath.Join(req.Path, file))
		if err != nil {
			continue
		}

		info, err := os.Stat(srcPath)
		if err != nil {
			continue
		}

		if info.IsDir() {
			filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				relPath, _ := filepath.Rel(filepath.Join(basePath, req.Path), path)
				header, _ := zip.FileInfoHeader(info)
				header.Name = relPath
				if info.IsDir() {
					header.Name += "/"
				} else {
					header.Method = zip.Deflate
				}
				writer, _ := zw.CreateHeader(header)
				if !info.IsDir() {
					data, _ := os.ReadFile(path)
					writer.Write(data)
				}
				return nil
			})
		} else {
			header, _ := zip.FileInfoHeader(info)
			header.Name = file
			header.Method = zip.Deflate
			writer, _ := zw.CreateHeader(header)
			data, _ := os.ReadFile(srcPath)
			writer.Write(data)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Archive created", "zip": req.ZipName})
}

func (h *FileHandler) Unzip(c *gin.Context) {
	basePath := h.getBasePath(c)

	var req struct {
		Path    string `json:"path" binding:"required"`
		ZipName string `json:"zip_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	zipPath, err := h.checkPath(basePath, filepath.Join(req.Path, req.ZipName))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	destPath, err := h.checkPath(basePath, req.Path)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to open zip file"})
		return
	}
	defer reader.Close()

	for _, file := range reader.File {
		fullPath := filepath.Join(destPath, file.Name)
		if !strings.HasPrefix(fullPath, destPath) {
			continue
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(fullPath, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(fullPath), 0755)

		rc, err := file.Open()
		if err != nil {
			continue
		}
		defer rc.Close()

		dst, err := os.Create(fullPath)
		if err != nil {
			continue
		}
		defer dst.Close()

		io.Copy(dst, rc)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Archive extracted"})
}
