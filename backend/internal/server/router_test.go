package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type verifierStub struct {
	acceptedToken string
}

func (stub verifierStub) Verify(_ context.Context, token string) error {
	if token != stub.acceptedToken {
		return errors.New("invalid token")
	}
	return nil
}

func TestRouterUsesCognitoVerifierInsteadOfStaticToken(t *testing.T) {
	handler := NewRouter(Options{
		APIHandler: http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusNoContent)
		}),
		APIToken:      "legacy-token",
		TokenVerifier: verifierStub{acceptedToken: "cognito-token"},
	})

	legacyResponse := performAuthorizedRequest(handler, "legacy-token")
	if legacyResponse.Code != http.StatusUnauthorized {
		t.Fatalf("legacy token status = %d, want %d", legacyResponse.Code, http.StatusUnauthorized)
	}

	cognitoResponse := performAuthorizedRequest(handler, "cognito-token")
	if cognitoResponse.Code != http.StatusNoContent {
		t.Fatalf("Cognito token status = %d, want %d", cognitoResponse.Code, http.StatusNoContent)
	}
}

func TestRuntimeConfigDoesNotExposeStaticTokenWhenCognitoIsEnabled(t *testing.T) {
	handler := NewRouter(Options{
		APIHandler:     http.NotFoundHandler(),
		APIToken:       "do-not-expose",
		ExposeAPIToken: true,
		TokenVerifier:  verifierStub{acceptedToken: "valid"},
		RuntimeAuth: RuntimeAuthConfig{
			Domain:      "https://login.example.com",
			ClientID:    "client-1",
			RedirectURI: "https://app.example.com/",
			LogoutURI:   "https://app.example.com/",
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/config.js", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("config status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	if strings.Contains(body, "do-not-expose") {
		t.Fatal("runtime config exposed the legacy API token")
	}
	if !strings.Contains(body, `"clientId": "client-1"`) {
		t.Fatalf("runtime config did not contain Cognito client: %s", body)
	}
}

func performAuthorizedRequest(handler http.Handler, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/reminders", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
