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
