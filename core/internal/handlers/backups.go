package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/middleware"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
	"gorm.io/gorm"
)

// maxBackupLimit bounds what an admin can allow per server.
const maxBackupLimit = 50

// backupDownloadHTTP has no overall timeout: an archive can take a long time
// to stream. The connection is still bounded by the browser staying connected.
var backupDownloadHTTP = &http.Client{}

// serviceForMember loads a service for the caller and applies a sub-user
// permission check. Owners and admins always pass.
func serviceForMember(c *gin.Context, allowed func(*models.ServiceUser) bool, denied string) (*models.Service, bool) {
	userID := c.MustGet("user_id").(uint)
	service, subUser, ok := utils.FindServiceForUser(c.Param("uuid"), userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return nil, false
	}
	if subUser != nil && !allowed(subUser) {
		c.JSON(http.StatusForbidden, gin.H{"error": denied})
		return nil, false
	}
	return service, true
}

func serviceForBackups(c *gin.Context) (*models.Service, bool) {
	return serviceForMember(c, func(subUser *models.ServiceUser) bool { return subUser.CanManageBackups },
		"You do not have permission to manage this server's backups")
}

func findBackup(c *gin.Context, service *models.Service) (*models.Backup, bool) {
	var backup models.Backup
	if err := database.DB.Where("uuid = ? AND service_id = ?", c.Param("backup"), service.ID).First(&backup).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
		return nil, false
	}
	return &backup, true
}

func backupNodePath(service *models.Service, backup *models.Backup, suffix string) string {
	return "/api/servers/" + service.UUID + "/backups/" + backup.UUID + suffix
}

// ListBackups returns a service's backups, newest first, with its limit.
func ListBackups(c *gin.Context) {
	service, ok := serviceForBackups(c)
	if !ok {
		return
	}
	backups := []models.Backup{}
	if err := database.DB.Where("service_id = ?", service.ID).Order("created_at DESC, id DESC").Find(&backups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load backups"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"backups": backups, "limit": service.BackupLimit})
}

// errBackupRefused is a reason a backup cannot be started that the caller
// can do something about.
type errBackupRefused struct {
	status  int
	message string
}

func (e *errBackupRefused) Error() string { return e.message }

func refuseBackup(status int, message string) error {
	return &errBackupRefused{status: status, message: message}
}

var backupNameCleaner = regexp.MustCompile(`[\x00-\x1f\x7f]`)

func cleanBackupName(name string, fallback string) string {
	name = strings.TrimSpace(backupNameCleaner.ReplaceAllString(name, ""))
	if name == "" {
		name = fallback
	}
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}

// startBackup creates a backup record and tells the node to make the
// archive. An automated backup makes room for itself by removing the oldest
// automated one once the limit is reached; a manual backup is refused instead,
// so nobody's backup disappears because someone clicked a button.
func startBackup(service *models.Service, name string, automated bool) (*models.Backup, error) {
	if service.BackupLimit <= 0 {
		return nil, refuseBackup(http.StatusForbidden, "Backups are not enabled for this server")
	}
	if service.Status == StatusInstalling || service.Status == StatusInstallationFailed {
		return nil, refuseBackup(http.StatusConflict, "This server has not finished installing")
	}

	var active int64
	database.DB.Model(&models.Backup{}).
		Where("service_id = ? AND status IN ?", service.ID, []string{models.BackupCreating, models.BackupRestoring}).
		Count(&active)
	if active > 0 {
		return nil, refuseBackup(http.StatusConflict, "Another backup is already in progress for this server")
	}

	// Failed backups hold no archive and do not count.
	var kept []models.Backup
	if err := database.DB.Where("service_id = ? AND status <> ?", service.ID, models.BackupFailed).
		Order("created_at ASC, id ASC").Find(&kept).Error; err != nil {
		return nil, err
	}
	if len(kept) >= service.BackupLimit {
		var oldest *models.Backup
		if automated {
			for i := range kept {
				if kept[i].Automated && kept[i].Status == models.BackupCompleted {
					oldest = &kept[i]
					break
				}
			}
		}
		if oldest == nil {
			return nil, refuseBackup(http.StatusConflict,
				fmt.Sprintf("This server already has its limit of %d backup(s). Delete one first.", service.BackupLimit))
		}
		if err := removeBackup(service, oldest); err != nil {
			return nil, fmt.Errorf("could not remove the oldest backup to make room: %w", err)
		}
	}

	backup := models.Backup{
		UUID:      uuid.New().String(),
		ServiceID: service.ID,
		Name:      cleanBackupName(name, "Backup "+time.Now().Format("2006-01-02 15:04")),
		Status:    models.BackupCreating,
		Automated: automated,
	}
	if err := database.DB.Create(&backup).Error; err != nil {
		return nil, err
	}

	err := daemonDo(daemonHTTP, &service.Node, http.MethodPost, "/api/servers/"+service.UUID+"/backups", map[string]interface{}{"id": backup.UUID})
	if err != nil {
		database.DB.Delete(&models.Backup{}, backup.ID)
		return nil, err
	}
	return &backup, nil
}

// removeBackup deletes a backup's archive from its node and then its record.
// A node that no longer has the archive is not an error.
func removeBackup(service *models.Service, backup *models.Backup) error {
	err := daemonDo(daemonHTTP, &service.Node, http.MethodDelete, backupNodePath(service, backup, ""), nil)
	var reported *nodeError
	if err != nil && !(errors.As(err, &reported) && reported.Status == http.StatusNotFound) {
		return err
	}
	return database.DB.Delete(&models.Backup{}, backup.ID).Error
}

func respondBackupError(c *gin.Context, err error, fallback string) {
	var refused *errBackupRefused
	if errors.As(err, &refused) {
		c.JSON(refused.status, gin.H{"error": refused.message})
		return
	}
	var reported *nodeError
	if errors.As(err, &reported) {
		c.JSON(nodeStatus(reported.Status), gin.H{"error": reported.Message})
		return
	}
	log.Printf("[Core] %s: %v", fallback, err)
	c.JSON(http.StatusBadGateway, gin.H{"error": fallback + ": " + err.Error()})
}

// CreateBackup starts a new backup of a service.
func CreateBackup(c *gin.Context) {
	service, ok := serviceForBackups(c)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&req)

	backup, err := startBackup(service, req.Name, false)
	if err != nil {
		respondBackupError(c, err, "The backup could not be started")
		return
	}
	utils.LogActivity(c, service.ID, "backup", backup.Name, "Started a backup", nil)
	c.JSON(http.StatusAccepted, backup)
}

