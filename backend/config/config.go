package config

import (
	"os"

	"github.com/joho/godotenv"
)

//handle and provide configurations

type Config struct {
	DBUser string
	DBPass string
	DBHost string
	DBPort string
	DBName string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	//return to database.go the config
	return &Config{
		DBUser: os.Getenv("DB_USER"),
		DBPass: os.Getenv("DB_PASS"),
		DBHost: os.Getenv("DB_HOST"),
		DBPort: os.Getenv("DB_PORT"),
		DBName: os.Getenv("DB_NAME"),
	}, nil
}