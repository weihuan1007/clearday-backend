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

variable "github_branch" {
  description = "GitHub branch that deploys production."
  type        = string
  default     = "main"
}
