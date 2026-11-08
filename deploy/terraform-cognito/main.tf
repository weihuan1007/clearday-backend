locals {
  domain_prefix = trimspace(var.cognito_domain_prefix) != "" ? var.cognito_domain_prefix : "${var.app_name}-${data.aws_caller_identity.current.account_id}-${var.aws_region}"
  callback_urls = compact([var.application_url, var.local_application_url])

  common_tags = {
    App       = var.app_name
    ManagedBy = "terraform"
    Purpose   = "personal-login"
  }
}

resource "aws_cognito_user_pool" "app" {
  name                = "${var.app_name}-users"
  user_pool_tier      = "ESSENTIALS"
  deletion_protection = "ACTIVE"

  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]
  mfa_configuration        = "ON"

  admin_create_user_config {
    allow_admin_create_user_only = true
  }

  password_policy {
    minimum_length                   = 12
    require_lowercase                = true
    require_numbers                  = true
    require_symbols                  = true
    require_uppercase                = true
    temporary_password_validity_days = 7
  }

  software_token_mfa_configuration {
    enabled = true
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }

  user_attribute_update_settings {
    attributes_require_verification_before_update = ["email"]
  }

  tags = local.common_tags
}

resource "aws_cognito_user_pool_client" "web" {
  name         = "${var.app_name}-web"
  user_pool_id = aws_cognito_user_pool.app.id

  generate_secret                      = false
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["email", "openid"]
  supported_identity_providers         = ["COGNITO"]
  callback_urls                        = local.callback_urls
  logout_urls                          = local.callback_urls
  prevent_user_existence_errors        = "ENABLED"
  enable_token_revocation              = true
  explicit_auth_flows                  = ["ALLOW_USER_SRP_AUTH"]
  auth_session_validity                = 3
  access_token_validity                = 60
  id_token_validity                    = 60
  refresh_token_validity               = 30

  token_validity_units {
    access_token  = "minutes"
    id_token      = "minutes"
    refresh_token = "days"
  }

  refresh_token_rotation {
    feature                    = "ENABLED"
    retry_grace_period_seconds = 10
  }
}

resource "aws_cognito_user_pool_domain" "app" {
  domain                = local.domain_prefix
  user_pool_id          = aws_cognito_user_pool.app.id
  managed_login_version = 2
}

resource "aws_cognito_managed_login_branding" "app" {
  client_id                   = aws_cognito_user_pool_client.web.id
  user_pool_id                = aws_cognito_user_pool.app.id
  use_cognito_provided_values = true

  depends_on = [aws_cognito_user_pool_domain.app]
}
