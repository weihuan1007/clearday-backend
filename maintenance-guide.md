# ClearDay Backend Maintenance Guide

This repository contains the backend half of ClearDay:

```text
backend/
deploy/aws/
deploy/scripts/
deploy/terraform/
```

The frontend is kept separately at:

```text
D:\reminder-app\repos\frontend
```

## Production Environment

Production config lives on EC2:

```text
/etc/clearday.env
```

Example:

```bash
CLEAR_DAY_ADDR=:8080
CLEAR_DAY_STORE=dynamodb
CLEAR_DAY_DYNAMODB_TABLE=clearday-reminders
CLEAR_DAY_API_TOKEN=your-secret-token
CLEAR_DAY_STATIC_DIR=/opt/clearday/app/web/static
AWS_REGION=ap-southeast-1
```

Use the EC2 IAM instance profile for DynamoDB access. Do not store AWS keys in this repository.

## Development Environment

Local development config lives here:

```text
D:\reminder-app\repos\backend\backend\.env
```

This file is ignored by Git. It should use the development table:

```bash
CLEAR_DAY_STORE=dynamodb
CLEAR_DAY_DYNAMODB_TABLE=clearday-reminders-dev
AWS_REGION=ap-southeast-1
```

Use `clearday-reminders-dev` only for dummy data. Production uses `clearday-reminders`.

## Manual Deploy

From PowerShell:

```powershell
cd D:\reminder-app\repos\backend
.\deploy\scripts\deploy-backend.ps1 -VmHost YOUR_EC2_PUBLIC_DNS -SshKey C:\path\to\key.pem
```

## Health Checks

On EC2:

```bash
curl http://127.0.0.1:8080/api/health
sudo systemctl status clearday --no-pager
sudo journalctl -u clearday -n 100 --no-pager
```

Through CloudFront:

```powershell
curl.exe https://YOUR_CLOUDFRONT_DOMAIN/api/health
```

Expected:

```json
{"status":"ok"}
```
