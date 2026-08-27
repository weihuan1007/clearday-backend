# ClearDay AWS Setup Guide

This guide starts from zero AWS resources for ClearDay.

Recommended beginner architecture:

```text
Browser
  |
  | HTTPS
  v
CloudFront
  |-- static frontend files -> S3
  |
  |-- /api/*                -> EC2 Ubuntu backend on :8080
                                  |
                                  v
                               DynamoDB
```

This keeps the app simple:

- Frontend deploys separately to S3.
- Backend deploys separately to EC2.
- Real reminder data goes to the production DynamoDB table.
- Dummy development data goes to a separate development DynamoDB table.
- No RDS database is needed.

## Why DynamoDB Instead Of RDS

For this reminder app, DynamoDB is cheaper and simpler than RDS:

- No database server to patch.
- No database password in `/etc/clearday.env`.
- Small personal reminder data fits DynamoDB well.
- AWS free-tier-style usage is generous for tiny apps.

## Step 1: Choose AWS Region

Use one region for everything.

Recommended near Malaysia:

```text
ap-southeast-1
```

That is Singapore. If you prefer another region, keep it consistent in every step.

## Step 2: Create Billing Protection First

Before creating resources:

1. Open AWS Console.
2. Go to `Billing and Cost Management`.
3. Go to `Budgets`.
4. Create a budget using a template.
5. Choose `Zero spend budget` or a very small monthly budget like USD 1 or USD 5.
6. Add your email.

This is the small smoke alarm for your wallet.

## Step 3: Create DynamoDB Table

You can use Terraform later, but for a beginner first run, the Console is easiest.

Go to:

```text
AWS Console -> DynamoDB -> Tables -> Create table
```

Create the production table first:

```text
Table name: clearday-reminders
Partition key: id
Partition key type: String
Table settings: Customize settings
Capacity mode: Provisioned
Read capacity units: 1
Write capacity units: 1
```

Do not enable global tables.

Do not enable streams.

Do not enable backups for the first dummy-data setup.

Create one more table for development dummy data:

```text
Table name: clearday-reminders-dev
Partition key: id
Partition key type: String
Table settings: Customize settings
Capacity mode: Provisioned
Read capacity units: 1
Write capacity units: 1
```

Use `clearday-reminders` only for production and `clearday-reminders-dev` only for local development tests.

## Step 4: Create S3 Bucket For Frontend

Go to:

```text
AWS Console -> S3 -> Create bucket
```

Use a globally unique name, for example:

```text
clearday-YOURNAME-frontend
```

Use:

```text
Region: ap-southeast-1
Block all public access: enabled
Bucket versioning: disabled
Default encryption: SSE-S3
```

Keep the bucket private. CloudFront will read it later.

## Step 5: Create CloudFront Distribution

Go to:

```text
AWS Console -> CloudFront -> Create distribution
```

Origin 1:

```text
Origin domain: your S3 bucket regional domain
Origin access: Origin access control settings
Viewer protocol policy: Redirect HTTP to HTTPS
Default root object: index.html
```

Create the distribution. Later, after EC2 is ready, add a second origin for the backend.

Save these values:

```text
CloudFront distribution ID
CloudFront domain name
```

The domain will look like:

```text
https://dxxxxxxxxxxxxx.cloudfront.net
```

## Step 6: Create IAM Role For EC2 Backend

Go to:

```text
AWS Console -> IAM -> Roles -> Create role
```

Use:

```text
Trusted entity type: AWS service
Use case: EC2
Role name: clearday-ec2-role
```

Create a policy for DynamoDB access.

Policy JSON:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:DeleteItem",
        "dynamodb:DescribeTable",
        "dynamodb:GetItem",
        "dynamodb:PutItem",
        "dynamodb:Scan",
        "dynamodb:UpdateItem"
      ],
      "Resource": [
        "arn:aws:dynamodb:ap-southeast-1:YOUR_ACCOUNT_ID:table/clearday-reminders",
        "arn:aws:dynamodb:ap-southeast-1:YOUR_ACCOUNT_ID:table/clearday-reminders-dev"
      ]
    }
  ]
}
```

Attach this policy to `clearday-ec2-role`.

## Step 7: Create EC2 Instance

Go to:

```text
AWS Console -> EC2 -> Instances -> Launch instance
```

Use:

```text
Name: clearday-backend
AMI: Ubuntu Server LTS, free-tier eligible
Instance type: choose a free-tier eligible instance
Key pair: create or select your SSH key
Network: default VPC
Auto-assign public IP: enabled
Storage: keep within free-tier amount
IAM instance profile: clearday-ec2-role
```

Security group inbound rules:

```text
SSH  TCP 22    Source: your public IP /32
App  TCP 8080  Source: 0.0.0.0/0 for first test
```

Later, restrict port `8080` after CloudFront works.

Save:

```text
EC2 public IPv4 address
EC2 public DNS name
```

## Step 8: Prepare Ubuntu EC2

SSH into EC2:

```bash
ssh -i /path/to/key.pem ubuntu@YOUR_EC2_PUBLIC_DNS
```

Upload the setup script from your PC:

```powershell
scp -i C:\path\to\key.pem D:\reminder-app\deploy\aws\setup-ubuntu-ec2.sh ubuntu@YOUR_EC2_PUBLIC_DNS:/home/ubuntu/
```

Run it:

```bash
sudo bash /home/ubuntu/setup-ubuntu-ec2.sh
```

## Step 9: Create Production Environment File

On EC2:

```bash
sudo tee /etc/clearday.env >/dev/null <<'EOF'
CLEAR_DAY_ADDR=:8080
CLEAR_DAY_STORE=dynamodb
CLEAR_DAY_DYNAMODB_TABLE=clearday-reminders
CLEAR_DAY_API_TOKEN=replace-with-a-long-random-token
CLEAR_DAY_STATIC_DIR=/opt/clearday/app/web/static
AWS_REGION=ap-southeast-1
EOF

