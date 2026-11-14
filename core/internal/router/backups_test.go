package router_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/handlers"
	"github.com/luketaylor45/atlas/core/internal/models"
)

// nodeReport sends a callback the way a node does.
func (e *env) nodeReport(token string, path string, payload interface{}) int {
	e.t.Helper()
	var body bytes.Buffer
	must(e.t, json.NewEncoder(&body).Encode(payload))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal"+path, &body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", token)
	recorder := httptest.NewRecorder()
	e.engine.ServeHTTP(recorder, req)
	return recorder.Code
}

func (e *env) backup(uuid string) models.Backup {
	e.t.Helper()
	var backup models.Backup
	must(e.t, database.DB.Where("uuid = ?", uuid).First(&backup).Error)
	return backup
}

// startBackup creates a backup through the API and returns its UUID.
func (e *env) startBackup(service models.Service, name string) string {
	e.t.Helper()
	res := e.expect(e.do(&e.owner, http.MethodPost, "/services/"+service.UUID+"/backups", map[string]string{"name": name}), http.StatusAccepted)
	return res.Body["uuid"].(string)
}

func (e *env) completeBackup(uuid string) {
	e.t.Helper()
	code := e.nodeReport(e.node.token, "/backups/"+uuid+"/status", map[string]interface{}{
		"operation": "create", "status": "completed", "size": 4096, "checksum": "sha256:abc",
	})
	if code != http.StatusOK {
		e.t.Fatalf("backup report: %d", code)
	}
}

func TestBackupLifecycle(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	path := "/services/" + service.UUID + "/backups"

	// Not before the server is installed.
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"name": "Early"}), http.StatusConflict)
	e.markInstalled(&service)

	id := e.startBackup(service, "  Before the update  ")
	created := e.node.calls(http.MethodPost, "/backups")
	if len(created) != 1 || created[0].Body["id"] != id {
		t.Fatalf("node backup calls = %+v", created)
	}
	if backup := e.backup(id); backup.Status != models.BackupCreating || backup.Name != "Before the update" || backup.Automated {
		t.Fatalf("new backup = %+v", backup)
	}

	// One at a time.
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"name": "Second"}), http.StatusConflict)
	// It cannot be restored or downloaded until it exists.
	e.expect(e.do(&e.owner, http.MethodPost, path+"/"+id+"/restore", nil), http.StatusConflict)
	e.expect(e.do(&e.owner, http.MethodPost, path+"/"+id+"/download", nil), http.StatusConflict)

	e.completeBackup(id)
	backup := e.backup(id)
	if backup.Status != models.BackupCompleted || backup.Size != 4096 || backup.Checksum != "sha256:abc" || backup.CompletedAt == nil {
		t.Fatalf("completed backup = %+v", backup)
	}

	listed := e.expect(e.do(&e.owner, http.MethodGet, path, nil), http.StatusOK)
	if items, _ := listed.Body["backups"].([]interface{}); len(items) != 1 || listed.Body["limit"] != float64(3) {
		t.Fatalf("list = %s", listed.Raw)
	}

	// Restoring needs a stopped server.
	must(t, database.DB.Model(&models.Service{}).Where("id = ?", service.ID).Update("status", "running").Error)
	e.expect(e.do(&e.owner, http.MethodPost, path+"/"+id+"/restore", map[string]bool{"truncate": true}), http.StatusConflict)
	must(t, database.DB.Model(&models.Service{}).Where("id = ?", service.ID).Update("status", "offline").Error)

	e.expect(e.do(&e.owner, http.MethodPost, path+"/"+id+"/restore", map[string]bool{"truncate": true}), http.StatusAccepted)
	restores := e.node.calls(http.MethodPost, "/restore")
	if len(restores) != 1 || restores[0].Body["truncate"] != true {
		t.Fatalf("restore calls = %+v", restores)
	}
	if e.backup(id).Status != models.BackupRestoring {
		t.Fatal("backup is not marked as restoring")
	}
	// While restoring it cannot be deleted or restored again.
	e.expect(e.do(&e.owner, http.MethodDelete, path+"/"+id, nil), http.StatusConflict)
	e.expect(e.do(&e.owner, http.MethodPost, path+"/"+id+"/restore", nil), http.StatusConflict)

	// A failed restore leaves a usable backup with the reason attached.
	if code := e.nodeReport(e.node.token, "/backups/"+id+"/status", map[string]interface{}{"operation": "restore", "status": "failed", "error": "disk full"}); code != http.StatusOK {
		t.Fatalf("restore report: %d", code)
	}
	if backup := e.backup(id); backup.Status != models.BackupCompleted || !strings.Contains(backup.Error, "disk full") || backup.Size != 4096 {
		t.Fatalf("backup after failed restore = %+v", backup)
	}

	e.expect(e.do(&e.owner, http.MethodDelete, path+"/"+id, nil), http.StatusOK)
	if deleted := e.node.calls(http.MethodDelete, "/backups/"+id); len(deleted) != 1 {
		t.Fatalf("node delete calls = %+v", deleted)
	}
	var remaining int64
	database.DB.Model(&models.Backup{}).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("%d backups remain", remaining)
	}
}

