# AWS Deployment Scripts

These scripts are for the AWS EC2 backend.

## Files

- `setup-ubuntu-ec2.sh`: run once on a fresh Ubuntu EC2 instance. It installs Go 1.24, creates the `clearday` user, and installs the `clearday.service` systemd unit.
- `update-backend.sh`: run during deploy. It builds the Go backend, updates `/opt/clearday/app/backend`, restarts `clearday.service`, and checks `/api/health`.

The production GitHub Actions workflow deploys through AWS Systems Manager:

```text
GitHub Actions -> S3 backend deploy bucket -> SSM Run Command -> EC2
```

That means GitHub does not need SSH access to EC2.

## Production Environment

Create this file on EC2:

```text
/etc/clearday.env
```

Use:

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

Use an EC2 IAM instance profile for DynamoDB access. Do not put AWS keys in `/etc/clearday.env`.

The EC2 instance profile should also include:

```text
AmazonSSMManagedInstanceCore
```

and read access to the backend deploy bucket.
