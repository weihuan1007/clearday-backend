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
CLEAR_DAY_API_TOKEN=replace-with-a-long-random-token
CLEAR_DAY_STATIC_DIR=/opt/clearday/app/web/static
AWS_REGION=ap-southeast-1
```

Use an EC2 IAM instance profile for DynamoDB access. Do not put AWS keys in `/etc/clearday.env`.

The EC2 instance profile should also include:

```text
AmazonSSMManagedInstanceCore
```

and read access to the backend deploy bucket.
