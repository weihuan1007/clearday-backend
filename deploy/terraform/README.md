# Terraform for ClearDay on AWS

This folder creates the AWS resources that are cheap and appropriate for the reminder app:

- DynamoDB production table for real reminders.
- DynamoDB development table for dummy reminders.
- S3 bucket for frontend files.
- CloudFront distribution for HTTPS frontend hosting.
- EC2 instance profile that lets the backend read/write the DynamoDB tables.
- EC2 security group for SSH and the Go backend port.

Terraform does not create the EC2 instance yet. For a beginner setup, create the EC2 instance in the AWS Console so you can easily choose a free-tier-eligible instance and key pair.

The included EC2 security group uses the default VPC. If your AWS account does not have a default VPC in the selected region, create the VPC manually first or extend this Terraform folder to manage a VPC.

## Free-Tier-Friendly Choices

- Each DynamoDB table uses provisioned capacity with `1` read capacity unit and `1` write capacity unit.
- S3 stores only static frontend files.
- CloudFront uses the default certificate and `PriceClass_100`.
- EC2 should be a free-tier-eligible Ubuntu instance.

## Required Variables

Create `terraform.tfvars` locally, but do not commit it:

```hcl
aws_region           = "ap-southeast-1"
frontend_bucket_name = "your-globally-unique-clearday-bucket-name"
```

After your EC2 instance exists, add:

```hcl
backend_origin_domain_name = "ec2-your-public-dns-name.ap-southeast-1.compute.amazonaws.com"
allowed_ssh_cidr          = "YOUR_PUBLIC_IP/32"
allowed_backend_cidr      = "0.0.0.0/0"
```

For a more locked-down setup, later replace `allowed_backend_cidr` with CloudFront origin-facing IP restrictions or put the backend behind a private origin pattern.

## Commands

```powershell
cd D:\reminder-app\repos\backend\deploy\terraform
terraform init
terraform fmt
terraform plan
terraform apply
```

## Important Outputs

After apply:

- `dynamodb_table_name`: production table. Set this as `CLEAR_DAY_DYNAMODB_TABLE` on EC2.
- `development_dynamodb_table_name`: development table. Use this in local `backend/.env` for dummy data.
- `ec2_instance_profile_name`: attach this IAM instance profile to your EC2 instance.
- `ec2_security_group_id`: attach this security group to your EC2 instance.
- `frontend_bucket_name`: set this as `AWS_FRONTEND_BUCKET` in GitHub variables.
- `cloudfront_distribution_id`: set this as `CLOUDFRONT_DISTRIBUTION_ID` in GitHub variables.
- `cloudfront_domain_name`: your public website URL.

## GitHub Actions

The GitHub Actions Terraform workflow expects:

```text
AWS_REGION
AWS_ROLE_TO_ASSUME
AWS_FRONTEND_BUCKET
APP_NAME
ALLOWED_SSH_CIDR
ALLOWED_BACKEND_CIDR
```

`AWS_ROLE_TO_ASSUME` should be an IAM role trusted by GitHub Actions OIDC.
