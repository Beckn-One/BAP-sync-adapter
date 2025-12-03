package config

import "os"

type Config struct {
	Port          string
	AppEnv        string
	OnixURL       string
	RedisURL      string
	RedisPassword string
}

func Load() *Config {
	return &Config{
		Port:          getEnv("PORT", "3000"),
		AppEnv:        getEnv("APP_ENV", "development"),
		OnixURL:       getEnv("ONIX_URL", "http://localhost:8080"),
		RedisURL:      getEnv("REDIS_URL", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
