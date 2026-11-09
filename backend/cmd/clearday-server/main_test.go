package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"clearday/backend/internal/config"
	"clearday/backend/internal/server"
)

func TestPersonalMoneyRouteRequiresSeparateKey(t *testing.T) {
	key := strings.Repeat("ab", 32)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, test := range []struct {
		name, moneyKey, reminderKey string
		status                      int
	}{
		{"missing key", "", "reminder-token", http.StatusNotFound},
		{"shared key", key, key, http.StatusNotFound},
		{"separate key", key, "reminder-token", http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.Config{
				MoneyTable: "test-money",
				MoneyToken: test.moneyKey,
				APIToken:   test.reminderKey,
				AWSRegion:  "ap-southeast-1",
			}
			moneyHandler, _ := openMoneyHandler(context.Background(), cfg, nil, logger)
			router := server.NewRouter(server.Options{
				APIHandler:   http.NotFoundHandler(),
				MoneyHandler: moneyHandler,
				APIToken:     cfg.APIToken,
			})
			for _, token := range []string{"", test.reminderKey} {
				req := httptest.NewRequest(http.MethodGet, "/api/transactions", nil)
				if token != "" {
					req.Header.Set("Authorization", "Bearer "+token)
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				if response.Code != test.status {
					t.Fatalf("Money returned %d, want %d", response.Code, test.status)
				}
			}
			health := httptest.NewRecorder()
			router.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/health", nil))
			if health.Code != http.StatusOK {
				t.Fatalf("health returned %d", health.Code)
			}
		})
	}
}
