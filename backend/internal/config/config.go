package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Addr          string
	APIToken      string
	Store         string
	JSONPath      string
	DynamoDBTable string
	AWSRegion     string
	StaticDir     string
}

type InvalidStoreError struct {
	Store string
}

func (err InvalidStoreError) Error() string {
	return fmt.Sprintf("unsupported CLEAR_DAY_STORE %q", err.Store)
}

func Load() (Config, error) {
	for _, envPath := range []string{".env", filepath.Join("backend", ".env")} {
		if err := loadDotEnv(envPath); err != nil {
			return Config{}, err
		}
	}

	cfg := Config{
		Addr:          getEnv("CLEAR_DAY_ADDR", ":8080"),
		APIToken:      strings.TrimSpace(os.Getenv("CLEAR_DAY_API_TOKEN")),
		Store:         strings.ToLower(getEnv("CLEAR_DAY_STORE", "json")),
		JSONPath:      getEnv("CLEAR_DAY_JSON_PATH", filepath.Join("..", "data", "reminders.json")),
		DynamoDBTable: strings.TrimSpace(os.Getenv("CLEAR_DAY_DYNAMODB_TABLE")),
		AWSRegion:     getEnv("AWS_REGION", getEnv("AWS_DEFAULT_REGION", "")),
		StaticDir:     getEnv("CLEAR_DAY_STATIC_DIR", filepath.Join("..", "..", "frontend")),
	}

	if cfg.Store == "" {
		cfg.Store = "json"
	}

	if cfg.Store == "dynamodb" && cfg.DynamoDBTable == "" {
		return cfg, errors.New("CLEAR_DAY_DYNAMODB_TABLE is required when CLEAR_DAY_STORE=dynamodb")
	}

	if cfg.Store == "dynamodb" && cfg.AWSRegion == "" {
		return cfg, errors.New("AWS_REGION is required when CLEAR_DAY_STORE=dynamodb")
	}

	return cfg, nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimPrefix(strings.TrimSpace(scanner.Text()), "\ufeff")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = trimEnvValue(strings.TrimSpace(value))
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from %s: %w", key, path, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	return nil
}

func trimEnvValue(value string) string {
	if len(value) < 2 {
		return value
	}

	quote := value[0]
	if (quote == '"' || quote == '\'') && value[len(value)-1] == quote {
		return value[1 : len(value)-1]
	}

	return value
}

func getEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