sudo chmod 600 /etc/clearday.env
```

Generate a token:

```bash
openssl rand -hex 32
```

Put that value in `CLEAR_DAY_API_TOKEN`.

Do not put AWS access keys in this file. The EC2 role gives the app DynamoDB access.

Keep `/etc/clearday.env` pointed at `clearday-reminders`. Your local development `.env` should point at `clearday-reminders-dev`.

## Step 10: First Manual Backend Deploy

From your local PC:

```powershell
cd D:\reminder-app
.\deploy\scripts\deploy-backend.ps1 -VmHost YOUR_EC2_PUBLIC_DNS -SshKey C:\path\to\key.pem
```

Then on EC2:

```bash
sudo systemctl status clearday --no-pager
curl http://127.0.0.1:8080/api/health
```

Expected:

```json
{"status":"ok"}
```

## Step 11: Connect CloudFront To Backend

Open your CloudFront distribution.

Add origin:

```text
Origin domain: YOUR_EC2_PUBLIC_DNS
Protocol: HTTP only
HTTP port: 8080
```

Add behavior:

```text
Path pattern: /api/*
Origin: EC2 backend origin
Allowed HTTP methods: GET, HEAD, OPTIONS, PUT, POST, PATCH, DELETE
Cache policy: CachingDisabled
Origin request policy: AllViewerExceptHostHeader, or forward Authorization and CORS headers
Viewer protocol policy: Redirect HTTP to HTTPS
```

Wait until CloudFront status is `Deployed`.

Test:

```powershell
curl.exe https://YOUR_CLOUDFRONT_DOMAIN/api/health
```

## Step 12: Configure GitHub Actions

Add repository variables:

```text
APP_NAME=clearday
AWS_REGION=ap-southeast-1
AWS_ROLE_TO_ASSUME=arn:aws:iam::YOUR_ACCOUNT_ID:role/clearday-github-actions
AWS_FRONTEND_BUCKET=your-bucket-name
CLOUDFRONT_DISTRIBUTION_ID=your-cloudfront-distribution-id
FRONTEND_API_BASE=/api
AWS_BACKEND_ROLE_TO_ASSUME=arn:aws:iam::YOUR_ACCOUNT_ID:role/clearday-backend-github-actions
AWS_BACKEND_DEPLOY_BUCKET=your-backend-deploy-bucket-name
AWS_BACKEND_INSTANCE_ID=your-ec2-instance-id
```

The backend workflow uses AWS Systems Manager, so you do not need to add your EC2 private key to GitHub.

```text
D:\reminder-app\repos\backend\ssm-github-actions-deploy-guide.md
```

## Step 13: Create GitHub OIDC Role

In AWS IAM:

1. Create an OpenID Connect provider:

```text
Provider URL: https://token.actions.githubusercontent.com
Audience: sts.amazonaws.com
```

2. Create role:

```text
Role name: clearday-github-actions
Trusted entity: Web identity
Identity provider: token.actions.githubusercontent.com
Audience: sts.amazonaws.com
```

3. Limit the trust policy to your repo and branch:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::YOUR_ACCOUNT_ID:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
          "token.actions.githubusercontent.com:sub": "repo:YOUR_GITHUB_USERNAME/YOUR_REPO:ref:refs/heads/main"
        }
      }
    }
  ]
}
```

4. Attach permissions for frontend deploy:

Replace bucket, account, and distribution values, then attach this policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:ListBucket"
      ],
      "Resource": "arn:aws:s3:::YOUR_FRONTEND_BUCKET"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:DeleteObject",
        "s3:GetObject",
        "s3:PutObject"
      ],
      "Resource": "arn:aws:s3:::YOUR_FRONTEND_BUCKET/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "cloudfront:CreateInvalidation"
      ],
      "Resource": "arn:aws:cloudfront::YOUR_ACCOUNT_ID:distribution/YOUR_DISTRIBUTION_ID"
    }
  ]
}
```

If you want GitHub Actions to run Terraform apply, the role also needs permission to manage the resources in `deploy/terraform`.

For first-time learning, you can temporarily use broader permissions for Terraform, then reduce them after the infrastructure is created. Do not leave administrator-style permissions attached forever.

## Step 14: Use Terraform When Ready

Terraform files are in:

```text
D:\reminder-app\repos\backend\deploy\terraform
```

They can create:

- DynamoDB production and development tables
- S3 bucket
- CloudFront distribution
- EC2 instance profile
- EC2 security group

For the first AWS setup, manual Console setup is easier. Once it works, Terraform can become the source of truth.

## Normal Update Flow

Frontend:

```powershell
cd D:\reminder-app\repos\frontend
git add .
git commit -m "Update frontend"
git push
```

Backend:

```powershell
cd D:\reminder-app\repos\backend
git add backend deploy
git commit -m "Update backend"
git push
```

## Cost Checklist

To stay close to free tier:

- Use a free-tier-eligible EC2 instance.
- Keep EC2 storage small.
- Use DynamoDB provisioned capacity `1` read and `1` write.
- Store only frontend files in S3.
- Create an AWS Budget before testing.
- Avoid NAT Gateway, Load Balancer, RDS, Elastic IP left unattached, and paid Route 53 hosted zones unless you intentionally need them.
