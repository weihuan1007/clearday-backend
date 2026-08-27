locals {
  common_tags = {
    App       = var.app_name
    ManagedBy = "terraform"
  }

  backend_enabled = trimspace(var.backend_origin_domain_name) != ""

  backend_deploy_bucket_name      = trimspace(var.backend_deploy_bucket_name) != "" ? var.backend_deploy_bucket_name : "${var.frontend_bucket_name}-backend-deploy"
  github_backend_role_enabled     = trimspace(var.github_backend_repository) != "" && trimspace(var.backend_ec2_instance_id) != ""
  github_oidc_provider_arn        = trimspace(var.github_oidc_provider_arn) != "" ? var.github_oidc_provider_arn : "arn:aws:iam::${data.aws_caller_identity.current.account_id}:oidc-provider/token.actions.githubusercontent.com"
  github_backend_repository_older = "repo:${var.github_backend_repository}:environment:production"
  github_backend_repository_newer = "repo:${replace(var.github_backend_repository, "/", "@*/")}@*:environment:production"
}

data "aws_caller_identity" "current" {}

data "aws_vpc" "default" {
  default = true
}

resource "aws_dynamodb_table" "reminders" {
  name         = var.dynamodb_table_name
  billing_mode = "PROVISIONED"
  hash_key     = "id"

  read_capacity  = 1
  write_capacity = 1

  attribute {
    name = "id"
    type = "S"
  }

  point_in_time_recovery {
    enabled = false
  }

  tags = merge(local.common_tags, {
    Environment = "production"
  })
}

resource "aws_dynamodb_table" "reminders_dev" {
  name         = var.development_dynamodb_table_name
  billing_mode = "PROVISIONED"
  hash_key     = "id"

  read_capacity  = 1
  write_capacity = 1

  attribute {
    name = "id"
    type = "S"
  }

  point_in_time_recovery {
    enabled = false
  }

  tags = merge(local.common_tags, {
    Environment = "development"
  })
}

resource "aws_s3_bucket" "frontend" {
  bucket = var.frontend_bucket_name
  tags   = local.common_tags
}

resource "aws_s3_bucket_ownership_controls" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_public_access_block" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket" "backend_deploy" {
  bucket = local.backend_deploy_bucket_name

  tags = merge(local.common_tags, {
    Purpose = "backend-deploy-artifacts"
  })
}

resource "aws_s3_bucket_ownership_controls" "backend_deploy" {
  bucket = aws_s3_bucket.backend_deploy.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_public_access_block" "backend_deploy" {
  bucket = aws_s3_bucket.backend_deploy.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "backend_deploy" {
  bucket = aws_s3_bucket.backend_deploy.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "backend_deploy" {
  bucket = aws_s3_bucket.backend_deploy.id

  rule {
    id     = "expire-old-backend-releases"
    status = "Enabled"

    filter {
      prefix = ""
    }

    expiration {
      days = 7
    }
  }
}

resource "aws_cloudfront_origin_access_control" "frontend" {
  name                              = "${var.app_name}-frontend-oac"
  description                       = "Allow CloudFront to read ClearDay frontend files from S3."
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_distribution" "app" {
  enabled             = true
  is_ipv6_enabled     = true
  comment             = "${var.app_name} frontend"
  default_root_object = "index.html"
  price_class         = "PriceClass_100"

  origin {
    domain_name              = aws_s3_bucket.frontend.bucket_regional_domain_name
    origin_id                = "frontend-s3"
    origin_access_control_id = aws_cloudfront_origin_access_control.frontend.id
  }

  dynamic "origin" {
    for_each = local.backend_enabled ? [1] : []

    content {
      domain_name = var.backend_origin_domain_name
      origin_id   = "backend-ec2"

      custom_origin_config {
        http_port              = var.backend_origin_http_port
        https_port             = 443
        origin_protocol_policy = "http-only"
        origin_ssl_protocols   = ["TLSv1.2"]
      }
    }
  }

  default_cache_behavior {
    target_origin_id       = "frontend-s3"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    compress               = true
    min_ttl                = 0
    default_ttl            = 3600
    max_ttl                = 86400

    forwarded_values {
      query_string = false

      cookies {
        forward = "none"
      }
    }
  }

  dynamic "ordered_cache_behavior" {
    for_each = local.backend_enabled ? [1] : []

    content {
      path_pattern           = "/api/*"
      target_origin_id       = "backend-ec2"
      viewer_protocol_policy = "redirect-to-https"
      allowed_methods        = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
      cached_methods         = ["GET", "HEAD", "OPTIONS"]
      compress               = false
      min_ttl                = 0
      default_ttl            = 0
      max_ttl                = 0

      forwarded_values {
        query_string = true
        headers      = ["Authorization", "Content-Type", "Origin", "Access-Control-Request-Headers", "Access-Control-Request-Method"]

        cookies {
          forward = "none"
        }
      }
    }
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }

  tags = local.common_tags
}

data "aws_iam_policy_document" "frontend_bucket" {
  statement {
    sid     = "AllowCloudFrontRead"
    actions = ["s3:GetObject"]

    resources = [
      "${aws_s3_bucket.frontend.arn}/*",
    ]

    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.app.arn]
    }
  }
}

