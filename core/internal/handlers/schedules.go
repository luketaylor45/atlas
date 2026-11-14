package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
	"github.com/robfig/cron/v3"
)

const (
	maxSchedulesPerService = 25
	schedulerInterval      = 15 * time.Second

	scheduleSuccess = "success"
	scheduleFailed  = "failed"
	scheduleSkipped = "skipped"
)

// cronParser reads the usual five fields: minute, hour, day of month, month
// and day of week.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// parseCron checks an expression and returns its schedule.
func parseCron(expression string) (cron.Schedule, error) {
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return nil, errors.New("the schedule needs five fields: minute, hour, day of month, month and day of week")
	}
	// The parser also accepts a "TZ=" prefix; schedules all share Core's time zone.
	if strings.Contains(expression, "=") {
		return nil, errors.New("the schedule cannot set its own time zone")
	}
	schedule, err := cronParser.Parse(strings.Join(fields, " "))
	if err != nil {
		return nil, fmt.Errorf("the schedule is not valid: %s", strings.TrimPrefix(err.Error(), "failed to parse "))
	}
	return schedule, nil
}

// nextRun is the first time after `after` that an expression matches, or nil
// when it never will (for example 31 February).
func nextRun(expression string, after time.Time) *time.Time {
	schedule, err := parseCron(expression)
	if err != nil {
		return nil
	}
	next := schedule.Next(after)
	if next.IsZero() {
		return nil
	}
	return &next
}

// ScheduleRequest carries the editable fields of a schedule.
type ScheduleRequest struct {
	Name           *string `json:"name"`
	Cron           *string `json:"cron"`
	Action         *string `json:"action"`
	Payload        *string `json:"payload"`
	Enabled        *bool   `json:"enabled"`
	OnlyWhenOnline *bool   `json:"only_when_online"`
}

func serviceForSchedules(c *gin.Context) (*models.Service, *models.ServiceUser, bool) {
	userID := c.MustGet("user_id").(uint)
	service, subUser, ok := utils.FindServiceForUser(c.Param("uuid"), userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or no access"})
		return nil, nil, false
	}
	if subUser != nil && !subUser.CanManageSchedules {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to manage this server's schedules"})
		return nil, nil, false
	}
	return service, subUser, true
}

// validateScheduleAction checks an action and its payload. A sub-user may
// only schedule what they could do by hand, so a schedule can never be a way
// around their other permissions.
func validateScheduleAction(action string, payload string, subUser *models.ServiceUser) (string, error) {
	payload = strings.TrimSpace(payload)
	switch action {
	case models.ScheduleActionPower:
		switch payload {
		case "start", "stop", "restart", "kill":
		default:
			return "", errors.New("choose start, stop, restart or kill")
		}
		if subUser != nil && !subUser.CanControlPower {
			return "", errForbiddenAction("You do not have permission to control this server's power")
		}
	case models.ScheduleActionCommand:
		if payload == "" {
			return "", errors.New("enter the command to send")
		}
		if len(payload) > 500 || strings.ContainsAny(payload, "\n\r\x00") {
			return "", errors.New("the command must be a single line of at most 500 characters")
		}
		if subUser != nil && !subUser.CanSendCommands {
			return "", errForbiddenAction("You do not have permission to send commands to this server")
		}
	case models.ScheduleActionBackup:
		payload = cleanBackupName(payload, "")
		if subUser != nil && !subUser.CanManageBackups {
			return "", errForbiddenAction("You do not have permission to manage this server's backups")
		}
	default:
		return "", errors.New("choose what the schedule should do: power, command or backup")
	}
	return payload, nil
}

type errForbiddenAction string

func (e errForbiddenAction) Error() string { return string(e) }

func respondScheduleError(c *gin.Context, err error) {
	var forbidden errForbiddenAction
	if errors.As(err, &forbidden) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": capitalize(err.Error())})
}

func capitalize(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

// ListSchedules returns a service's schedules and the time zone they run in.
func ListSchedules(c *gin.Context) {
	service, _, ok := serviceForSchedules(c)
	if !ok {
		return
	}
	schedules := []models.Schedule{}
	if err := database.DB.Where("service_id = ?", service.ID).Order("id ASC").Find(&schedules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load schedules"})
		return
	}
	now := time.Now()
	zone, offset := now.Zone()
	c.JSON(http.StatusOK, gin.H{
		"schedules": schedules,
		// What the cron fields are read in, so the panel can say so.
		"timezone":        time.Local.String(),
		"timezone_abbr":   zone,
		"timezone_offset": offset,
		"now":             now,
	})
}

// CreateSchedule adds a schedule to a service.
func CreateSchedule(c *gin.Context) {
	service, subUser, ok := serviceForSchedules(c)
	if !ok {
		return
	}
	var req ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var count int64
	database.DB.Model(&models.Schedule{}).Where("service_id = ?", service.ID).Count(&count)
	if count >= maxSchedulesPerService {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("A server can have at most %d schedules", maxSchedulesPerService)})
		return
	}

	schedule := models.Schedule{ServiceID: service.ID, Enabled: true}
	if err := applyScheduleRequest(&schedule, &req, subUser, true); err != nil {
		respondScheduleError(c, err)
		return
	}
	if err := database.DB.Create(&schedule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create the schedule"})
		return
	}

	utils.LogActivity(c, service.ID, "schedule", schedule.Name, "Created a schedule", nil)
	c.JSON(http.StatusCreated, schedule)
}