func TestBackupLimitAndNodeFailures(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	e.markInstalled(&service)
	path := "/services/" + service.UUID + "/backups"

	// A node that refuses leaves no record behind.
	e.node.fail["POST /backups"] = http.StatusInternalServerError
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"name": "Nope"}), http.StatusInternalServerError)
	delete(e.node.fail, "POST /backups")
	var count int64
	database.DB.Model(&models.Backup{}).Count(&count)
	if count != 0 {
		t.Fatalf("a refused backup left %d records", count)
	}

	// The limit is the admin's to set.
	e.expect(e.do(&e.owner, http.MethodPut, fmt.Sprintf("/admin/services/%d", service.ID), map[string]int{"backup_limit": 9}), http.StatusForbidden)
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/services/%d", service.ID), map[string]int{"backup_limit": 999}), http.StatusBadRequest)
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/services/%d", service.ID), map[string]int{"backup_limit": 1}), http.StatusOK)

	first := e.startBackup(service, "One")
	e.completeBackup(first)
	res := e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"name": "Two"}), http.StatusConflict)
	if !strings.Contains(res.Raw, "limit of 1") {
		t.Fatalf("limit error = %s", res.Raw)
	}

	// A failed backup holds no archive, so it does not use up the limit.
	must(t, database.DB.Model(&models.Backup{}).Where("uuid = ?", first).Update("status", models.BackupFailed).Error)
	second := e.startBackup(service, "Two")
	if code := e.nodeReport(e.node.token, "/backups/"+second+"/status", map[string]interface{}{"operation": "create", "status": "failed", "error": "no space left"}); code != http.StatusOK {
		t.Fatalf("failure report: %d", code)
	}
	if backup := e.backup(second); backup.Status != models.BackupFailed || backup.Error != "no space left" {
		t.Fatalf("failed backup = %+v", backup)
	}

	// Zero turns backups off.
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/services/%d", service.ID), map[string]int{"backup_limit": 0}), http.StatusOK)
	e.expect(e.do(&e.owner, http.MethodPost, path, map[string]string{"name": "Three"}), http.StatusForbidden)

	// A node that cannot be reached keeps the record, so the archive is not orphaned.
	e.node.fail["DELETE /backups/"+second] = http.StatusInternalServerError
	e.expect(e.do(&e.owner, http.MethodDelete, path+"/"+second, nil), http.StatusInternalServerError)
	e.backup(second)
	// But an archive that is already gone does not block cleaning up.
	e.node.fail["DELETE /backups/"+second] = http.StatusNotFound
	e.expect(e.do(&e.owner, http.MethodDelete, path+"/"+second, nil), http.StatusOK)

	// Deleting the server removes its backup records.
	e.expect(e.do(&e.admin, http.MethodDelete, fmt.Sprintf("/admin/services/%d", service.ID), nil), http.StatusOK)
	database.DB.Model(&models.Backup{}).Count(&count)
	if count != 0 {
		t.Fatalf("%d backup records outlived their server", count)
	}
}

