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
	Addr               string
	APIToken           string
	Store              string
	JSONPath           string
	DynamoDBTable      string
	MoneyTable         string
	MoneyToken         string
	AWSRegion          string
	StaticDir          string
	ExposeAPIToken     bool
	CognitoUserPoolID  string
	CognitoClientID    string
	CognitoDomain      string
	CognitoRedirectURI string
	CognitoLogoutURI   string
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
		Addr:               getEnv("CLEAR_DAY_ADDR", ":8080"),
		APIToken:           strings.TrimSpace(os.Getenv("CLEAR_DAY_API_TOKEN")),
		Store:              strings.ToLower(getEnv("CLEAR_DAY_STORE", "json")),
		JSONPath:           getEnv("CLEAR_DAY_JSON_PATH", filepath.Join("..", "data", "reminders.json")),
		DynamoDBTable:      strings.TrimSpace(os.Getenv("CLEAR_DAY_DYNAMODB_TABLE")),
		MoneyTable:         strings.TrimSpace(os.Getenv("CLEAR_DAY_MONEY_TABLE")),
		MoneyToken:         strings.TrimSpace(os.Getenv("CLEAR_DAY_MONEY_TOKEN")),
		AWSRegion:          getEnv("AWS_REGION", getEnv("AWS_DEFAULT_REGION", "")),
		StaticDir:          getEnv("CLEAR_DAY_STATIC_DIR", filepath.Join("..", "..", "frontend")),
		ExposeAPIToken:     getEnvBool("CLEAR_DAY_EXPOSE_API_TOKEN", false),
		CognitoUserPoolID:  strings.TrimSpace(os.Getenv("CLEAR_DAY_COGNITO_USER_POOL_ID")),
		CognitoClientID:    strings.TrimSpace(os.Getenv("CLEAR_DAY_COGNITO_CLIENT_ID")),
		CognitoDomain:      strings.TrimRight(strings.TrimSpace(os.Getenv("CLEAR_DAY_COGNITO_DOMAIN")), "/"),
		CognitoRedirectURI: strings.TrimSpace(os.Getenv("CLEAR_DAY_COGNITO_REDIRECT_URI")),
		CognitoLogoutURI:   strings.TrimSpace(os.Getenv("CLEAR_DAY_COGNITO_LOGOUT_URI")),
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

	cognitoValues := []string{
		cfg.CognitoUserPoolID,
		cfg.CognitoClientID,
		cfg.CognitoDomain,
		cfg.CognitoRedirectURI,
		cfg.CognitoLogoutURI,
	}
	cognitoConfigured := 0
	for _, value := range cognitoValues {
		if value != "" {
			cognitoConfigured++
		}
	}
	if cognitoConfigured != 0 && cognitoConfigured != len(cognitoValues) {
		return cfg, errors.New("all CLEAR_DAY_COGNITO_* settings are required when Cognito login is enabled")
	}
	if cognitoConfigured > 0 && cfg.AWSRegion == "" {
		return cfg, errors.New("AWS_REGION is required when Cognito login is enabled")
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

func getEnvBool(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
