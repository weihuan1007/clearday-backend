package dynamodbstore

import (
	"context"
	"errors"
	"time"

	"clearday/backend/internal/reminders"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Store struct {
	client    *dynamodb.Client
	tableName string
}

type record struct {
	ID             string `dynamodbav:"id"`
	Title          string `dynamodbav:"title"`
	Category       string `dynamodbav:"category"`
	Date           string `dynamodbav:"date"`
	Time           string `dynamodbav:"time,omitempty"`
	Start          string `dynamodbav:"start,omitempty"`
	StartTime      string `dynamodbav:"startTime,omitempty"`
	StartTimeTitle string `dynamodbav:"StartTime,omitempty"`
	StartTimeSnake string `dynamodbav:"start_time,omitempty"`
	End            string `dynamodbav:"end,omitempty"`
	EndTime        string `dynamodbav:"endTime,omitempty"`
	EndTimeTitle   string `dynamodbav:"EndTime,omitempty"`
	EndTimeSnake   string `dynamodbav:"end_time,omitempty"`
	Notes          string `dynamodbav:"notes"`
	IsDone         bool   `dynamodbav:"isDone"`
	CreatedAt      string `dynamodbav:"createdAt"`
	UpdatedAt      string `dynamodbav:"updatedAt"`
}

func Open(tableName string, cfg aws.Config) (*Store, error) {
	if tableName == "" {
		return nil, errors.New("dynamodb table name is required")
	}

	return &Store{
		client:    dynamodb.NewFromConfig(cfg),
		tableName: tableName,
	}, nil
}

func (store *Store) List(ctx context.Context) ([]reminders.Reminder, error) {
	paginator := dynamodb.NewScanPaginator(store.client, &dynamodb.ScanInput{
		TableName: aws.String(store.tableName),
	})

	var items []reminders.Reminder
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		var records []record
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &records); err != nil {
			return nil, err
		}

		for _, item := range records {
			items = append(items, item.toReminder())
		}
	}

	return items, nil
}

func (store *Store) Get(ctx context.Context, id string) (reminders.Reminder, error) {
	output, err := store.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(store.tableName),
		Key:       keyForID(id),
	})
	if err != nil {
		return reminders.Reminder{}, err
	}

	if len(output.Item) == 0 {
		return reminders.Reminder{}, reminders.ErrNotFound
	}

	var item record
	if err := attributevalue.UnmarshalMap(output.Item, &item); err != nil {
		return reminders.Reminder{}, err
	}

	return item.toReminder(), nil
}

func (store *Store) Create(ctx context.Context, reminder reminders.Reminder) (reminders.Reminder, error) {
	item, err := attributevalue.MarshalMap(fromReminder(reminder))
	if err != nil {
		return reminders.Reminder{}, err
	}

	_, err = store.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(store.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	})
	return reminder, err
}

func (store *Store) Update(ctx context.Context, reminder reminders.Reminder) (reminders.Reminder, error) {
	item, err := attributevalue.MarshalMap(fromReminder(reminder))
	if err != nil {
		return reminders.Reminder{}, err
	}

	_, err = store.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(store.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_exists(id)"),
	})
	if isConditionalFailure(err) {
		return reminders.Reminder{}, reminders.ErrNotFound
	}

	return reminder, err
}

func (store *Store) Delete(ctx context.Context, id string) error {
	_, err := store.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName:           aws.String(store.tableName),
		Key:                 keyForID(id),
		ConditionExpression: aws.String("attribute_exists(id)"),
	})
	if isConditionalFailure(err) {
		return reminders.ErrNotFound
	}

	return err
}

func keyForID(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: id},
	}
}

func fromReminder(reminder reminders.Reminder) record {
	start := firstNonEmpty(reminder.Time, reminder.StartTime)

	return record{
		ID:        reminder.ID,
		Title:     reminder.Title,
		Category:  reminder.Category,
		Date:      reminder.Date,
		Time:      start,
		StartTime: start,
		EndTime:   reminder.EndTime,
		Notes:     reminder.Notes,
		IsDone:    reminder.IsDone,
		CreatedAt: formatTime(reminder.CreatedAt),
		UpdatedAt: formatTime(reminder.UpdatedAt),
	}
}

func (item record) toReminder() reminders.Reminder {
	start := firstNonEmpty(item.Time, item.StartTime, item.StartTimeTitle, item.StartTimeSnake, item.Start)
	end := firstNonEmpty(item.EndTime, item.EndTimeTitle, item.EndTimeSnake, item.End)

	return reminders.Reminder{
		ID:        item.ID,
		Title:     item.Title,
		Category:  item.Category,
		Date:      item.Date,
		Time:      start,
		StartTime: start,
		EndTime:   end,
		Notes:     item.Notes,
		IsDone:    item.IsDone,
		CreatedAt: parseTime(item.CreatedAt),
		UpdatedAt: parseTime(item.UpdatedAt),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func isConditionalFailure(err error) bool {
	var conditionErr *types.ConditionalCheckFailedException
	return errors.As(err, &conditionErr)
}