func TestBackupAccessAndDownloadTickets(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	otherService := e.createService(8300)
	e.markInstalled(&service)
	e.markInstalled(&otherService)
	path := "/services/" + service.UUID + "/backups"

	id := e.startBackup(service, "Mine")
	e.completeBackup(id)

	// Strangers see nothing; sub-users need the permission.
	e.expect(e.do(&e.other, http.MethodGet, path, nil), http.StatusNotFound)
	e.addSubUser(service, e.other, models.ServiceUser{CanViewConsole: true, CanControlPower: true})
	e.expect(e.do(&e.other, http.MethodGet, path, nil), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodPost, path+"/"+id+"/download", nil), http.StatusForbidden)
	must(t, database.DB.Model(&models.ServiceUser{}).Where("service_id = ? AND user_id = ?", service.ID, e.other.ID).Update("can_manage_backups", true).Error)
	e.expect(e.do(&e.other, http.MethodGet, path, nil), http.StatusOK)

	// A backup is only reachable through its own server.
	e.expect(e.do(&e.owner, http.MethodDelete, "/services/"+otherService.UUID+"/backups/"+id, nil), http.StatusNotFound)

	res := e.expect(e.do(&e.other, http.MethodPost, path+"/"+id+"/download", nil), http.StatusOK)
	link, _ := res.Body["url"].(string)
	if !strings.HasPrefix(link, "/api/v1/services/"+service.UUID+"/backups/"+id+"/download?ticket=") {
		t.Fatalf("download link = %q", link)
	}

	get := func(target string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		e.engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		return recorder
	}
	// The link works without a session, exactly once access still holds.
	download := get(link)
	if download.Code != http.StatusOK || !strings.Contains(download.Header().Get("Content-Disposition"), "Server-8211-Mine.tar.gz") {
		t.Fatalf("download = %d %v", download.Code, download.Header())
	}
	if calls := e.node.calls(http.MethodGet, "/backups/"+id+"/download"); len(calls) != 1 {
		t.Fatalf("node download calls = %+v", calls)
	}

	ticket := link[strings.Index(link, "ticket=")+len("ticket="):]
	if code := get("/api/v1/services/" + service.UUID + "/backups/" + id + "/download").Code; code != http.StatusUnauthorized {
		t.Fatalf("no ticket: %d", code)
	}
	// A ticket is for one backup of one server.
	second := e.startBackup(service, "Other")
	e.completeBackup(second)
	if code := get("/api/v1/services/" + service.UUID + "/backups/" + second + "/download?ticket=" + ticket).Code; code != http.StatusUnauthorized {
		t.Fatalf("ticket reused for another backup: %d", code)
	}
	// Losing access makes an issued ticket useless.
	must(t, database.DB.Model(&models.ServiceUser{}).Where("service_id = ? AND user_id = ?", service.ID, e.other.ID).Update("can_manage_backups", false).Error)
	if code := get(link).Code; code != http.StatusForbidden {
		t.Fatalf("revoked access: %d", code)
	}

	// Nodes can only report on backups they hold.
	otherNode := models.Node{Name: "Node-02", Address: "b.example.com", Port: "8081", Token: "n_other-node-token", IsOnline: true}
	must(t, database.DB.Create(&otherNode).Error)
	if code := e.nodeReport(otherNode.Token, "/backups/"+id+"/status", map[string]interface{}{"operation": "create", "status": "failed"}); code != http.StatusNotFound {
		t.Fatalf("foreign node report: %d", code)
	}
	if code := e.nodeReport(e.node.token, "/backups/"+id+"/status", map[string]interface{}{"operation": "create", "status": "exploded"}); code != http.StatusBadRequest {
		t.Fatalf("unknown status: %d", code)
	}
	if e.backup(id).Status != models.BackupCompleted {
		t.Fatal("a rejected report changed the backup")
	}
}

