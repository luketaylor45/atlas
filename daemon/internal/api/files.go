package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/daemon/internal/config"
)

type FileInfo struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
	Mime  string `json:"mime"`
}

// securePath ensures the requested path is within the server's data directory
func securePath(uuid string, subPath string) (string, error) {
	dataDir := fmt.Sprintf("C:\\AtlasData\\%s", uuid)

	// Clean the subPath to prevent traversal (.. stuff)
	cleanPath := filepath.Join("/", subPath)
	fullPath := filepath.Join(dataDir, cleanPath)

	// Ensure the result is still within dataDir
	if !strings.HasPrefix(fullPath, dataDir) {
		return "", fmt.Errorf("invalid path: directory traversal attempt")
	}

	return fullPath, nil
}

func ListFiles(c *gin.Context) {
	token := c.GetHeader("X-Node-Token")
	if token != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uuid := c.Param("uuid")
	subPath := c.Query("path")

	fullPath, err := securePath(uuid, subPath)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read directory: " + err.Error()})
		return
	}

	var files []FileInfo
	for _, entry := range entries {
		info, _ := entry.Info()
		files = append(files, FileInfo{
			Name:  entry.Name(),
			Size:  info.Size(),
			IsDir: entry.IsDir(),
			Mime:  "", // Optional: could detect mime here
		})
	}

	// Sort: Directories first, then alphabetical
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	c.JSON(http.StatusOK, files)
}

func GetFileContent(c *gin.Context) {
	token := c.GetHeader("X-Node-Token")
	if token != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uuid := c.Param("uuid")
	subPath := c.Query("path")

	fullPath, err := securePath(uuid, subPath)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file: " + err.Error()})
		return
	}

	c.String(http.StatusOK, string(content))
}

func WriteFile(c *gin.Context) {
	token := c.GetHeader("X-Node-Token")
	if token != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uuid := c.Param("uuid")
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	fullPath, err := securePath(uuid, req.Path)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory structure"})
		return
	}

	// Note: We might want to handle Windows line endings if the user is on Windows editing files for Linux containers
	// But let's assume they want the raw content for now.
	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write file: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func DeleteFile(c *gin.Context) {
	token := c.GetHeader("X-Node-Token")
	if token != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uuid := c.Param("uuid")
	subPath := c.Query("path")

	if subPath == "" || subPath == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete root directory"})
		return
	}

	fullPath, err := securePath(uuid, subPath)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := os.RemoveAll(fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func CreateFolder(c *gin.Context) {
	token := c.GetHeader("X-Node-Token")
	if token != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uuid := c.Param("uuid")
	var req struct {
		Path string `json:"path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	fullPath, err := securePath(uuid, req.Path)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create folder: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func UploadFile(c *gin.Context) {
	token := c.GetHeader("X-Node-Token")
	if token != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uuid := c.Param("uuid")
	subPath := c.Query("path")

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	fullPath, err := securePath(uuid, filepath.Join(subPath, file.Filename))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