// DeleteBackup removes a backup and its archive.
func DeleteBackup(c *gin.Context) {
	service, ok := serviceForBackups(c)
	if !ok {
		return
	}
	backup, ok := findBackup(c, service)
	if !ok {
		return
	}
	if backup.Status == models.BackupRestoring {
		c.JSON(http.StatusConflict, gin.H{"error": "This backup is being restored. Wait for that to finish."})
		return
	}

	if err := removeBackup(service, backup); err != nil {
		respondBackupError(c, err, "The backup could not be deleted")
		return
	}
	utils.LogActivity(c, service.ID, "backup", backup.Name, "Deleted a backup", nil)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// RestoreBackup replaces a stopped server's files with a backup's.
func RestoreBackup(c *gin.Context) {
	service, ok := serviceForBackups(c)
	if !ok {
		return
	}
	backup, ok := findBackup(c, service)
	if !ok {
		return
	}
	var req struct {
		// Truncate deletes the server's current files first, so files that
		// are not in the backup do not survive the restore.
		Truncate bool `json:"truncate"`
	}
	_ = c.ShouldBindJSON(&req)

	if backup.Status != models.BackupCompleted {
		c.JSON(http.StatusConflict, gin.H{"error": "Only a finished backup can be restored"})
		return
	}
	if service.Status != StatusOffline {
		c.JSON(http.StatusConflict, gin.H{"error": "Stop the server before restoring a backup"})
		return
	}

	// Claimed in one statement so two restores cannot start together.
	claimed := database.DB.Model(&models.Backup{}).
		Where("id = ? AND status = ?", backup.ID, models.BackupCompleted).
		Updates(map[string]interface{}{"status": models.BackupRestoring, "error": ""})
	if claimed.Error != nil || claimed.RowsAffected == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "This backup is busy. Try again shortly."})
		return
	}

	err := daemonDo(daemonHTTP, &service.Node, http.MethodPost, backupNodePath(service, backup, "/restore"), map[string]interface{}{"truncate": req.Truncate})
	if err != nil {
		database.DB.Model(&models.Backup{}).Where("id = ?", backup.ID).Update("status", models.BackupCompleted)
		respondBackupError(c, err, "The restore could not be started")
		return
	}

	utils.LogActivity(c, service.ID, "backup", backup.Name, "Started restoring a backup", map[string]interface{}{"truncate": req.Truncate})
	backup.Status = models.BackupRestoring
	c.JSON(http.StatusAccepted, backup)
}

// CreateBackupDownload issues a short-lived link for downloading a backup.
// A browser download cannot carry the session header, so the link carries a
// ticket that is only good for this one backup.
func CreateBackupDownload(c *gin.Context) {
	service, ok := serviceForBackups(c)
	if !ok {
		return
	}
	backup, ok := findBackup(c, service)
	if !ok {
		return
	}
	if backup.Status != models.BackupCompleted && backup.Status != models.BackupRestoring {
		c.JSON(http.StatusConflict, gin.H{"error": "Only a finished backup can be downloaded"})
		return
	}

	ticket, err := utils.GenerateDownloadTicket(c.MustGet("user_id").(uint), service.UUID, backup.UUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to issue a download link"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"url": fmt.Sprintf("/api/v1/services/%s/backups/%s/download?ticket=%s", service.UUID, backup.UUID, ticket),
	})
}

