terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  # Recommended later:
  # Store Terraform state in a private S3 bucket with DynamoDB state locking.
  # Configure remote state only after the AWS bootstrap is working.
}
