package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Port      string `mapstructure:"PORT"`
	CoreURL   string `mapstructure:"CORE_URL"`
	NodeToken string `mapstructure:"NODE_TOKEN"`
	SFTPPort  string `mapstructure:"SFTP_PORT"`
}

var NodeConfig Config

func LoadConfig() {
	viper.SetDefault("PORT", "8081")
	viper.SetDefault("CORE_URL", "http://localhost:8080")
	viper.SetDefault("NODE_TOKEN", "change-me")
	viper.SetDefault("SFTP_PORT", "2022")

	viper.SetConfigName("config")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Printf("Error reading config file: %v", err)
		}
	}

	if err := viper.Unmarshal(&NodeConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}
}
