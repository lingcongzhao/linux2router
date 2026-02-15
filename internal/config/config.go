package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           int
	DataDir        string
	ConfigDir      string
	SessionSecret  string
	SessionMaxAge  int
	DefaultAdmin   string
	DefaultPassword string
}

func Load() *Config {
	cfg := &Config{
		Port:            getEnvInt("ROUTER_PORT", 8090),
		DataDir:         getEnvString("ROUTER_DATA_DIR", "./data"),
		ConfigDir:       getEnvString("ROUTER_CONFIG_DIR", "./configs"),
		SessionSecret:   getEnvString("ROUTER_SESSION_SECRET", "change-me-in-production-32bytes!"),
		SessionMaxAge:   getEnvInt("ROUTER_SESSION_MAX_AGE", 86400), // 24 hours
		DefaultAdmin:    getEnvString("ROUTER_DEFAULT_ADMIN", "admin"),
		DefaultPassword: getEnvString("ROUTER_DEFAULT_PASSWORD", "admin"),
	}

	// Ensure directories exist
	os.MkdirAll(cfg.DataDir, 0755)
	os.MkdirAll(cfg.ConfigDir, 0755)
	os.MkdirAll(cfg.ConfigDir+"/iptables", 0755)
	os.MkdirAll(cfg.ConfigDir+"/routes", 0755)
	os.MkdirAll(cfg.ConfigDir+"/rules", 0755)

	return cfg
}

func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