func TestScheduleValidationAndPermissions(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	e.markInstalled(&service)
	path := "/services/" + service.UUID + "/schedules"

	valid := map[string]interface{}{"name": "Nightly restart", "cron": "0  4 * * *", "action": "power", "payload": "restart"}
	res := e.expect(e.do(&e.owner, http.MethodPost, path, valid), http.StatusCreated)
	id := int(res.Body["id"].(float64))
	if res.Body["cron"] != "0 4 * * *" || res.Body["enabled"] != true || res.Body["next_run_at"] == nil {
		t.Fatalf("created schedule = %s", res.Raw)
	}

	for name, body := range map[string]map[string]interface{}{
		"no name":        {"cron": "0 4 * * *", "action": "power", "payload": "restart"},
		"four fields":    {"name": "x", "cron": "0 4 * *", "action": "power", "payload": "restart"},
		"bad field":      {"name": "x", "cron": "61 4 * * *", "action": "power", "payload": "restart"},
		"own time zone":  {"name": "x", "cron": "TZ=UTC 4 * * *", "action": "power", "payload": "restart"},
		"seconds":        {"name": "x", "cron": "0 0 4 * * *", "action": "power", "payload": "restart"},
		"unknown action": {"name": "x", "cron": "0 4 * * *", "action": "delete", "payload": ""},
		"unknown power":  {"name": "x", "cron": "0 4 * * *", "action": "power", "payload": "explode"},
		"empty command":  {"name": "x", "cron": "0 4 * * *", "action": "command", "payload": " "},
		"two commands":   {"name": "x", "cron": "0 4 * * *", "action": "command", "payload": "say hi\nstop"},
	} {
		if code := e.do(&e.owner, http.MethodPost, path, body).Code; code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, code)
		}
	}

	// Disabling clears the next run; re-enabling sets it again.
	schedulePath := fmt.Sprintf("%s/%d", path, id)
	res = e.expect(e.do(&e.owner, http.MethodPut, schedulePath, map[string]interface{}{"enabled": false}), http.StatusOK)
	if res.Body["enabled"] != false || res.Body["next_run_at"] != nil || res.Body["name"] != "Nightly restart" {
		t.Fatalf("disabled schedule = %s", res.Raw)
	}
	var stored models.Schedule
	must(t, database.DB.First(&stored, id).Error)
	if stored.Enabled || stored.NextRunAt != nil {
		t.Fatalf("stored schedule = %+v", stored)
	}
	res = e.expect(e.do(&e.owner, http.MethodPut, schedulePath, map[string]interface{}{"enabled": true, "cron": "*/5 * * * *"}), http.StatusOK)
	if res.Body["next_run_at"] == nil || res.Body["cron"] != "*/5 * * * *" {
		t.Fatalf("re-enabled schedule = %s", res.Raw)
	}

	// A sub-user needs the schedules permission, and can only schedule what
	// they could do by hand.
	e.addSubUser(service, e.other, models.ServiceUser{CanViewConsole: true, CanSendCommands: true})
	e.expect(e.do(&e.other, http.MethodGet, path, nil), http.StatusForbidden)
	must(t, database.DB.Model(&models.ServiceUser{}).Where("service_id = ? AND user_id = ?", service.ID, e.other.ID).Update("can_manage_schedules", true).Error)
	e.expect(e.do(&e.other, http.MethodGet, path, nil), http.StatusOK)
	e.expect(e.do(&e.other, http.MethodPost, path, valid), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodPost, path, map[string]interface{}{"name": "Backup", "cron": "0 4 * * *", "action": "backup"}), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodPost, path, map[string]interface{}{"name": "Announce", "cron": "0 4 * * *", "action": "command", "payload": "say hi"}), http.StatusCreated)
	// Nor may they edit or run a schedule that does more than they can.
	e.expect(e.do(&e.other, http.MethodPut, schedulePath, map[string]interface{}{"name": "Mine now"}), http.StatusForbidden)
	e.expect(e.do(&e.other, http.MethodPost, schedulePath+"/run", nil), http.StatusForbidden)

	// Schedules belong to their server.
	otherService := e.createService(8300)
	e.expect(e.do(&e.owner, http.MethodDelete, fmt.Sprintf("/services/%s/schedules/%d", otherService.UUID, id), nil), http.StatusNotFound)
	e.expect(e.do(&e.owner, http.MethodDelete, schedulePath, nil), http.StatusOK)
}

