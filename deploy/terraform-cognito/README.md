# Isolated Cognito Terraform

This folder manages only ClearDay login resources. It intentionally has separate Terraform state so applying it cannot change the existing EC2, S3, CloudFront, DynamoDB, IAM, or networking setup.

Follow `D:\reminder-app\repos\backend\cognito-login-guide.md` before applying.
