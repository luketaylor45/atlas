package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Port         string `mapstructure:"PORT"`
	CoreURL      string `mapstructure:"CORE_URL"`
	NodeToken    string `mapstructure:"NODE_TOKEN"`
	SFTPPort     string `mapstructure:"SFTP_PORT"`
	DataPath     string `mapstructure:"DATA_PATH"`
	HostDataPath string `mapstructure:"HOST_DATA_PATH"`
	ServerUID    int    `mapstructure:"SERVER_UID"`
	ServerGID    int    `mapstructure:"SERVER_GID"`
}

var NodeConfig Config

func LoadConfig() {
	viper.SetDefault("PORT", "8081")
	viper.SetDefault("CORE_URL", "http://localhost:8080")
	viper.SetDefault("NODE_TOKEN", "change-me")
	viper.SetDefault("SFTP_PORT", "2022")
	viper.SetDefault("DATA_PATH", "/var/lib/atlas/data")
	viper.SetDefault("HOST_DATA_PATH", "")
	viper.SetDefault("SERVER_UID", 1000)
	viper.SetDefault("SERVER_GID", 1000)

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

	if NodeConfig.HostDataPath == "" {
		NodeConfig.HostDataPath = NodeConfig.DataPath
	}
	if NodeConfig.ServerUID < 0 || NodeConfig.ServerGID < 0 {
		log.Fatal("SERVER_UID and SERVER_GID must be non-negative integers")
	}
}
