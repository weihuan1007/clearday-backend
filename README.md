# ClearDay Backend

Go backend for the ClearDay reminder app.

## Folders

- `backend/`: Go API server and DynamoDB storage code
- `deploy/aws/`: EC2 setup and backend update scripts
- `deploy/scripts/`: local PowerShell deployment helper

## Production Environment

On EC2, create:

```text
/etc/clearday.env
```

Example:

```bash
CLEAR_DAY_ADDR=:8080
CLEAR_DAY_STORE=dynamodb
CLEAR_DAY_DYNAMODB_TABLE=clearday-reminders
CLEAR_DAY_API_TOKEN=replace-with-a-long-random-token
CLEAR_DAY_STATIC_DIR=/opt/clearday/app/web/static
AWS_REGION=ap-southeast-1
```

Use an EC2 IAM instance profile for DynamoDB access. Do not store AWS access keys in the repo or in `/etc/clearday.env`.

## Development Environment

Local development uses an ignored env file:

```text
backend/.env
```

That file is not pushed to GitHub. It should point to the development table only:

```bash
CLEAR_DAY_STORE=dynamodb
CLEAR_DAY_DYNAMODB_TABLE=clearday-reminders-dev
AWS_REGION=ap-southeast-1
```

Use `clearday-reminders-dev` for dummy data and testing. Keep real reminders in `clearday-reminders`.

## Manual Deploy

From PowerShell:

```powershell
.\deploy\scripts\deploy-backend.ps1 -VmHost YOUR_EC2_PUBLIC_DNS -SshKey C:\path\to\reminderapp.pem
```

Use this only for emergency manual deployments.

## GitHub Actions Deployment

The recommended production deployment uses GitHub Actions with AWS Systems Manager, not SSH.

Read:

```text
ssm-github-actions-deploy-guide.md
```

Repository variables:

```text
AWS_REGION=ap-southeast-1
AWS_BACKEND_ROLE_TO_ASSUME
AWS_BACKEND_DEPLOY_BUCKET
AWS_BACKEND_INSTANCE_ID
```

Do not add your EC2 `.pem` private key to GitHub for this flow.

The workflow file is:

```text
.github/workflows/deploy-backend.yml
```