// waitFor polls until a condition holds; schedule runs happen in the background.
func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestDueSchedulesRunOnceAndRecordTheOutcome(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	e.markInstalled(&service)
	must(t, database.DB.Model(&models.Service{}).Where("id = ?", service.ID).Update("status", "running").Error)
	path := "/services/" + service.UUID + "/schedules"

	create := func(body map[string]interface{}) models.Schedule {
		res := e.expect(e.do(&e.owner, http.MethodPost, path, body), http.StatusCreated)
		var schedule models.Schedule
		must(t, database.DB.First(&schedule, int(res.Body["id"].(float64))).Error)
		return schedule
	}
	makeDue := func(schedule models.Schedule) {
		past := time.Now().Add(-time.Hour)
		must(t, database.DB.Model(&models.Schedule{}).Where("id = ?", schedule.ID).Update("next_run_at", &past).Error)
	}
	reload := func(schedule models.Schedule) models.Schedule {
		var fresh models.Schedule
		must(t, database.DB.First(&fresh, schedule.ID).Error)
		return fresh
	}

	restart := create(map[string]interface{}{"name": "Restart", "cron": "0 4 * * *", "action": "power", "payload": "restart"})
	command := create(map[string]interface{}{"name": "Announce", "cron": "*/30 * * * *", "action": "command", "payload": "say Restarting soon"})
	disabled := create(map[string]interface{}{"name": "Off", "cron": "* * * * *", "action": "power", "payload": "stop", "enabled": false})
	notDue := create(map[string]interface{}{"name": "Later", "cron": "0 5 * * *", "action": "power", "payload": "stop"})
	makeDue(restart)
	makeDue(command)
	must(t, database.DB.Model(&models.Schedule{}).Where("id = ?", disabled.ID).Update("next_run_at", time.Now().Add(-time.Hour)).Error)

	now := time.Now()
	handlers.RunDueSchedules(now)
	// A second tick straight away must not run anything twice.
	handlers.RunDueSchedules(now)

	waitFor(t, "both schedules to finish", func() bool {
		return reload(restart).LastStatus != "" && reload(command).LastStatus != ""
	})
	time.Sleep(100 * time.Millisecond)

	power := e.node.calls(http.MethodPost, "/power")
	if len(power) != 1 || power[0].Body["action"] != "restart" || power[0].Body["stop_command"] != "stop" {
		t.Fatalf("power calls = %+v", power)
	}
	commands := e.node.calls(http.MethodPost, "/command")
	if len(commands) != 1 || commands[0].Body["command"] != "say Restarting soon" {
		t.Fatalf("command calls = %+v", commands)
	}
	for _, schedule := range []models.Schedule{reload(restart), reload(command)} {
		if schedule.LastStatus != "success" || schedule.LastRunAt == nil || schedule.NextRunAt == nil || !schedule.NextRunAt.After(now) {
			t.Fatalf("schedule after run = %+v", schedule)
		}
	}
	if reload(disabled).LastRunAt != nil || reload(notDue).LastRunAt != nil {
		t.Fatal("a disabled or not-yet-due schedule ran")
	}

	// A node failure is recorded, not lost.
	e.node.fail["POST /power"] = http.StatusInternalServerError
	makeDue(restart)
	handlers.RunDueSchedules(time.Now())
	waitFor(t, "the failed run", func() bool { return reload(restart).LastStatus == "failed" })
	if message := reload(restart).LastMessage; !strings.Contains(strings.ToLower(message), "node says no") {
		t.Fatalf("failure message = %q", message)
	}
	delete(e.node.fail, "POST /power")

	// "Only when online" skips a stopped server, but a run by hand goes ahead.
	must(t, database.DB.Model(&models.Service{}).Where("id = ?", service.ID).Update("status", "offline").Error)
	e.expect(e.do(&e.owner, http.MethodPut, fmt.Sprintf("%s/%d", path, restart.ID), map[string]interface{}{"only_when_online": true}), http.StatusOK)
	makeDue(restart)
	handlers.RunDueSchedules(time.Now())
	waitFor(t, "the skipped run", func() bool { return reload(restart).LastStatus == "skipped" })

	before := len(e.node.calls(http.MethodPost, "/power"))
	res := e.expect(e.do(&e.owner, http.MethodPost, fmt.Sprintf("%s/%d/run", path, restart.ID), nil), http.StatusOK)
	if res.Body["status"] != "success" || len(e.node.calls(http.MethodPost, "/power")) != before+1 {
		t.Fatalf("manual run = %s", res.Raw)
	}

	// A suspended server is never started by a schedule.
	must(t, database.DB.Model(&models.Service{}).Where("id = ?", service.ID).Update("is_suspended", true).Error)
	res = e.expect(e.do(&e.owner, http.MethodPost, fmt.Sprintf("%s/%d/run", path, restart.ID), nil), http.StatusOK)
	if res.Body["status"] != "skipped" {
		t.Fatalf("suspended run = %s", res.Raw)
	}
}

