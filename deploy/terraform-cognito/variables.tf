variable "aws_region" {
  description = "AWS region that hosts ClearDay."
  type        = string
  default     = "ap-southeast-1"
}

variable "app_name" {
  description = "Application name used for Cognito resources."
  type        = string
  default     = "clearday"
}

variable "application_url" {
  description = "Exact CloudFront website URL, including https:// and a trailing slash."
  type        = string

  validation {
    condition     = startswith(var.application_url, "https://") && endswith(var.application_url, "/")
    error_message = "application_url must use https:// and end with /."
  }
}

variable "cognito_domain_prefix" {
  description = "Globally unique Cognito domain prefix. Leave empty to derive a safe default."
  type        = string
  default     = ""
}

variable "local_application_url" {
  description = "Optional local callback URL. Leave empty until local Cognito testing is needed."
  type        = string
  default     = ""
}