resource "aws_s3_bucket_policy" "frontend" {
  bucket = aws_s3_bucket.frontend.id
  policy = data.aws_iam_policy_document.frontend_bucket.json
}

data "aws_iam_policy_document" "ec2_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ec2" {
  name               = "${var.app_name}-ec2-role"
  assume_role_policy = data.aws_iam_policy_document.ec2_assume_role.json
  tags               = local.common_tags
}

data "aws_iam_policy_document" "dynamodb_access" {
  statement {
    actions = [
      "dynamodb:DeleteItem",
      "dynamodb:DescribeTable",
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:Scan",
      "dynamodb:UpdateItem",
    ]

    resources = [
      aws_dynamodb_table.reminders.arn,
      aws_dynamodb_table.reminders_dev.arn,
    ]
  }
}

resource "aws_iam_policy" "dynamodb_access" {
  name   = "${var.app_name}-dynamodb-access"
  policy = data.aws_iam_policy_document.dynamodb_access.json
  tags   = local.common_tags
}

resource "aws_iam_role_policy_attachment" "ec2_dynamodb_access" {
  role       = aws_iam_role.ec2.name
  policy_arn = aws_iam_policy.dynamodb_access.arn
}

resource "aws_iam_role_policy_attachment" "ec2_ssm_core" {
  role       = aws_iam_role.ec2.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

data "aws_iam_policy_document" "backend_deploy_bucket_read" {
  statement {
    sid       = "ListBackendDeployBucket"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.backend_deploy.arn]
  }

  statement {
    sid       = "ReadBackendDeployPackages"
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.backend_deploy.arn}/*"]
  }
}

resource "aws_iam_policy" "backend_deploy_bucket_read" {
  name   = "${var.app_name}-backend-deploy-bucket-read"
  policy = data.aws_iam_policy_document.backend_deploy_bucket_read.json
  tags   = local.common_tags
}

resource "aws_iam_role_policy_attachment" "ec2_backend_deploy_bucket_read" {
  role       = aws_iam_role.ec2.name
  policy_arn = aws_iam_policy.backend_deploy_bucket_read.arn
}

resource "aws_iam_instance_profile" "ec2" {
  name = "${var.app_name}-ec2-profile"
  role = aws_iam_role.ec2.name
  tags = local.common_tags
}

data "aws_iam_policy_document" "github_backend_assume_role" {
  count = local.github_backend_role_enabled ? 1 : 0

  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [local.github_oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values = [
        local.github_backend_repository_older,
        local.github_backend_repository_newer,
      ]
    }
  }
}

resource "aws_iam_role" "github_backend_deploy" {
  count = local.github_backend_role_enabled ? 1 : 0

  name               = var.github_backend_deploy_role_name
  assume_role_policy = data.aws_iam_policy_document.github_backend_assume_role[0].json
  tags               = local.common_tags
}

data "aws_iam_policy_document" "github_backend_deploy" {
  count = local.github_backend_role_enabled ? 1 : 0

  statement {
    sid       = "ListBackendDeployBucket"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.backend_deploy.arn]
  }

  statement {
    sid = "WriteBackendDeployPackages"
    actions = [
      "s3:DeleteObject",
      "s3:PutObject",
    ]
    resources = ["${aws_s3_bucket.backend_deploy.arn}/*"]
  }

  statement {
    sid = "SendBackendDeployCommand"
    actions = [
      "ssm:SendCommand",
    ]
    resources = [
      "arn:aws:ec2:${var.aws_region}:${data.aws_caller_identity.current.account_id}:instance/${var.backend_ec2_instance_id}",
      "arn:aws:ssm:${var.aws_region}::document/AWS-RunShellScript",
    ]
  }

  statement {
    sid = "ReadBackendDeployCommandStatus"
    actions = [
      "ssm:DescribeInstanceInformation",
      "ssm:GetCommandInvocation",
      "ssm:ListCommandInvocations",
      "ssm:ListCommands",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_policy" "github_backend_deploy" {
  count = local.github_backend_role_enabled ? 1 : 0

  name   = "${var.app_name}-github-backend-deploy"
  policy = data.aws_iam_policy_document.github_backend_deploy[0].json
  tags   = local.common_tags
}

resource "aws_iam_role_policy_attachment" "github_backend_deploy" {
  count = local.github_backend_role_enabled ? 1 : 0

  role       = aws_iam_role.github_backend_deploy[0].name
  policy_arn = aws_iam_policy.github_backend_deploy[0].arn
}

resource "aws_security_group" "ec2" {
  name        = "${var.app_name}-ec2-sg"
  description = "ClearDay EC2 inbound rules"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.allowed_ssh_cidr]
  }

  ingress {
    description = "Go backend"
    from_port   = var.backend_origin_http_port
    to_port     = var.backend_origin_http_port
    protocol    = "tcp"
    cidr_blocks = [var.allowed_backend_cidr]
  }

  egress {
    description = "Outbound internet"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = local.common_tags
}
