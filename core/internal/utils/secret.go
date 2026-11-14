package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"gorm.io/gorm/clause"
)

const jwtSecretSetting = "jwt_secret"

// weakJWTSecrets are placeholders that have appeared in Atlas source or docs.
// A token signed with one of them can be forged by anyone.
var weakJWTSecrets = map[string]bool{
	"":                        true,
	"change-me":               true,
	"change-me-in-production": true,
}

// EnsureJWTSecret guarantees tokens are signed with a private secret. An
// operator-supplied JWT_SECRET wins; otherwise a random secret is generated
// once and kept in the database so sessions survive restarts.
func EnsureJWTSecret() error {
	if secret := config.AppConfig.JWTSecret; !weakJWTSecrets[secret] {
		if len(secret) < 32 {
			return fmt.Errorf("JWT_SECRET must be at least 32 characters; unset it to let Atlas generate one")
		}
		return nil
	}

	raw := make([]byte, 48)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate signing secret: %w", err)
	}
	candidate := models.Setting{Key: jwtSecretSetting, Value: hex.EncodeToString(raw)}

	// Insert-if-absent keeps concurrent starts from generating different secrets.
	if err := database.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
		return fmt.Errorf("store signing secret: %w", err)
	}

	var stored models.Setting
	if err := database.DB.First(&stored, "key = ?", jwtSecretSetting).Error; err != nil {
		return fmt.Errorf("load signing secret: %w", err)
	}
	if stored.Value == candidate.Value {
		log.Println("[Core] Generated a new signing secret. Existing sessions must log in again.")
	}
	config.AppConfig.JWTSecret = stored.Value
	return nil
}
