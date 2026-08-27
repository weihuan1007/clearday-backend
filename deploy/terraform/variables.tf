variable "aws_region" {
  description = "AWS region for the app, for example ap-southeast-1."
  type        = string
  default     = "ap-southeast-1"
}

variable "app_name" {
  description = "Short application name used in AWS resource names."
  type        = string
  default     = "clearday"
}

variable "frontend_bucket_name" {
  description = "Globally unique S3 bucket name for frontend files."
  type        = string
}

variable "backend_deploy_bucket_name" {
  description = "Globally unique private S3 bucket name for backend release packages. Leave empty to use frontend_bucket_name plus -backend-deploy."
  type        = string
  default     = ""
}

variable "dynamodb_table_name" {
  description = "Production DynamoDB table name for reminders."
  type        = string
  default     = "clearday-reminders"
}

variable "development_dynamodb_table_name" {
  description = "Development DynamoDB table name for dummy reminders."
  type        = string
  default     = "clearday-reminders-dev"
}

variable "backend_origin_domain_name" {
  description = "Optional EC2 public DNS name for CloudFront /api/* routing, without http:// and without port."
  type        = string
  default     = ""
}

variable "backend_origin_http_port" {
  description = "HTTP port where the Go backend listens on EC2."
  type        = number
  default     = 8080
}

variable "allowed_ssh_cidr" {
  description = "CIDR allowed to SSH to EC2. Use your public IP with /32."
  type        = string
  default     = "0.0.0.0/0"
}

variable "allowed_backend_cidr" {
  description = "CIDR allowed to call the backend directly. Use 0.0.0.0/0 for simple testing, then restrict later."
  type        = string
  default     = "0.0.0.0/0"
}

variable "github_repository" {
  description = "GitHub repository in owner/name format. Used only for documentation outputs."
  type        = string
  default     = ""
}

variable "github_backend_repository" {
  description = "Backend GitHub repository in owner/name format, for example weihuan1007/clearday-backend. When set with backend_ec2_instance_id, Terraform creates the backend GitHub Actions deploy role."
  type        = string
  default     = ""
}

variable "github_branch" {
  description = "GitHub branch that deploys production."
  type        = string
  default     = "main"
}

variable "github_oidc_provider_arn" {
  description = "Existing GitHub Actions OIDC provider ARN. Leave empty to use arn:aws:iam::<account-id>:oidc-provider/token.actions.githubusercontent.com."
  type        = string
  default     = ""
}

variable "github_backend_deploy_role_name" {
  description = "IAM role name used by the backend GitHub Actions SSM deployment workflow."
  type        = string
  default     = "clearday-backend-github-actions"
}

variable "backend_ec2_instance_id" {
  description = "Existing EC2 instance id for backend SSM deployment, for example i-0123456789abcdef0. When set with github_backend_repository, Terraform creates the backend GitHub Actions deploy role."
  type        = string
  default     = ""
}
