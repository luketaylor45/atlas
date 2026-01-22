package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/router"
)

func main() {
	config.LoadConfig()

	// Connect DB
	database.Connect()

	// Auto Migrate
	database.DB.AutoMigrate(&models.User{}, &models.Node{}, &models.Nest{}, &models.Egg{}, &models.EggVariable{}, &models.Service{})

	r := gin.Default()

	// Setup routes
	router.Setup(r)

	// Seed Defaults
	seedDefaults()

	log.Printf("Atlas Core starting on port %s", config.AppConfig.Port)
	if err := r.Run(":" + config.AppConfig.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func seedDefaults() {
	// 1. Seed Nests
	nests := []models.Nest{
		{
			UUID:        "73d63914-1e09-4621-a477-76cd4a9f394c",
			Name:        "Garry's Mod",
			Description: "Garry's Mod game servers and variations.",
		},
		{
			UUID:        "6e530691-893c-497f-bc3a-f11667cf7567",
			Name:        "Minecraft",
			Description: "Minecraft game servers (Paper, Forge, Vanilla).",
		},
	}

	for _, nest := range nests {
		var existing models.Nest
		if err := database.DB.Where("name = ?", nest.Name).First(&existing).Error; err != nil {
			database.DB.Create(&nest)
			log.Printf("Seeded Nest: %s", nest.Name)
		} else {
			database.DB.Model(&existing).Updates(nest)
		}
	}

	// 2. Seed Eggs
	var gmodNest, mcNest models.Nest
	database.DB.Where("name = ?", "Garry's Mod").First(&gmodNest)
	database.DB.Where("name = ?", "Minecraft").First(&mcNest)

	eggs := []models.Egg{
		{
			UUID:              "a1b2c3d4-e5f6-4a5b-8c9d-0e1f2a3b4c5d",
			NestID:            gmodNest.ID,
			Name:              "Garry's Mod (Legacy 32-bit)",
			Description:       "Traditional 32-bit Garry's Mod server. Recommended for older addons with binary modules.",
			DockerImages:      `["ghcr.io/pterodactyl/games:source"]`,
			StartupCommand:    "./srcds_run -game garrysmod -console -port {{SERVER_PORT}} +maxplayers {{MAX_PLAYERS}} +map {{SERVER_MAP}} +sv_setsteamaccount {{STEAM_ACC}} +gamemode {{GAMEMODE}}",
			StopCommand:       "quit",
			InstallContainer:  "steamcmd/steamcmd:ubuntu-22",
			InstallEntrypoint: "bash",
			InstallScript: `#!/bin/bash
cd /home/container
echo "Installing Garry's Mod (32-bit)..."
./steamcmd/steamcmd.sh +force_install_dir /home/container +login anonymous +app_update 4020 validate +quit
echo "Installation complete."
`,
		},
		{
			UUID:              "b2c3d4e5-f6a7-4b6c-9d0e-1f2a3b4c5d6e",
			NestID:            gmodNest.ID,
			Name:              "Garry's Mod (Modern 64-bit)",
			Description:       "Modern 64-bit Garry's Mod server. Better performance and stability on modern hardware.",
			DockerImages:      `["ghcr.io/pterodactyl/games:source"]`,
			StartupCommand:    "./srcds_run -game garrysmod -console -port {{SERVER_PORT}} +maxplayers {{MAX_PLAYERS}} +map {{SERVER_MAP}} +sv_setsteamaccount {{STEAM_ACC}} +gamemode {{GAMEMODE}}",
			StopCommand:       "quit",
			InstallContainer:  "steamcmd/steamcmd:ubuntu-22",
			InstallEntrypoint: "bash",
			InstallScript: `#!/bin/bash
cd /home/container
echo "Installing Garry's Mod (64-bit branch)..."
./steamcmd/steamcmd.sh +force_install_dir /home/container +login anonymous +app_update 4020 -beta x86-64 validate +quit
echo "Installation complete."
`,
		},
		{
			UUID:              "c3d4e5f6-a7b8-4c7d-0e1f-2a3b4c5d6e7f",
			NestID:            mcNest.ID,
			Name:              "Minecraft (Paper/Spigot)",
			Description:       "High-performance Minecraft server using Paper or Spigot. Supports plugins.",
			DockerImages:      `["ghcr.io/pterodactyl/yolks:java_17"]`,
			StartupCommand:    "java -Xms128M -Xmx{{SERVER_MEMORY}}M -jar {{SERVER_JARFILE}}",
			StopCommand:       "stop",
			InstallContainer:  "eclipse-temurin:17-jdk",
			InstallEntrypoint: "bash",
			InstallScript: `#!/bin/bash
cd /home/container
PAPER_VERSION=${PAPER_VERSION:-1.20.4}
echo "Downloading Paper ${PAPER_VERSION}..."
curl -o server.jar -L "https://papermc.io/api/v2/projects/paper/versions/${PAPER_VERSION}/builds/latest/downloads/paper-${PAPER_VERSION}.jar" || echo "Download failed, please set correct version"
echo "eula=true" > eula.txt
echo "Installation complete."
`,
		},
	}

	for _, egg := range eggs {
		var existing models.Egg
		if err := database.DB.Where("name = ?", egg.Name).First(&existing).Error; err != nil {
			database.DB.Create(&egg)
			log.Printf("Seeded Egg: %s", egg.Name)
		} else {
			database.DB.Model(&existing).Updates(egg)
		}
	}
}
