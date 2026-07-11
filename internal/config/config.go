package config

import (
	"os"
)

type Config struct {
	ServerAddress string
	MasterURL     string
	Slave1URL     string
	Slave2URL     string
	Slave3URL     string
	Slave4URL     string
}

func Load() Config {
	return Config{
		ServerAddress: getEnv("SERVER_ADDRESS", ":8080"),
		MasterURL:     getEnv("MASTER_URL", "http://localhost:8080"),
		Slave1URL:     getEnv("SLAVE1_URL", "http://localhost:8081"),
		Slave2URL:     getEnv("SLAVE2_URL", "http://localhost:5000"),
		Slave3URL:     getEnv("SLAVE3_URL", "http://localhost:8082"),
		Slave4URL:     getEnv("SLAVE4_URL", "http://localhost:5001"),
	}
}

func LoadSlave() Config {
	return Config{
		ServerAddress: getEnv("SLAVE_SERVER_ADDRESS", ":8081"),
		MasterURL:     getEnv("MASTER_URL", "http://localhost:8080"),
		Slave1URL:     getEnv("SLAVE1_URL", "http://localhost:8081"),
		Slave2URL:     getEnv("SLAVE2_URL", "http://localhost:5000"),
		Slave3URL:     getEnv("SLAVE3_URL", "http://localhost:8082"),
		Slave4URL:     getEnv("SLAVE4_URL", "http://localhost:5001"),
	}
}

func LoadSlave3() Config {
	return Config{
		ServerAddress: getEnv("SLAVE3_SERVER_ADDRESS", ":8082"),
		MasterURL:     getEnv("MASTER_URL", "http://localhost:8080"),
		Slave1URL:     getEnv("SLAVE1_URL", "http://localhost:8081"),
		Slave2URL:     getEnv("SLAVE2_URL", "http://localhost:5000"),
		Slave3URL:     getEnv("SLAVE3_URL", "http://localhost:8082"),
		Slave4URL:     getEnv("SLAVE4_URL", "http://localhost:5001"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