// applyScheduleRequest validates a request and copies it onto a schedule.
// On create every required field must be present; on update only what was
// sent changes.
func applyScheduleRequest(schedule *models.Schedule, req *ScheduleRequest, subUser *models.ServiceUser, creating bool) error {
	if req.Name != nil || creating {
		name := ""
		if req.Name != nil {
			name = strings.TrimSpace(*req.Name)
		}
		if name == "" {
			return errors.New("give the schedule a name")
		}
		if len(name) > 100 {
			return errors.New("the name can be at most 100 characters")
		}
		schedule.Name = name
	}

	if req.Cron != nil || creating {
		expression := ""
		if req.Cron != nil {
			expression = strings.Join(strings.Fields(*req.Cron), " ")
		}
		if _, err := parseCron(expression); err != nil {
			return err
		}
		schedule.Cron = expression
	}

	if req.Action != nil || req.Payload != nil || creating {
		action, payload := schedule.Action, schedule.Payload
		if req.Action != nil {
			action = strings.TrimSpace(*req.Action)
		}
		if req.Payload != nil {
			payload = *req.Payload
		}
		cleaned, err := validateScheduleAction(action, payload, subUser)
		if err != nil {
			return err
		}
		schedule.Action, schedule.Payload = action, cleaned
	}

	if req.Enabled != nil {
		schedule.Enabled = *req.Enabled
	}
	if req.OnlyWhenOnline != nil {
		schedule.OnlyWhenOnline = *req.OnlyWhenOnline
	}

	schedule.NextRunAt = nil
	if schedule.Enabled {
		schedule.NextRunAt = nextRun(schedule.Cron, time.Now())
	}
	return nil
}

func findSchedule(c *gin.Context, service *models.Service) (*models.Schedule, bool) {
	var schedule models.Schedule
	if err := database.DB.Where("id = ? AND service_id = ?", c.Param("schedule"), service.ID).First(&schedule).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Schedule not found"})
		return nil, false
	}
	return &schedule, true
}

// UpdateSchedule changes a schedule.
func UpdateSchedule(c *gin.Context) {
	service, subUser, ok := serviceForSchedules(c)
	if !ok {
		return
	}
	schedule, ok := findSchedule(c, service)
	if !ok {
		return
	}
	var req ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// A sub-user may not touch a schedule that does something they are not
	// allowed to do themselves, even just to re-enable it.
	if _, err := validateScheduleAction(schedule.Action, schedule.Payload, subUser); err != nil {
		var forbidden errForbiddenAction
		if errors.As(err, &forbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
	}
	if err := applyScheduleRequest(schedule, &req, subUser, false); err != nil {
		respondScheduleError(c, err)
		return
	}

	// A map, so that false and empty values are written too.
	err := database.DB.Model(&models.Schedule{}).Where("id = ?", schedule.ID).Updates(map[string]interface{}{
		"name":             schedule.Name,
		"cron":             schedule.Cron,
		"action":           schedule.Action,
		"payload":          schedule.Payload,
		"enabled":          schedule.Enabled,
		"only_when_online": schedule.OnlyWhenOnline,
		"next_run_at":      schedule.NextRunAt,
	}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update the schedule"})
		return
	}

	utils.LogActivity(c, service.ID, "schedule", schedule.Name, "Updated a schedule", nil)
	c.JSON(http.StatusOK, schedule)
}

// DeleteSchedule removes a schedule.
func DeleteSchedule(c *gin.Context) {
	service, _, ok := serviceForSchedules(c)
	if !ok {
		return
	}
	schedule, ok := findSchedule(c, service)
	if !ok {
		return
	}
	if err := database.DB.Delete(&models.Schedule{}, schedule.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete the schedule"})
		return
	}
	utils.LogActivity(c, service.ID, "schedule", schedule.Name, "Deleted a schedule", nil)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// RunScheduleNow runs a schedule's action straight away, whatever its timing.
func RunScheduleNow(c *gin.Context) {
	service, subUser, ok := serviceForSchedules(c)
	if !ok {
		return
	}
	schedule, ok := findSchedule(c, service)
	if !ok {
		return
	}
	if _, err := validateScheduleAction(schedule.Action, schedule.Payload, subUser); err != nil {
		respondScheduleError(c, err)
		return
	}

	utils.LogActivity(c, service.ID, "schedule", schedule.Name, "Ran a schedule by hand", nil)
	status, message := runSchedule(schedule, true)
	database.DB.First(schedule, schedule.ID)
	if status == scheduleFailed {
		c.JSON(http.StatusBadGateway, gin.H{"error": message, "schedule": schedule})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status, "message": message, "schedule": schedule})
}

// StartScheduler runs due schedules until ctx ends.
func StartScheduler(ctx context.Context) {
	ticker := time.NewTicker(schedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			RunDueSchedules(time.Now())
		}
	}
}

