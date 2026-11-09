# ClearDay Backend

Go backend for the ClearDay reminder app.

For the Money tab's DynamoDB, EC2, and frontend rollout, see the [Money deployment guide](money-aws-deploy-guide.md).

For personal transaction capture in Telegram, see the [Telegram Money guide](telegram-money-guide.md).

## Folders

- `backend/`: Go API server and DynamoDB storage code
- `deploy/aws/`: EC2 setup and backend update scripts
- `deploy/terraform-cognito/`: isolated personal-login infrastructure
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
CLEAR_DAY_STATIC_DIR=/opt/clearday/app/web/static
AWS_REGION=ap-southeast-1
CLEAR_DAY_COGNITO_USER_POOL_ID=ap-southeast-1_example
CLEAR_DAY_COGNITO_CLIENT_ID=example-client-id
CLEAR_DAY_COGNITO_DOMAIN=https://example.auth.ap-southeast-1.amazoncognito.com
CLEAR_DAY_COGNITO_REDIRECT_URI=https://example.cloudfront.net/
CLEAR_DAY_COGNITO_LOGOUT_URI=https://example.cloudfront.net/
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
CLEAR_DAY_EXPOSE_API_TOKEN=true
```

Use `clearday-reminders-dev` for dummy data and testing. Keep real reminders in `clearday-reminders`.

`CLEAR_DAY_EXPOSE_API_TOKEN=true` is a local-only convenience. Keep it off in production. The Money API accepts the separate private Money key or a verified Cognito access token.

Follow `cognito-login-guide.md` to create the private login and migrate production safely.

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
