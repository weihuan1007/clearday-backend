package money

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestChatEditsUseLedgerValidation(t *testing.T) {
	entry := Transaction{ID: "tg_test", Type: "expense", AmountSen: 2000, Category: "Food",
		Date: "2026-11-24", Account: "Unspecified", Source: "telegram"}
	sent := time.Date(2026, 11, 24, 17, 0, 0, 0, time.UTC) // Already Nov 25 in Malaysia.
	for _, test := range []struct {
		field, value string
		valid        bool
	}{
		{"amount", "RM1,234.56", true}, {"amount", "0.01", true},
		{"amount", "0", false}, {"amount", "-1", false}, {"amount", "1.234", false},
		{"amount", "999999999.99", true}, {"amount", "1000000000", false},
		{"date", "yesterday", true}, {"date", "2026-02-30", false}, {"date", "0000-01-01", false},
		{"note", strings.Repeat("餐", 80), true}, {"note", strings.Repeat("餐", 81), false},
		{"note", "-", true}, {"category", "", false}, {"account", "", false},
		{"type", "income", true}, {"type", "transfer", true}, {"type", "invalid", false},
	} {
		edited, err := EditChatTransaction(entry, test.field, test.value, sent)
		if (err == nil) != test.valid {
			t.Errorf("%s %q: %v", test.field, test.value, err)
		}
		if err == nil && (edited.ID != entry.ID || edited.Source != entry.Source) {
			t.Error("edit changed transaction identity or source")
		}
	}
	edited, _ := EditChatTransaction(entry, "date", "yesterday", sent)
	if edited.Date != "2026-11-24" {
		t.Fatalf("wrong Malaysia date %s", edited.Date)
	}
}

func TestTelegramEditsConditionallyUpdateTheSharedDynamoLedger(t *testing.T) {
	items := map[string]map[string]json.RawMessage{}
	endpoint := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var input struct {
			Item                      map[string]json.RawMessage
			Key                       map[string]json.RawMessage
			ConditionExpression       string
			ExpressionAttributeValues map[string]json.RawMessage
			ExpressionAttributeNames  map[string]string
			ConsistentRead            bool
		}
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
			response.WriteHeader(400)
			return
		}
		response.Header().Set("Content-Type", "application/x-amz-json-1.0")
		switch request.Header.Get("X-Amz-Target") {
		case "DynamoDB_20120810.Query":
			if !input.ConsistentRead {
				t.Error("Telegram reads must see the latest ledger edit")
			}
			key := string(input.ExpressionAttributeValues[":owner"]) + string(input.ExpressionAttributeValues[":id"])
			matches := []map[string]json.RawMessage{}
			if item := items[key]; item != nil {
				matches = append(matches, item)
			}
			_ = json.NewEncoder(response).Encode(map[string]any{"Items": matches})
		case "DynamoDB_20120810.PutItem", "DynamoDB_20120810.DeleteItem":
			target := input.Item
			if target == nil {
				target = input.Key
			}
			key := string(target["owner"]) + string(target["id"])
			current := items[key]
			matches := current != nil
			if input.ConditionExpression == "attribute_not_exists(id)" {
				matches = current == nil
			}
			for alias, field := range input.ExpressionAttributeNames {
				if !strings.Contains(input.ConditionExpression, alias+" = :"+field) {
					t.Errorf("missing original-field guard for %s", field)
				}
				matches = matches && string(current[field]) == string(input.ExpressionAttributeValues[":"+field])
			}
			if input.ConditionExpression != "" && !matches {
				writeConditionalFailure(response, current)
				return
			}
			if input.Item == nil {
				delete(items, key)
			} else {
				items[key] = input.Item
			}
			_, _ = response.Write([]byte("{}"))
		default:
			t.Error("unexpected Dynamo operation")
			response.WriteHeader(400)
		}
	}))
	defer endpoint.Close()
	store := NewDynamoStore("money", aws.Config{
		Region: "ap-southeast-1", BaseEndpoint: aws.String(endpoint.URL), Credentials: aws.AnonymousCredentials{},
	})
	ctx := context.Background()
	if edit, err := store.PendingTelegramEdit(ctx, "alice", 123); err != nil || edit.PromptID != 0 {
		t.Fatalf("missing prompt should be inactive: %#v %v", edit, err)
	}
	before := Transaction{ID: "tg_test", Type: "expense", AmountSen: 2000, Category: "Food",
		Date: "2026-11-24", Account: "Unspecified", Source: "telegram"}
	edit := TelegramEdit{PromptID: 10, SourceID: 1, CardID: 9, Field: "amount", Before: before, Expires: time.Now().Add(time.Minute).Unix()}
	if err := store.SaveTelegramEdit(ctx, "alice", 123, edit); err != nil {
		t.Fatal(err)
	}
	if pending, err := store.PendingTelegramEdit(ctx, "alice", 123); err != nil || pending != edit {
		t.Fatalf("durable prompt failed: %#v %v", pending, err)
	}
	if _, err := store.Create(ctx, "alice", before); err != nil {
		t.Fatal(err)
	}
	if entry, exists, err := store.TelegramTransaction(ctx, "alice", before.ID); err != nil || !exists || entry != before {
		t.Fatalf("incorrect transaction read: %#v %v", entry, err)
	}
	if _, exists, _ := store.TelegramTransaction(ctx, "bob", before.ID); exists {
		t.Error("Telegram read another owner's entry")
	}
	after := before
	after.AmountSen = 2500
	if changed, err := store.ChangeTelegramTransaction(ctx, "alice", before, &after); err != nil || !changed {
		t.Fatalf("edit failed: %t %v", changed, err)
	}
	if changed, err := store.ChangeTelegramTransaction(ctx, "alice", before, &after); err != nil || changed {
		t.Fatalf("stale edit succeeded: %t %v", changed, err)
	}
	if changed, err := store.ChangeTelegramTransaction(ctx, "alice", before, nil); err != nil || changed {
		t.Fatalf("stale deletion succeeded: %t %v", changed, err)
	}
	if changed, err := store.ChangeTelegramTransaction(ctx, "alice", after, nil); err != nil || !changed {
		t.Fatalf("deletion failed: %t %v", changed, err)
	}
	if changed, err := store.ChangeTelegramTransaction(ctx, "alice", after, &before); err != nil || changed {
		t.Fatalf("edit recreated a deleted record: %t %v", changed, err)
	}
}