var unsafeFilename = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// DownloadBackup streams a backup archive from its node to the browser.
func DownloadBackup(c *gin.Context) {
	serviceUUID, backupUUID := c.Param("uuid"), c.Param("backup")

	claims, err := utils.ValidateDownloadTicket(c.Query("ticket"), serviceUUID, backupUUID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "This download link is invalid or has expired"})
		return
	}
	// Access is checked again in case it was revoked after the link was made.
	service, subUser, ok := utils.FindServiceForUser(serviceUUID, claims.UserID)
	if !ok || (subUser != nil && !subUser.CanManageBackups) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Download denied"})
		return
	}
	var backup models.Backup
	if err := database.DB.Where("uuid = ? AND service_id = ?", backupUUID, service.ID).First(&backup).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
		return
	}

	resp, err := daemonJSON(backupDownloadHTTP, &service.Node, http.MethodGet, backupNodePath(service, &backup, "/download"), nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "The node hosting this backup is unreachable"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(nodeStatus(resp.StatusCode), gin.H{"error": nodeErrorMessage(resp.Body, "The node could not provide this backup")})
		return
	}

	filename := strings.Trim(unsafeFilename.ReplaceAllString(service.Name+"-"+backup.Name, "-"), "-.")
	if filename == "" {
		filename = "backup"
	}
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.tar.gz"`, filename))
	if resp.ContentLength > 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	}
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		log.Printf("[Core] Backup download for %s ended early: %v", service.UUID, err)
	}
}

// BackupStatusRequest is what a node reports when a backup job ends.
type BackupStatusRequest struct {
	// Operation is "create" or "restore".
	Operation string `json:"operation"`
	Status    string `json:"status"` // "completed" or "failed"
	Size      int64  `json:"size"`
	Checksum  string `json:"checksum"`
	Error     string `json:"error"`
}

// HandleBackupStatusUpdate records the outcome of a backup or restore
// reported by the node that holds it.
func HandleBackupStatusUpdate(c *gin.Context) {
	node := middleware.NodeFromContext(c)

	var req BackupStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Status != models.BackupCompleted && req.Status != models.BackupFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown backup status"})
		return
	}

	// A node can only report on backups of its own services.
	var backup models.Backup
	err := database.DB.
		Joins("JOIN services ON services.id = backups.service_id").
		Where("backups.uuid = ? AND services.node_id = ?", c.Param("backup"), node.ID).
		First(&backup).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found on this node"})
		return
	}

	message := strings.TrimSpace(req.Error)
	if len(message) > 1000 {
		message = message[:997] + "..."
	}

	var updates map[string]interface{}
	var from string
	switch req.Operation {
	case "restore":
		// The archive is intact either way; a failed restore is reported
		// on the backup without making it look like a failed backup.
		from = models.BackupRestoring
		updates = map[string]interface{}{"status": models.BackupCompleted, "error": ""}
		if req.Status == models.BackupFailed {
			updates["error"] = "The last restore failed: " + message
		}
	default:
		from = models.BackupCreating
		now := time.Now()
		updates = map[string]interface{}{"status": req.Status, "completed_at": &now, "error": message}
		if req.Status == models.BackupCompleted {
			updates["size"] = req.Size
			updates["checksum"] = req.Checksum
			updates["error"] = ""
		}
	}

	if err := database.DB.Model(&models.Backup{}).Where("id = ? AND status = ?", backup.ID, from).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record backup status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// interruptedBackupMessage explains a job that ended without a report.
const interruptedBackupMessage = "The node stopped before this finished."

// reconcileBackups settles backups whose node no longer has a job for them:
// the node restarted, or its final report was lost.
func reconcileBackups(node *models.Node, activeJobs []string, now time.Time) {
	active := make(map[string]bool, len(activeJobs))
	for _, id := range activeJobs {
		active[id] = true
	}

	var pending []models.Backup
	err := database.DB.
		Joins("JOIN services ON services.id = backups.service_id").
		Where("services.node_id = ? AND backups.status IN ? AND backups.updated_at < ?",
			node.ID, []string{models.BackupCreating, models.BackupRestoring}, now.Add(-statusGrace)).
		Find(&pending).Error
	if err != nil {
		return
	}

	for _, backup := range pending {
		if active[backup.UUID] {
			continue
		}
		updates := map[string]interface{}{"status": models.BackupFailed, "error": interruptedBackupMessage}
		if backup.Status == models.BackupRestoring {
			updates = map[string]interface{}{"status": models.BackupCompleted, "error": "The last restore was interrupted. " + interruptedBackupMessage}
		}
		database.DB.Model(&models.Backup{}).Where("id = ? AND status = ?", backup.ID, backup.Status).Updates(updates)
		log.Printf("[Core] Backup %s on %s had no running job; marked as %s", backup.UUID, node.Name, updates["status"])
	}
}

// deleteServiceBackups removes a service's backup records. The archives go
// with the service's files when the node deletes it.
func deleteServiceBackups(tx *gorm.DB, serviceID uint) error {
	return tx.Where("service_id = ?", serviceID).Delete(&models.Backup{}).Error
}
