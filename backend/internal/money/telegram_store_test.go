package money

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestTelegramStoreAtomicallyDeduplicatesDeletedRecords(t *testing.T) {
	const owner = "cognito-owner"
	const id = "tg_test"
	var marker, transaction bool
	var state sync.Mutex
	endpoint := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		state.Lock()
		defer state.Unlock()
		response.Header().Set("Content-Type", "application/x-amz-json-1.0")
		switch request.Header.Get("X-Amz-Target") {
		case "DynamoDB_20120810.TransactWriteItems":
			var input struct {
				TransactItems []struct {
					Put struct {
						ConditionExpression string
						Item                map[string]map[string]string
					}
				}
			}
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil || len(input.TransactItems) != 2 {
				t.Error("expected a single atomic two-item write")
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			for index, partition := range []string{"clearday:telegram-message:" + owner, owner} {
				put := input.TransactItems[index].Put
				if put.ConditionExpression != "attribute_not_exists(id)" || put.Item["owner"]["S"] != partition ||
					put.Item["id"]["S"] != id {
					t.Errorf("unexpected transactional item for partition %s", partition)
				}
			}
			if marker {
				response.WriteHeader(http.StatusBadRequest)
				_, _ = response.Write([]byte(`{"__type":"TransactionCanceledException","CancellationReasons":[{"Code":"ConditionalCheckFailed"},{"Code":"None"}]}`))
				return
			}
			marker, transaction = true, true
		case "DynamoDB_20120810.DeleteItem":
			var input struct {
				Key map[string]map[string]string
			}
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil ||
				input.Key["owner"]["S"] != owner || input.Key["id"]["S"] != id || !transaction {
				t.Error("delete should only target the transaction")
			}
			transaction = false
		default:
			t.Error("unexpected DynamoDB operation")
		}
		_, _ = response.Write([]byte("{}"))
	}))
	defer endpoint.Close()

	store := NewDynamoStore("money", aws.Config{
		Region: "ap-southeast-1", BaseEndpoint: aws.String(endpoint.URL), Credentials: aws.AnonymousCredentials{},
	})
	entry := Transaction{ID: id, Type: "expense", AmountSen: 2000, Category: "Food", Date: "2026-11-09", Account: "Unspecified"}
	if created, err := store.RecordTelegramTransaction(context.Background(), owner, entry); err != nil || !created {
		t.Fatalf("initial delivery: created %t, error %v", created, err)
	}
	if err := store.Delete(context.Background(), owner, id); err != nil {
		t.Fatal(err)
	}
	if created, err := store.RecordTelegramTransaction(context.Background(), owner, entry); err != nil || created {
		t.Fatalf("replayed delivery: created %t, error %v", created, err)
	}
}
