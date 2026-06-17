package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	System    SystemConfig    `mapstructure:"system"`
	Backup    BackupConfig    `mapstructure:"backup"`
	Installed bool            `mapstructure:"installed"`
}

type ServerConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	PanelPort   int    `mapstructure:"panel_port"`
	Mode        string `mapstructure:"mode"`
	LogLevel    string `mapstructure:"log_level"`
	FrontendDir string `mapstructure:"frontend_dir"`
}

type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

type JWTConfig struct {
	Secret          string `mapstructure:"secret"`
	AccessTokenTTL  int    `mapstructure:"access_token_ttl"`
	RefreshTokenTTL int    `mapstructure:"refresh_token_ttl"`
}

type SystemConfig struct {
	NginxPath     string `mapstructure:"nginx_path"`
	NginxSitesDir string `mapstructure:"nginx_sites_dir"`
	PHPBaseDir    string `mapstructure:"php_base_dir"`
	HomeDirPrefix string `mapstructure:"home_dir_prefix"`
	SSLCertDir    string `mapstructure:"ssl_cert_dir"`
	LogsDir       string `mapstructure:"logs_dir"`
	DataDir       string `mapstructure:"data_dir"`
}

type BackupConfig struct {
	Dir           string `mapstructure:"dir"`
	RetentionDays int    `mapstructure:"retention_days"`
	S3Bucket      string `mapstructure:"s3_bucket"`
	S3Region      string `mapstructure:"s3_region"`
	S3AccessKey   string `mapstructure:"s3_access_key"`
	S3SecretKey   string `mapstructure:"s3_secret_key"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:        "0.0.0.0",
			Port:        8080,
			PanelPort:   2083,
			Mode:        "release",
			LogLevel:    "info",
			FrontendDir: "/var/lib/novapanel/frontend",
		},
		Database: DatabaseConfig{
			Driver:  "sqlite",
			Host:    "localhost",
			Port:    5432,
			User:    "novapanel",
			Name:    "novapanel",
			SSLMode: "disable",
		},
		JWT: JWTConfig{
			Secret:          generateSecret(),
			AccessTokenTTL:  15,
			RefreshTokenTTL: 4320,
		},
		System: SystemConfig{
			NginxPath:     "/etc/nginx",
			NginxSitesDir: "/etc/nginx/sites-enabled",
			PHPBaseDir:    "/etc/php",
			HomeDirPrefix: "/home",
			SSLCertDir:    "/etc/letsencrypt/live",
			LogsDir:       "/var/log/novapanel",
			DataDir:       "/var/lib/novapanel",
		},
		Backup: BackupConfig{
			Dir:           "/var/backups/novapanel",
			RetentionDays: 30,
		},
	}
}

func generateSecret() string {
	return fmt.Sprintf("np_%x", os.Getegid())
}

func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) DSN() string {
	if c.Database.Driver == "sqlite" {
		return filepath.Join(c.System.DataDir, "novapanel.db")
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host, c.Database.Port, c.Database.User, c.Database.Password, c.Database.Name, c.Database.SSLMode)
}
