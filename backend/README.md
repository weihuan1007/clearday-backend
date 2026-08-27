# ClearDay Go Backend

This backend serves the calendar UI and provides the reminder API.

## Folders

- `cmd/clearday-server`: application entry point.
- `internal/config`: environment variable loading.
- `internal/reminders`: reminder model, validation, service, and HTTP handlers.
- `internal/storage/jsonstore`: offline JSON file storage for local testing.
- `internal/storage/dynamodbstore`: AWS DynamoDB storage for development and production.
- `internal/server`: routing, static files, CORS, and bearer-token protection.

## Local Run

Install Go 1.24 or newer, then run:

```powershell
cd D:\reminder-app\repos\backend\backend
go run ./cmd/clearday-server
```

Open `http://localhost:8080`.

The backend automatically reads `D:\reminder-app\repos\backend\backend\.env` when it exists.

For normal development, use the dev DynamoDB table so dummy data stays separate:

```bash
CLEAR_DAY_STORE=dynamodb
CLEAR_DAY_DYNAMODB_TABLE=clearday-reminders-dev
AWS_REGION=ap-southeast-1
```

If you want to test without AWS, temporarily switch the env file to:

```bash
CLEAR_DAY_STORE=json
CLEAR_DAY_JSON_PATH=../data/reminders.json
```

## AWS Production Store

Use DynamoDB in production:

```bash
CLEAR_DAY_STORE=dynamodb
CLEAR_DAY_DYNAMODB_TABLE=clearday-reminders
AWS_REGION=ap-southeast-1
```

On EC2, prefer an IAM instance profile with permission to read and write only the reminder table. Do not store AWS access keys in `/etc/clearday.env`.

Production should use `clearday-reminders`. Development should use `clearday-reminders-dev`.

## API

- `GET /api/health`
- `GET /api/reminders`
- `POST /api/reminders`
- `PUT /api/reminders/{id}`
- `DELETE /api/reminders/{id}`

Set `CLEAR_DAY_API_TOKEN` in production. The browser will ask for the token once and store it locally.
