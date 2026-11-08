variable "money_table_name" {
  description = "DynamoDB table for the Money tab. Set this in the existing deployment state to enable it."
  type        = string
  default     = null
}

# Money uses the verified Cognito subject as its partition key.
resource "aws_dynamodb_table" "money" {
  count = var.money_table_name == null ? 0 : 1

  name                        = var.money_table_name
  billing_mode                = "PAY_PER_REQUEST"
  hash_key                    = "owner"
  range_key                   = "id"
  deletion_protection_enabled = true

  attribute {
    name = "owner"
    type = "S"
  }

  attribute {
    name = "id"
    type = "S"
  }

  tags = local.common_tags
}

resource "aws_iam_role_policy" "money" {
  count = length(aws_dynamodb_table.money)

  name = "clearday-money-access"
  role = aws_iam_role.ec2.name
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["dynamodb:Query", "dynamodb:PutItem", "dynamodb:DeleteItem"]
      Resource = aws_dynamodb_table.money[count.index].arn
    }]
  })
}

output "money_table_name" {
  value = one(aws_dynamodb_table.money[*].name)
}
