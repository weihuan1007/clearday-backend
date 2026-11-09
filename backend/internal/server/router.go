package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

type Options struct {
	APIHandler          http.Handler
	MoneyHandler        http.Handler
	TelegramPairHandler http.Handler
	APIToken            string
	TokenVerifier       TokenVerifier
	RuntimeAuth         RuntimeAuthConfig
	ExposeAPIToken      bool
	StaticDir           string
	Logger              *slog.Logger
}

type TokenVerifier interface {
	Verify(context.Context, string) error
}

type RuntimeAuthConfig struct {
	Domain      string `json:"domain"`
	ClientID    string `json:"clientId"`
	RedirectURI string `json:"redirectUri"`
	LogoutURI   string `json:"logoutUri"`
}

func NewRouter(options Options) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(response http.ResponseWriter, request *http.Request) {
		writeCORS(response)
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(`{"status":"ok"}` + "\n"))
	})

	mux.HandleFunc("/config.js", serveRuntimeConfig(options))
	mux.Handle("/api/reminders", withCORS(withBearerAuth(options.TokenVerifier, options.APIToken, options.APIHandler)))
	mux.Handle("/api/reminders/", withCORS(withBearerAuth(options.TokenVerifier, options.APIToken, options.APIHandler)))
	if options.MoneyHandler != nil {
		mux.Handle("/api/transactions", withCORS(options.MoneyHandler))
		mux.Handle("/api/transactions/", withCORS(options.MoneyHandler))
	}
	if options.TelegramPairHandler != nil {
		mux.Handle("/api/integrations/telegram/pair", withCORS(options.TelegramPairHandler))
	}

	if options.StaticDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(options.StaticDir)))
	}

	return logRequests(options.Logger, mux)
}

func serveRuntimeConfig(options Options) http.HandlerFunc {
	type runtimeConfig struct {
		APIBase  string             `json:"apiBase"`
		Auth     *RuntimeAuthConfig `json:"auth,omitempty"`
		APIToken string             `json:"apiToken,omitempty"`
	}

	cfg := runtimeConfig{APIBase: "/api"}
	if options.TokenVerifier != nil {
		cfg.Auth = &options.RuntimeAuth
	} else if options.ExposeAPIToken && options.APIToken != "" {
		cfg.APIToken = options.APIToken
	}

	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		payload, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			return
		}

		response.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		response.Header().Set("Cache-Control", "no-cache")
		_, _ = response.Write([]byte("window.CLEARDAY_CONFIG = "))
		_, _ = response.Write(payload)
		_, _ = response.Write([]byte(";\n"))
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		writeCORS(response)
		if request.Method == http.MethodOptions {
			response.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(response, request)
	})
}

func writeCORS(response http.ResponseWriter) {
	response.Header().Set("Access-Control-Allow-Origin", "*")
	response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
}

func withBearerAuth(verifier TokenVerifier, token string, next http.Handler) http.Handler {
	if verifier == nil && token == "" {
		return next
	}

	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		header := request.Header.Get("Authorization")
		actual, found := strings.CutPrefix(header, "Bearer ")
		authorized := found && actual != ""
		if authorized && verifier != nil {
			authorized = verifier.Verify(request.Context(), actual) == nil
		} else if authorized {
			authorized = subtle.ConstantTimeCompare([]byte(actual), []byte(token)) == 1
		}
		if !authorized {
			response.Header().Set("Content-Type", "application/json")
			response.WriteHeader(http.StatusUnauthorized)
			_, _ = response.Write([]byte(`{"error":"Sign in is required."}` + "\n"))
			return
		}
		next.ServeHTTP(response, request)
	})
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		return next
	}

	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		logger.Info("request", "method", request.Method, "path", request.URL.Path)
		next.ServeHTTP(response, request)
	})
}
