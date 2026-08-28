package reminders

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (handler *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(request.URL.Path, "/api/reminders")
	if path == "" || path == "/" {
		handler.collection(response, request)
		return
	}

	id := strings.Trim(strings.TrimPrefix(path, "/"), "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(response, http.StatusNotFound, "Unknown reminder route.")
		return
	}

	handler.item(response, request, id)
}

func (handler *Handler) collection(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		items, err := handler.service.List(request.Context())
		if err != nil {
			handler.logError("list reminders", err)
			writeError(response, http.StatusInternalServerError, "Could not load reminders.")
			return
		}
		writeJSON(response, http.StatusOK, items)
	case http.MethodPost:
		var input Input
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			writeError(response, http.StatusBadRequest, "Request body must be valid JSON.")
			return
		}
		item, err := handler.service.Create(request.Context(), input)
		if err != nil {
			handler.logError("create reminder", err)
			writeServiceError(response, err)
			return
		}
		writeJSON(response, http.StatusCreated, item)
	default:
		response.Header().Set("Allow", "GET, POST")
		writeError(response, http.StatusMethodNotAllowed, "Method is not allowed.")
	}
}

func (handler *Handler) item(response http.ResponseWriter, request *http.Request, id string) {
	switch request.Method {
	case http.MethodPut, http.MethodPost:
		var input Input
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			writeError(response, http.StatusBadRequest, "Request body must be valid JSON.")
			return
		}
		item, err := handler.service.Update(request.Context(), id, input)
		if err != nil {
			handler.logError("update reminder", err)
			writeServiceError(response, err)
			return
		}
		writeJSON(response, http.StatusOK, item)
	case http.MethodDelete:
		if err := handler.service.Delete(request.Context(), id); err != nil {
			handler.logError("delete reminder", err)
			writeServiceError(response, err)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	default:
		response.Header().Set("Allow", "POST, PUT, DELETE")
		writeError(response, http.StatusMethodNotAllowed, "Method is not allowed.")
	}
}

func (handler *Handler) logError(message string, err error) {
	if handler.logger != nil {
		handler.logger.Error(message, "error", err)
	}
}

func writeServiceError(response http.ResponseWriter, err error) {
	var validationErr ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeJSON(response, http.StatusUnprocessableEntity, map[string]any{
			"error":  validationErr.Error(),
			"fields": validationErr.Fields,
		})
	case errors.Is(err, ErrNotFound):
		writeError(response, http.StatusNotFound, "Reminder was not found.")
	default:
		writeError(response, http.StatusInternalServerError, "Something went wrong.")
	}
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}
