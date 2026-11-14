package database

import (
	"fmt"
	"log"
	"time"

	"github.com/luketaylor45/atlas/core/internal/models"
	"gorm.io/gorm"
)

// SchemaMigration records a one-off migration that has been applied.
type SchemaMigration struct {
	Version   string `gorm:"primaryKey;size:100"`
	AppliedAt time.Time
}

type migration struct {
	version string
	run     func(tx *gorm.DB) error
}

// AllModels is every table Atlas owns, in dependency order.
func AllModels() []interface{} {
	return []interface{}{
		&models.Setting{},
		&models.User{},
		&models.Node{},
		&models.Nest{},
		&models.Egg{},
		&models.EggVariable{},
		&models.Service{},
		&models.ServicePort{},
		&models.ServiceUser{},
		&models.ActivityLog{},
		&models.News{},
		&models.Backup{},
		&models.Schedule{},
	}
}

// migrations are applied once each, in order, and never edited after release.
// Additive schema changes (new tables and columns) are handled by AutoMigrate;
// anything AutoMigrate cannot express safely goes here.
var migrations = []migration{
	{"2026-11-001-backfill-service-ports", backfillServicePorts},
	{"2026-11-002-eggs-do-not-cascade-to-services", restrictEggServiceCascade},
	{"2026-11-003-purge-soft-deleted-services", purgeSoftDeletedServices},
	{"2026-11-004-release-deleted-usernames", releaseDeletedUsernames},
	{"2026-11-005-activity-without-service-or-user", allowActivityWithoutServiceOrUser},
}

// Migrate brings the database schema and data up to date. It is safe to run
// on every start and from more than one process.
func Migrate() error {
	if err := DB.AutoMigrate(&SchemaMigration{}); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	if err := DB.AutoMigrate(AllModels()...); err != nil {
		return fmt.Errorf("update schema: %w", err)
	}

	for _, step := range migrations {
		var applied int64
		if err := DB.Model(&SchemaMigration{}).Where("version = ?", step.version).Count(&applied).Error; err != nil {
			return err
		}
		if applied > 0 {
			continue
		}

		// The migration and its record commit together, so a failure leaves
		// it to be retried in full on the next start.
		err := DB.Transaction(func(tx *gorm.DB) error {
			if err := step.run(tx); err != nil {
				return err
			}
			return tx.Create(&SchemaMigration{Version: step.version, AppliedAt: time.Now()}).Error
		})
		if err != nil {
			return fmt.Errorf("migration %s: %w", step.version, err)
		}
		log.Printf("[Core] Applied migration %s", step.version)
	}
	return nil
}

// backfillServicePorts gives services created before port allocations existed
// a row for their primary port.
func backfillServicePorts(tx *gorm.DB) error {
	var services []models.Service
	err := tx.
		Where("id NOT IN (?)", tx.Model(&models.ServicePort{}).Select("service_id").Where("is_primary = ?", true)).
		Order("id").
		Find(&services).Error
	if err != nil {
		return err
	}

	taken := map[[2]int]bool{}
	var existing []models.ServicePort
	if err := tx.Find(&existing).Error; err != nil {
		return err
	}
	for _, port := range existing {
		taken[[2]int{int(port.NodeID), port.Port}] = true
	}

	for _, service := range services {
		key := [2]int{int(service.NodeID), service.Port}
		if taken[key] {
			// Two legacy services share a port and cannot both hold it. The
			// first keeps it; the other needs a new port from an admin.
			log.Printf("[Core] Service %s shares port %d with another service on its node; assign it a different port.", service.UUID, service.Port)
			continue
		}
		taken[key] = true

		port := models.ServicePort{
			ServiceID: service.ID,
			NodeID:    service.NodeID,
			Port:      service.Port,
			Env:       models.PrimaryPortEnv,
			Name:      "Game",
			Protocol:  "both",
			IsPrimary: true,
		}
		if err := tx.Create(&port).Error; err != nil {
			return err
		}
	}
	return nil
}

// restrictEggServiceCascade replaces the foreign key that deleted every
// server of an egg when the egg was deleted. AutoMigrate never alters an
// existing constraint, so it is replaced explicitly.
func restrictEggServiceCascade(tx *gorm.DB) error {
	if tx.Dialector.Name() != "postgres" {
		return nil
	}
	if err := tx.Exec(`ALTER TABLE services DROP CONSTRAINT IF EXISTS fk_eggs_servers`).Error; err != nil {
		return err
	}
	return tx.Exec(`ALTER TABLE services ADD CONSTRAINT fk_eggs_servers FOREIGN KEY (egg_id) REFERENCES eggs(id) ON DELETE RESTRICT`).Error
}

// purgeSoftDeletedServices removes services that older releases only marked
// as deleted. They are invisible everywhere but still held ports and blocked
// their eggs and owners from being removed.
func purgeSoftDeletedServices(tx *gorm.DB) error {
	var ids []uint
	if err := tx.Unscoped().Model(&models.Service{}).Where("deleted_at IS NOT NULL").Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	for _, table := range []interface{}{&models.ServicePort{}, &models.ServiceUser{}, &models.ActivityLog{}} {
		if err := tx.Where("service_id IN ?", ids).Delete(table).Error; err != nil {
			return err
		}
	}
	return tx.Unscoped().Where("id IN ?", ids).Delete(&models.Service{}).Error
}

// releaseDeletedUsernames frees the usernames of deleted accounts, which the
// unique index otherwise reserves forever.
func releaseDeletedUsernames(tx *gorm.DB) error {
	var users []models.User
	if err := tx.Unscoped().Where("deleted_at IS NOT NULL").Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		if err := tx.Unscoped().Model(&models.User{}).Where("id = ?", user.ID).
			Update("username", DeletedUsername(user.Username, user.ID)).Error; err != nil {
			return err
		}
	}
	return nil
}

// allowActivityWithoutServiceOrUser lets the activity log record actions
// that are not about one service (managing nodes or users) or not taken by a
// user. Those entries used to fail the foreign key and were silently lost.
func allowActivityWithoutServiceOrUser(tx *gorm.DB) error {
	if tx.Dialector.Name() != "postgres" {
		return nil
	}
	if err := tx.Exec(`ALTER TABLE activity_logs ALTER COLUMN service_id DROP NOT NULL`).Error; err != nil {
		return err
	}
	return tx.Exec(`ALTER TABLE activity_logs ALTER COLUMN user_id DROP NOT NULL`).Error
}

// DeletedUsername is the name a deleted account is parked under.
func DeletedUsername(username string, id uint) string {
	const marker = "#deleted-"
	for i := 0; i+len(marker) <= len(username); i++ {
		if username[i:i+len(marker)] == marker {
			return username
		}
	}
	return fmt.Sprintf("%s%s%d", username, marker, id)
}
