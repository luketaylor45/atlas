package api

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/daemon/internal/config"
	"github.com/luketaylor45/atlas/daemon/internal/serverfs"
)

type FileInfo struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
	Mime  string `json:"mime"`
	// Modified is the last modification time in RFC 3339, empty if unknown.
	Modified string `json:"modified"`
}

const (
	maxZipEntries       = 100_000
	maxZipExtractedSize = uint64(20 << 30) // 20 GiB
)

var errInvalidZipArchive = errors.New("invalid zip archive")

type archiveFileOperations struct {
	ensureDirectory func(string) error
	normalizePath   func(string) error
}

var managedArchiveFiles = archiveFileOperations{
	ensureDirectory: serverfs.EnsureDirectory,
	normalizePath:   serverfs.NormalizePath,
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func securePath(uuid string, subPath string) (string, error) {
	dataRoot := filepath.Clean(config.NodeConfig.DataPath)
	dataDir := filepath.Clean(filepath.Join(dataRoot, uuid))
	if !pathWithin(dataRoot, dataDir) {
		return "", fmt.Errorf("invalid path: directory traversal attempt")
	}

	normalizedSubPath := strings.ReplaceAll(subPath, "\\", "/")
	cleanPath := filepath.FromSlash(strings.TrimLeft(normalizedSubPath, "/"))
	fullPath := filepath.Clean(filepath.Join(dataDir, cleanPath))
	if !pathWithin(dataDir, fullPath) {
		return "", fmt.Errorf("invalid path: directory traversal attempt")
	}

	return fullPath, nil
}

func zipEntryTarget(destinationRoot, entryName string) (string, error) {
	normalizedName := strings.ReplaceAll(entryName, "\\", "/")
	if normalizedName == "" || strings.ContainsRune(normalizedName, '\x00') || path.IsAbs(normalizedName) {
		return "", fmt.Errorf("%w: unsafe entry path %q", errInvalidZipArchive, entryName)
	}

	cleanName := path.Clean(normalizedName)
	if cleanName == "." {
		return destinationRoot, nil
	}
	if cleanName == ".." || strings.HasPrefix(cleanName, "../") {
		return "", fmt.Errorf("%w: unsafe entry path %q", errInvalidZipArchive, entryName)
	}

	target := filepath.Clean(filepath.Join(destinationRoot, filepath.FromSlash(cleanName)))
	if !pathWithin(destinationRoot, target) {
		return "", fmt.Errorf("%w: unsafe entry path %q", errInvalidZipArchive, entryName)
	}
	return target, nil
}

func rejectSymlinkComponents(root, target string) error {
	if !pathWithin(root, target) {
		return fmt.Errorf("%w: extraction target escapes its directory", errInvalidZipArchive)
	}

	relative, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("inspect extraction path: %w", err)
	}

	current := root
	components := []string{"."}
	if relative != "." {
		components = append(components, strings.Split(relative, string(os.PathSeparator))...)
	}
	for _, component := range components {
		if component != "." {
			current = filepath.Join(current, component)
		}
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return fmt.Errorf("inspect extraction path: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: extraction path contains a symbolic link", errInvalidZipArchive)
		}
	}
	return nil
}

