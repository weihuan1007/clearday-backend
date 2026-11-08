package money

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type Handler struct {
	store         Store
	verifySubject func(context.Context, string) (string, error)
	logger        *slog.Logger
}

func NewHandler(store Store, verifySubject func(context.Context, string) (string, error), logger *slog.Logger) *Handler {
	return &Handler{store: store, verifySubject: verifySubject, logger: logger}
}

func (handler *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "private, no-store")
	token, found := strings.CutPrefix(request.Header.Get("Authorization"), "Bearer ")
	if !found || token == "" {
		writeError(response, http.StatusUnauthorized, "Sign in is required.")
		return
	}
	owner, err := handler.verifySubject(request.Context(), token)
	if err != nil || owner == "" {
		writeError(response, http.StatusUnauthorized, "Sign in is required.")
		return
	}

	const collection = "/api/transactions"
	if request.URL.Path == collection {
		handler.collection(response, request, owner)
		return
	}
	id := strings.TrimPrefix(request.URL.Path, collection+"/")
	if !validID.MatchString(id) {
		writeError(response, http.StatusNotFound, "Transaction not found.")
		return
	}
	handler.item(response, request, owner, id)
}

func (handler *Handler) collection(response http.ResponseWriter, request *http.Request, owner string) {
	switch request.Method {
	case http.MethodGet:
		transactions, err := handler.store.List(request.Context(), owner)
		if err != nil {
			handler.storageError(response, err)
			return
		}
		writeJSON(response, http.StatusOK, transactions)
	case http.MethodPost:
		transaction, err := readTransaction(response, request, "")
		if err != nil {
			writeError(response, http.StatusBadRequest, err.Error())
			return
		}
		created, err := handler.store.Create(request.Context(), owner, transaction)
		if err != nil {
			handler.storageError(response, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeJSON(response, status, struct {
			Created     bool        `json:"created"`
			Transaction Transaction `json:"transaction"`
		}{created, transaction})
	default:
		response.Header().Set("Allow", "GET, POST")
		writeError(response, http.StatusMethodNotAllowed, "Method not allowed.")
	}
}

func (handler *Handler) item(response http.ResponseWriter, request *http.Request, owner, id string) {
	switch request.Method {
	case http.MethodPut:
		transaction, err := readTransaction(response, request, id)
		if err != nil {
			writeError(response, http.StatusBadRequest, err.Error())
			return
		}
		if err := handler.store.Update(request.Context(), owner, transaction); err != nil {
			handler.storageError(response, err)
			return
		}
		writeJSON(response, http.StatusOK, transaction)
	case http.MethodDelete:
		if err := handler.store.Delete(request.Context(), owner, id); err != nil {
			handler.storageError(response, err)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	default:
		response.Header().Set("Allow", "PUT, DELETE")
		writeError(response, http.StatusMethodNotAllowed, "Method not allowed.")
	}
}

func readTransaction(response http.ResponseWriter, request *http.Request, id string) (Transaction, error) {
	var transaction Transaction
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&transaction); err != nil || decoder.Decode(new(any)) != io.EOF {
		return transaction, errors.New("invalid transaction data")
	}
	if id != "" && transaction.ID != id {
		return transaction, errors.New("transaction ID must match the URL")
	}
	err := transaction.validate()
	return transaction, err
}

func (handler *Handler) storageError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNotFound):
		writeError(response, http.StatusNotFound, "Transaction not found.")
	case errors.Is(err, errConflict):
		writeError(response, http.StatusConflict, "A transaction already uses this ID.")
	default:
		if handler.logger != nil {
			handler.logger.Error("money storage request failed", "error", err)
		}
		writeError(response, http.StatusInternalServerError, "Could not access your transactions.")
	}
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
