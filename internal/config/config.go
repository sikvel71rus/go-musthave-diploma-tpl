package config

import (
	"flag"
	"os"
)

type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
}

func Parse() Config {
	cfg := Config{}
	flag.StringVar(&cfg.RunAddress, "a", "localhost:8080", "address to run HTTP server")
	flag.StringVar(&cfg.DatabaseURI, "d", "", "database connection URI")
	flag.StringVar(&cfg.AccrualSystemAddress, "r", "", "accrual system address")
	flag.Parse()

	if value := os.Getenv("RUN_ADDRESS"); value != "" {
		cfg.RunAddress = value
	}
	if value := os.Getenv("DATABASE_URI"); value != "" {
		cfg.DatabaseURI = value
	}
	if value := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); value != "" {
		cfg.AccrualSystemAddress = value
	}

	return cfg
}
