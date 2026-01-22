package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Port        string `mapstructure:"PORT"`
	DatabaseURL string `mapstructure:"DATABASE_URL"`
	JWTSecret   string `mapstructure:"JWT_SECRET"`
	Environment string `mapstructure:"ENVIRONMENT"`
}

var AppConfig Config

func LoadConfig() {
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("ENVIRONMENT", "development")
	viper.SetDefault("DATABASE_URL", "host=localhost user=postgres password=Mandude007 dbname=atlas port=5432 sslmode=disable")
	viper.SetDefault("JWT_SECRET", "change-me-in-production")

	viper.SetConfigName("config")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Printf("Error reading config file: %v", err)
		}
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}
}
