package database

import (
	"log"

	"github.com/luketaylor45/atlas/core/internal/models"
)

func SeedDefaults() {
	// 1. Seed Top-Level Nests (Categories)
	topNests := []models.Nest{
		{
			UUID:        "73d63914-1e09-4621-a477-76cd4a9f394c",
			Name:        "Game Servers",
			Code:        "games",
			Description: "High-performance game server environments.",
		},
		{
			UUID:        "8f21a32b-9e41-4d92-bf3c-2a1f8e7d6c5b",
			Name:        "Generic Apps",
			Code:        "generic",
			Description: "Standard applications and development tools.",
		},
		{
			UUID:        "6e530691-893c-497f-bc3a-f11667cf7567",
			Name:        "Voice & Communication",
			Code:        "voice",
			Description: "Low-latency communication platforms.",
		},
	}

	for _, nest := range topNests {
		var existing models.Nest
		if err := DB.Where("uuid = ?", nest.UUID).First(&existing).Error; err != nil {
			DB.Create(&nest)
			log.Printf("[Seed] Created Category: %s", nest.Name)
		} else {
			DB.Model(&existing).Updates(nest)
		}
	}
}
