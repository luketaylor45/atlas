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

	// RegistrationEnabled allows anyone who can reach the panel to create an
	// account. It is off by default; admins create users in Admin > Users.
	RegistrationEnabled bool `mapstructure:"REGISTRATION_ENABLED"`
	// TrustedProxies lists the proxy addresses whose X-Forwarded-For is believed.
	TrustedProxies string `mapstructure:"TRUSTED_PROXIES"`
}

var AppConfig Config

func LoadConfig() {
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("ENVIRONMENT", "development")
	viper.SetDefault("DATABASE_URL", "host=localhost user=atlas password=atlas_password dbname=atlas port=5432 sslmode=disable")
	// Empty means Atlas generates a signing secret and stores it in the database.
	viper.SetDefault("JWT_SECRET", "")
	viper.SetDefault("REGISTRATION_ENABLED", false)
	viper.SetDefault("TRUSTED_PROXIES", "127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")

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