// RunDueSchedules claims and runs every schedule whose time has come. A
// schedule that was due several times while Core was down runs once.
func RunDueSchedules(now time.Time) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("[Core] Scheduler recovered from: %v", recovered)
		}
	}()

	var due []models.Schedule
	if err := database.DB.Where("enabled = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", true, now).Find(&due).Error; err != nil {
		log.Printf("[Core] Scheduler could not list schedules: %v", err)
		return
	}

	for i := range due {
		schedule := due[i]
		// Moving the next run forward is the claim: only one process (or
		// tick) gets to change a row that is still due.
		claimed := database.DB.Model(&models.Schedule{}).
			Where("id = ? AND enabled = ? AND next_run_at <= ?", schedule.ID, true, now).
			Update("next_run_at", nextRun(schedule.Cron, now))
		if claimed.Error != nil || claimed.RowsAffected == 0 {
			continue
		}
		go func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Printf("[Core] Schedule %d recovered from: %v", schedule.ID, recovered)
				}
			}()
			runSchedule(&schedule, false)
		}()
	}
}

// runSchedule performs a schedule's action and records how it went. A run
// started by hand ignores the "only when online" setting: the person asking
// can see the server's state for themselves.
func runSchedule(schedule *models.Schedule, manual bool) (string, string) {
	status, message := performSchedule(schedule, manual)

	now := time.Now()
	database.DB.Model(&models.Schedule{}).Where("id = ?", schedule.ID).Updates(map[string]interface{}{
		"last_run_at":  &now,
		"last_status":  status,
		"last_message": message,
	})
	// Skipped runs are visible on the schedule itself; logging each one would
	// bury the activity log under a schedule that runs every few minutes.
	if !manual && status != scheduleSkipped {
		utils.LogActivityDirect(schedule.ServiceID, 0, "schedule", schedule.Name, fmt.Sprintf("Schedule ran (%s): %s", status, message), "")
	}
	return status, message
}

func performSchedule(schedule *models.Schedule, manual bool) (string, string) {
	service, err := loadServiceForNode(schedule.ServiceID)
	if err != nil {
		return scheduleFailed, "The server no longer exists."
	}
	if service.Status == StatusInstalling || service.Status == StatusInstallationFailed {
		return scheduleSkipped, "The server has not finished installing."
	}
	if !manual && schedule.OnlyWhenOnline && service.Status != StatusRunning {
		return scheduleSkipped, "The server was not running."
	}

	switch schedule.Action {
	case models.ScheduleActionPower:
		if service.IsSuspended && (schedule.Payload == "start" || schedule.Payload == "restart") {
			return scheduleSkipped, "The server is suspended."
		}
		if err := daemonDo(daemonHTTP, &service.Node, http.MethodPost, "/api/servers/"+service.UUID+"/power", powerPayload(service, schedule.Payload)); err != nil {
			return scheduleFailed, capitalize(err.Error())
		}
		return scheduleSuccess, "Sent the power action: " + schedule.Payload + "."

	case models.ScheduleActionCommand:
		if service.Status != StatusRunning && service.Status != StatusStarting {
			return scheduleSkipped, "The server was not running, so the command was not sent."
		}
		if err := daemonDo(daemonHTTP, &service.Node, http.MethodPost, "/api/servers/"+service.UUID+"/command", map[string]interface{}{"command": schedule.Payload}); err != nil {
			return scheduleFailed, capitalize(err.Error())
		}
		return scheduleSuccess, "Sent the command."

	case models.ScheduleActionBackup:
		name := schedule.Payload
		if name == "" {
			name = schedule.Name
		}
		backup, err := startBackup(service, name+" "+time.Now().Format("2006-01-02 15:04"), true)
		if err != nil {
			var refused *errBackupRefused
			if errors.As(err, &refused) {
				return scheduleSkipped, refused.message
			}
			return scheduleFailed, capitalize(err.Error())
		}
		return scheduleSuccess, "Started the backup \"" + backup.Name + "\"."
	}
	return scheduleFailed, "This schedule has an unknown action."
}

// powerPayload is the body of a power request to a node.
func powerPayload(service *models.Service, action string) map[string]interface{} {
	return withEggRuntime(map[string]interface{}{
		"action":          action,
		"startup_command": resolveStartupCommand(service),
		"environment":     mustJSON(mergedEnvironment(service)),
		"memory":          service.Memory,
		"port":            service.Port,
	}, &service.Egg)
}
