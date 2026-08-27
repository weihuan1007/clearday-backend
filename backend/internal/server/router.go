package server

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

type Options struct {
	APIHandler http.Handler
	APIToken   string
	StaticDir  string
	Logger     *slog.Logger
}

func NewRouter(options Options) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(response http.ResponseWriter, request *http.Request) {
		writeCORS(response)
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(`{"status":"ok"}` + "\n"))
	})

	mux.Handle("/api/reminders", withCORS(withBearerAuth(options.APIToken, options.APIHandler)))
	mux.Handle("/api/reminders/", withCORS(withBearerAuth(options.APIToken, options.APIHandler)))

	if options.StaticDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(options.StaticDir)))
	}

	return logRequests(options.Logger, mux)
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

func withBearerAuth(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}

	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		header := request.Header.Get("Authorization")
		actual, found := strings.CutPrefix(header, "Bearer ")
		if !found || subtle.ConstantTimeCompare([]byte(actual), []byte(token)) != 1 {
			response.Header().Set("Content-Type", "application/json")
			response.WriteHeader(http.StatusUnauthorized)
			_, _ = response.Write([]byte(`{"error":"API token required."}` + "\n"))
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