func TestScheduledBackupsRotateOnlyAutomatedOnes(t *testing.T) {
	e := newEnv(t)
	service := e.createService(8211)
	e.markInstalled(&service)
	e.expect(e.do(&e.admin, http.MethodPut, fmt.Sprintf("/admin/services/%d", service.ID), map[string]int{"backup_limit": 2}), http.StatusOK)
	path := "/services/" + service.UUID

	res := e.expect(e.do(&e.owner, http.MethodPost, path+"/schedules", map[string]interface{}{"name": "Nightly", "cron": "0 3 * * *", "action": "backup"}), http.StatusCreated)
	run := fmt.Sprintf("%s/schedules/%d/run", path, int(res.Body["id"].(float64)))

	latest := func() models.Backup {
		var backup models.Backup
		must(t, database.DB.Where("service_id = ?", service.ID).Order("id DESC").First(&backup).Error)
		return backup
	}

	// Two automated backups fill the limit.
	e.expect(e.do(&e.owner, http.MethodPost, run, nil), http.StatusOK)
	first := latest()
	if !first.Automated || !strings.HasPrefix(first.Name, "Nightly ") {
		t.Fatalf("scheduled backup = %+v", first)
	}
	e.completeBackup(first.UUID)
	e.expect(e.do(&e.owner, http.MethodPost, run, nil), http.StatusOK)
	second := latest()
	e.completeBackup(second.UUID)

	// The third replaces the oldest automated one.
	e.expect(e.do(&e.owner, http.MethodPost, run, nil), http.StatusOK)
	third := latest()
	e.completeBackup(third.UUID)
	var uuids []string
	database.DB.Model(&models.Backup{}).Where("service_id = ?", service.ID).Order("id").Pluck("uuid", &uuids)
	if len(uuids) != 2 || uuids[0] != second.UUID || uuids[1] != third.UUID {
		t.Fatalf("backups after rotation = %v (first was %s)", uuids, first.UUID)
	}
	if deleted := e.node.calls(http.MethodDelete, "/backups/"+first.UUID); len(deleted) != 1 {
		t.Fatalf("the rotated backup's archive was not removed: %+v", deleted)
	}

	// Backups people made by hand are never rotated away.
	must(t, database.DB.Model(&models.Backup{}).Where("service_id = ?", service.ID).Update("automated", false).Error)
	res = e.expect(e.do(&e.owner, http.MethodPost, run, nil), http.StatusOK)
	if res.Body["status"] != "skipped" || !strings.Contains(res.Raw, "limit of 2") {
		t.Fatalf("run at a limit of manual backups = %s", res.Raw)
	}
	var count int64
	database.DB.Model(&models.Backup{}).Where("service_id = ?", service.ID).Count(&count)
	if count != 2 {
		t.Fatalf("%d backups, want 2", count)
	}
}
