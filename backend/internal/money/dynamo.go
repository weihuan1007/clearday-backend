package money

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoStore struct {
	client *dynamodb.Client
	table  *string
}

func NewDynamoStore(table string, cfg aws.Config) *DynamoStore {
	return &DynamoStore{client: dynamodb.NewFromConfig(cfg), table: aws.String(table)}
}

func (store *DynamoStore) List(ctx context.Context, owner string) ([]Transaction, error) {
	pages := dynamodb.NewQueryPaginator(store.client, &dynamodb.QueryInput{
		TableName:              store.table,
		ConsistentRead:         aws.Bool(true),
		KeyConditionExpression: aws.String("#owner = :owner"),
		ExpressionAttributeNames: map[string]string{
			"#owner": "owner",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":owner": &types.AttributeValueMemberS{Value: owner},
		},
	})
	transactions := []Transaction{}
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		var batch []Transaction
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &batch); err != nil {
			return nil, err
		}
		transactions = append(transactions, batch...)
	}
	return transactions, nil
}

func (store *DynamoStore) Create(ctx context.Context, owner string, transaction Transaction) (bool, error) {
	err := store.put(ctx, owner, transaction, "attribute_not_exists(id)", types.ReturnValuesOnConditionCheckFailureAllOld)
	var conflict *types.ConditionalCheckFailedException
	if !errors.As(err, &conflict) {
		return err == nil, err
	}

	// The ID survives form retries and repeat imports. The same payload succeeds
	// without overwriting a record that was since edited.
	var existing Transaction
	if err := attributevalue.UnmarshalMap(conflict.Item, &existing); err != nil {
		return false, err
	}
	if existing == transaction {
		return false, nil
	}
	return false, errConflict
}

func (store *DynamoStore) Update(ctx context.Context, owner string, transaction Transaction) error {
	return ifMissing(store.put(ctx, owner, transaction, "attribute_exists(id)", types.ReturnValuesOnConditionCheckFailureNone))
}

func (store *DynamoStore) put(ctx context.Context, owner string, transaction Transaction, condition string, onFailure types.ReturnValuesOnConditionCheckFailure) error {
	item, err := attributevalue.MarshalMap(transaction)
	if err != nil {
		return err
	}
	item["owner"] = &types.AttributeValueMemberS{Value: owner}
	_, err = store.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:                           store.table,
		Item:                                item,
		ConditionExpression:                 aws.String(condition),
		ReturnValuesOnConditionCheckFailure: onFailure,
	})
	return err
}

func (store *DynamoStore) Delete(ctx context.Context, owner, id string) error {
	_, err := store.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: store.table,
		Key: map[string]types.AttributeValue{
			"owner": &types.AttributeValueMemberS{Value: owner},
			"id":    &types.AttributeValueMemberS{Value: id},
		},
		ConditionExpression: aws.String("attribute_exists(id)"),
	})
	return ifMissing(err)
}

func ifMissing(err error) error {
	var conflict *types.ConditionalCheckFailedException
	if errors.As(err, &conflict) {
		return errNotFound
	}
	return err
}
