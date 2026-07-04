package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

//handle and provide configurations

type Config struct {
	DBUser          string
	DBPass          string
	DBHost          string
	DBPort          string
	DBName          string
	RunWithDatabase bool
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	runWithDB, _ := strconv.ParseBool(os.Getenv("RUN_WITH_DATABASE"))

	//return to database.go the config
	return &Config{
		DBUser:          os.Getenv("DB_USER"),
		DBPass:          os.Getenv("DB_PASS"),
		DBHost:          os.Getenv("DB_HOST"),
		DBPort:          os.Getenv("DB_PORT"),
		DBName:          os.Getenv("DB_NAME"),
		RunWithDatabase: runWithDB, //testing purposes
	}, nil
}