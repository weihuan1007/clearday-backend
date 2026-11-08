package money

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestMoneyRequestsAreScopedToTheVerifiedOwner(t *testing.T) {
	const record = `{"id":"legacy-id","type":"expense","amountSen":235,"date":"2026-11-08","category":"Food","account":"Cash"}`
	const secondOwnerRecord = `{"id":"legacy-id","type":"income","amountSen":1000,"date":"2026-11-08","category":"Salary","account":"Bank"}`
	const ownerOverride = `{"owner":"alice","id":"other","type":"expense","amountSen":235,"date":"2026-11-08","category":"Food","account":"Cash"}`
	const path = "/api/transactions"

	// Exercise the actual DynamoDB adapter against a local AWS JSON endpoint.
	// Matching only on owner and ID lets the test catch unscoped reads or writes.
	items := map[string]map[string]json.RawMessage{}
	awsEndpoint := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Item                      map[string]json.RawMessage
			Key                       map[string]json.RawMessage
			ExpressionAttributeValues map[string]json.RawMessage
			ConditionExpression       string
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "application/x-amz-json-1.0")
		switch request.Header.Get("X-Amz-Target") {
		case "DynamoDB_20120810.Query":
			owner := string(input.ExpressionAttributeValues[":owner"])
			matching := []map[string]json.RawMessage{}
			for _, item := range items {
				if string(item["owner"]) == owner {
					matching = append(matching, item)
				}
			}
			_ = json.NewEncoder(response).Encode(map[string]any{"Items": matching})
		case "DynamoDB_20120810.PutItem":
			key := string(input.Item["owner"]) + string(input.Item["id"])
			item, exists := items[key]
			if (input.ConditionExpression == "attribute_not_exists(id)" && exists) ||
				(input.ConditionExpression == "attribute_exists(id)" && !exists) {
				writeConditionalFailure(response, item)
				return
			}
			items[key] = input.Item
			_, _ = io.WriteString(response, "{}")
		case "DynamoDB_20120810.DeleteItem":
			key := string(input.Key["owner"]) + string(input.Key["id"])
			if _, exists := items[key]; !exists {
				writeConditionalFailure(response, nil)
				return
			}
			delete(items, key)
			_, _ = io.WriteString(response, "{}")
		default:
			t.Errorf("unexpected DynamoDB operation %q", request.Header.Get("X-Amz-Target"))
			response.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer awsEndpoint.Close()

	store := NewDynamoStore("money", aws.Config{
		Region:       "ap-southeast-1",
		BaseEndpoint: aws.String(awsEndpoint.URL),
		Credentials:  aws.AnonymousCredentials{},
	})
	handler := NewHandler(store, func(_ context.Context, token string) (string, error) {
		if token == "alice" || token == "bob" {
			return token, nil
		}
		return "", errors.New("invalid test token")
	}, nil)

	tests := []struct {
		name, method, path, token, body string
		status                          int
		contains                        string
	}{
		{"unsigned", "GET", path, "", "", 401, ""},
		{"shared token rejected", "GET", path, "static-token", "", 401, ""},
		{"create", "POST", path, "alice", record, 201, `"created":true`},
		{"retry same record", "POST", path, "alice", record, 200, `"created":false`},
		{"preserve changed record", "POST", path, "alice", secondOwnerRecord, 409, ""},
		{"other user sees nothing", "GET", path, "bob", "", 200, "[]"},
		{"other user cannot update", "PUT", path + "/legacy-id", "bob", secondOwnerRecord, 404, ""},
		{"other user cannot delete", "DELETE", path + "/legacy-id", "bob", "", 404, ""},
		{"request cannot choose owner", "POST", path, "bob", ownerOverride, 400, ""},
		{"second user can reuse ID", "POST", path, "bob", secondOwnerRecord, 201, ""},
		{"original user's amount preserved", "GET", path, "alice", "", 200, `"amountSen":235`},
		{"second user's amount preserved", "GET", path, "bob", "", 200, `"amountSen":1000`},
		{"owner can update", "PUT", path + "/legacy-id", "alice", secondOwnerRecord, 200, `"amountSen":1000`},
		{"owner can delete", "DELETE", path + "/legacy-id", "alice", "", 204, ""},
		{"second user's item still exists", "GET", path, "bob", "", 200, `"amountSen":1000`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("got %d %s, want %d containing %q", response.Code, response.Body.String(), test.status, test.contains)
			}
			if got := response.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Fatalf("unexpected cache header %q", got)
			}
		})
	}
}

func writeConditionalFailure(response http.ResponseWriter, item map[string]json.RawMessage) {
	response.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(response).Encode(map[string]any{
		"__type":  "ConditionalCheckFailedException",
		"message": "condition failed",
		"Item":    item,
	})
}