func extractZipArchive(archivePath, destinationRoot string, files archiveFileOperations) (int, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, fmt.Errorf("%w: cannot open archive", errInvalidZipArchive)
	}
	defer archive.Close()

	if len(archive.File) > maxZipEntries {
		return 0, fmt.Errorf("%w: archive contains more than %d entries", errInvalidZipArchive, maxZipEntries)
	}

	targets := make([]string, len(archive.File))
	targetKinds := make(map[string]bool, len(archive.File))
	var declaredSize uint64
	for index, entry := range archive.File {
		if entry.Mode()&os.ModeSymlink != 0 || (!entry.FileInfo().IsDir() && !entry.Mode().IsRegular()) {
			return 0, fmt.Errorf("%w: archive contains an unsupported special file", errInvalidZipArchive)
		}
		if entry.UncompressedSize64 > maxZipExtractedSize-declaredSize {
			return 0, fmt.Errorf("%w: expanded archive exceeds 20 GiB", errInvalidZipArchive)
		}
		declaredSize += entry.UncompressedSize64

		target, targetErr := zipEntryTarget(destinationRoot, entry.Name)
		if targetErr != nil {
			return 0, targetErr
		}
		if target == destinationRoot && !entry.FileInfo().IsDir() {
			return 0, fmt.Errorf("%w: archive contains an invalid root file", errInvalidZipArchive)
		}
		if target == destinationRoot {
			targets[index] = target
			continue
		}
		if target == archivePath {
			return 0, fmt.Errorf("%w: archive cannot overwrite itself", errInvalidZipArchive)
		}
		if _, duplicate := targetKinds[target]; duplicate {
			return 0, fmt.Errorf("%w: archive contains duplicate entry %q", errInvalidZipArchive, entry.Name)
		}
		if symlinkErr := rejectSymlinkComponents(destinationRoot, target); symlinkErr != nil {
			return 0, symlinkErr
		}
		if existing, statErr := os.Lstat(target); statErr == nil {
			if existing.IsDir() != entry.FileInfo().IsDir() {
				return 0, fmt.Errorf("%w: archive entry conflicts with existing path %q", errInvalidZipArchive, entry.Name)
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return 0, fmt.Errorf("inspect extraction target: %w", statErr)
		}
		targets[index] = target
		targetKinds[target] = entry.FileInfo().IsDir()
	}
	for target := range targetKinds {
		for parent := filepath.Dir(target); parent != destinationRoot && parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
			if isDirectory, exists := targetKinds[parent]; exists && !isDirectory {
				return 0, fmt.Errorf("%w: archive file conflicts with a nested entry", errInvalidZipArchive)
			}
		}
	}

	extracted := 0
	var writtenSize uint64
	for index, entry := range archive.File {
		target := targets[index]
		if target == destinationRoot {
			continue
		}
		if entry.FileInfo().IsDir() {
			if err := files.ensureDirectory(target); err != nil {
				return extracted, fmt.Errorf("create archive directory: %w", err)
			}
			extracted++
			continue
		}

		if err := files.ensureDirectory(filepath.Dir(target)); err != nil {
			return extracted, fmt.Errorf("create archive parent directory: %w", err)
		}
		if err := rejectSymlinkComponents(destinationRoot, target); err != nil {
			return extracted, err
		}

		source, err := entry.Open()
		if err != nil {
			return extracted, fmt.Errorf("open archive entry: %w", err)
		}
		mode := entry.Mode().Perm() | 0o660
		destination, err := os.CreateTemp(filepath.Dir(target), ".atlas-extract-*")
		if err != nil {
			source.Close()
			return extracted, fmt.Errorf("create temporary extracted file: %w", err)
		}
		temporaryPath := destination.Name()

		remaining := maxZipExtractedSize - writtenSize
		written, copyErr := io.Copy(destination, io.LimitReader(source, int64(remaining)+1))
		closeDestinationErr := destination.Close()
		closeSourceErr := source.Close()
		if written < 0 || uint64(written) > remaining {
			_ = os.Remove(temporaryPath)
			return extracted, fmt.Errorf("%w: expanded archive exceeds 20 GiB", errInvalidZipArchive)
		}
		writtenSize += uint64(written)
		if copyErr != nil {
			_ = os.Remove(temporaryPath)
			return extracted, fmt.Errorf("extract archive entry: %w", copyErr)
		}
		if closeDestinationErr != nil {
			_ = os.Remove(temporaryPath)
			return extracted, fmt.Errorf("close extracted file: %w", closeDestinationErr)
		}
		if closeSourceErr != nil {
			_ = os.Remove(temporaryPath)
			return extracted, fmt.Errorf("close archive entry: %w", closeSourceErr)
		}
		if err := os.Chmod(temporaryPath, mode); err != nil {
			_ = os.Remove(temporaryPath)
			return extracted, fmt.Errorf("set extracted file mode: %w", err)
		}
		if err := files.normalizePath(temporaryPath); err != nil {
			_ = os.Remove(temporaryPath)
			return extracted, fmt.Errorf("normalize extracted file permissions: %w", err)
		}
		if err := rejectSymlinkComponents(destinationRoot, target); err != nil {
			_ = os.Remove(temporaryPath)
			return extracted, err
		}
		if err := os.Rename(temporaryPath, target); err != nil {
			_ = os.Remove(temporaryPath)
			return extracted, fmt.Errorf("publish extracted file: %w", err)
		}
		extracted++
	}

	return extracted, nil
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

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		file := FileInfo{Name: entry.Name(), IsDir: entry.IsDir()}
		// A file can vanish between listing and stat; it is still listed,
		// just without its size and time.
		if info, err := entry.Info(); err == nil {
			file.Size = info.Size()
			file.Modified = info.ModTime().UTC().Format(time.RFC3339)
		}
		files = append(files, file)
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
	if err := serverfs.EnsureDirectory(filepath.Dir(fullPath)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory structure"})
		return
	}

	// Note: We might want to handle Windows line endings if the user is on Windows editing files for Linux containers
	// But let's assume they want the raw content for now.
	if err := serverfs.WriteFile(fullPath, []byte(req.Content), 0660); err != nil {
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

	if err := serverfs.EnsureDirectory(fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create folder: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func ExtractArchive(c *gin.Context) {
	token := c.GetHeader("X-Node-Token")
	if token != config.NodeConfig.NodeToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uuid := c.Param("uuid")
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A ZIP archive path is required"})
		return
	}
	if !strings.EqualFold(filepath.Ext(req.Path), ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only ZIP archives can be extracted"})
		return
	}

	archivePath, err := securePath(uuid, req.Path)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	serviceRoot, err := securePath(uuid, "")
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if err := rejectSymlinkComponents(serviceRoot, archivePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	archiveInfo, err := os.Lstat(archivePath)
	if errors.Is(err, os.ErrNotExist) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Archive not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to inspect archive: " + err.Error()})
		return
	}
	if !archiveInfo.Mode().IsRegular() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Archive must be a regular ZIP file"})
		return
	}

	extracted, err := extractZipArchive(archivePath, filepath.Dir(archivePath), managedArchiveFiles)
	if errors.Is(err, errInvalidZipArchive) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extract archive: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "extracted": extracted})
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

	fullPath, err := securePath(uuid, filepath.Join(subPath, filepath.Base(file.Filename)))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if err := serverfs.EnsureDirectory(filepath.Dir(fullPath)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory: " + err.Error()})
		return
	}

	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file: " + err.Error()})
		return
	}
	if err := serverfs.NormalizePath(fullPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set uploaded file permissions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
