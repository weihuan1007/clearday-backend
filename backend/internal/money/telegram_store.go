package money

import (
	"context"
	"errors"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const telegramLinkPartition = "clearday:telegram-link"

// TelegramEdit keeps one active reply prompt per chat. It survives restarts
// without adding interaction metadata to the user's transaction list.
type TelegramEdit struct {
	PromptID int64       `dynamodbav:"promptId"`
	SourceID int64       `dynamodbav:"sourceId"`
	CardID   int64       `dynamodbav:"cardId"`
	Field    string      `dynamodbav:"field"`
	Before   Transaction `dynamodbav:"before"`
	Expires  int64       `dynamodbav:"expires"`
}

func (store *DynamoStore) TelegramTransaction(ctx context.Context, owner, id string) (Transaction, bool, error) {
	item, err := store.telegramItem(ctx, owner, id)
	if err != nil || len(item) == 0 {
		return Transaction{}, false, err
	}
	var entry Transaction
	err = attributevalue.UnmarshalMap(item, &entry)
	return entry, err == nil, err
}

func (store *DynamoStore) PendingTelegramEdit(ctx context.Context, owner string, chat int64) (TelegramEdit, error) {
	item, err := store.telegramItem(ctx, "clearday:telegram-edit:"+owner, strconv.FormatInt(chat, 10))
	if err != nil {
		return TelegramEdit{}, err
	}
	var edit TelegramEdit
	err = attributevalue.UnmarshalMap(item, &edit)
	return edit, err
}

func (store *DynamoStore) SaveTelegramEdit(ctx context.Context, owner string, chat int64, edit TelegramEdit) error {
	item, err := attributevalue.MarshalMap(edit)
	if err != nil {
		return err
	}
	item["owner"] = &types.AttributeValueMemberS{Value: "clearday:telegram-edit:" + owner}
	item["id"] = &types.AttributeValueMemberS{Value: strconv.FormatInt(chat, 10)}
	_, err = store.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: store.table, Item: item})
	return err
}

func (store *DynamoStore) telegramItem(ctx context.Context, owner, id string) (map[string]types.AttributeValue, error) {
	result, err := store.client.Query(ctx, &dynamodb.QueryInput{
		TableName: store.table, ConsistentRead: aws.Bool(true),
		KeyConditionExpression:   aws.String("#owner = :owner AND id = :id"),
		ExpressionAttributeNames: map[string]string{"#owner": "owner"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":owner": &types.AttributeValueMemberS{Value: owner},
			":id":    &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil || len(result.Items) == 0 {
		return nil, err
	}
	return result.Items[0], nil
}

// ChangeTelegramTransaction compares every original field before saving.
// An older card or retried edit cannot overwrite a newer change or recreate
// a transaction that was deleted from the app. A nil replacement deletes it.
func (store *DynamoStore) ChangeTelegramTransaction(ctx context.Context, owner string, before Transaction, replacement *Transaction) (bool, error) {
	original, err := attributevalue.MarshalMap(before)
	if err != nil {
		return false, err
	}
	names := map[string]string{}
	values := map[string]types.AttributeValue{}
	condition := "attribute_exists(id)"
	for _, field := range []string{"type", "amountSen", "date", "category", "account", "note", "source"} {
		names["#"+field] = field
		values[":"+field] = original[field]
		comparison := "#" + field + " = :" + field
		// Older imports can omit the optional note or source attributes.
		if (field == "note" && before.Note == "") || (field == "source" && (before.Source == "" || before.Source == "manual")) {
			comparison = "(" + comparison + " OR attribute_not_exists(#" + field + "))"
		}
		condition += " AND " + comparison
	}
	if replacement == nil {
		_, err = store.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: store.table,
			Key: map[string]types.AttributeValue{
				"owner": &types.AttributeValueMemberS{Value: owner},
				"id":    &types.AttributeValueMemberS{Value: before.ID},
			},
			ConditionExpression: aws.String(condition), ExpressionAttributeNames: names, ExpressionAttributeValues: values,
		})
	} else {
		if replacement.ID != before.ID {
			return false, errors.New("a Telegram edit cannot change the transaction ID")
		}
		if err := replacement.validate(); err != nil {
			return false, err
		}
		item, marshalErr := attributevalue.MarshalMap(replacement)
		if marshalErr != nil {
			return false, marshalErr
		}
		item["owner"] = &types.AttributeValueMemberS{Value: owner}
		_, err = store.client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: store.table, Item: item, ConditionExpression: aws.String(condition),
			ExpressionAttributeNames: names, ExpressionAttributeValues: values,
		})
	}
	var conflict *types.ConditionalCheckFailedException
	if errors.As(err, &conflict) {
		return false, nil
	}
	return err == nil, err
}

// Store the link in a separate partition so it never appears in a user's ledger.
func (store *DynamoStore) LinkedTelegramChat(ctx context.Context, owner string) (int64, error) {
	result, err := store.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              store.table,
		ConsistentRead:         aws.Bool(true),
		KeyConditionExpression: aws.String("#owner = :owner AND id = :id"),
		ExpressionAttributeNames: map[string]string{
			"#owner": "owner",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":owner": &types.AttributeValueMemberS{Value: telegramLinkPartition},
			":id":    &types.AttributeValueMemberS{Value: owner},
		},
	})
	if err != nil || len(result.Items) == 0 {
		return 0, err
	}
	var link struct {
		ChatID int64 `dynamodbav:"chatId"`
	}
	err = attributevalue.UnmarshalMap(result.Items[0], &link)
	return link.ChatID, err
}

func (store *DynamoStore) LinkTelegramChat(ctx context.Context, owner string, chatID int64) error {
	_, err := store.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: store.table,
		Item: map[string]types.AttributeValue{
			"owner":  &types.AttributeValueMemberS{Value: telegramLinkPartition},
			"id":     &types.AttributeValueMemberS{Value: owner},
			"chatId": &types.AttributeValueMemberN{Value: strconv.FormatInt(chatID, 10)},
		},
	})
	return err
}

// Save the message marker and record atomically. The marker survives a user
// deleting the transaction, so a replay cannot bring the transaction back.
func (store *DynamoStore) RecordTelegramTransaction(ctx context.Context, owner string, transaction Transaction) (bool, error) {
	if err := transaction.validate(); err != nil {
		return false, err
	}
	item, err := attributevalue.MarshalMap(transaction)
	if err != nil {
		return false, err
	}
	item["owner"] = &types.AttributeValueMemberS{Value: owner}
	marker := map[string]types.AttributeValue{
		"owner": &types.AttributeValueMemberS{Value: "clearday:telegram-message:" + owner},
		"id":    &types.AttributeValueMemberS{Value: transaction.ID},
	}
	_, err = store.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{Put: &types.Put{TableName: store.table, Item: marker, ConditionExpression: aws.String("attribute_not_exists(id)")}},
			{Put: &types.Put{TableName: store.table, Item: item, ConditionExpression: aws.String("attribute_not_exists(id)")}},
		},
	})
	var canceled *types.TransactionCanceledException
	if errors.As(err, &canceled) {
		for _, reason := range canceled.CancellationReasons {
			if aws.ToString(reason.Code) == "ConditionalCheckFailed" {
				return false, nil
			}
		}
	}
	return err == nil, err
}
