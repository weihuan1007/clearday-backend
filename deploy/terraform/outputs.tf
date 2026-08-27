output "dynamodb_table_name" {
  value = aws_dynamodb_table.reminders.name
}

output "development_dynamodb_table_name" {
  value = aws_dynamodb_table.reminders_dev.name
}

output "frontend_bucket_name" {
  value = aws_s3_bucket.frontend.bucket
}

output "backend_deploy_bucket_name" {
  value = aws_s3_bucket.backend_deploy.bucket
}

output "cloudfront_domain_name" {
  value = aws_cloudfront_distribution.app.domain_name
}

output "cloudfront_distribution_id" {
  value = aws_cloudfront_distribution.app.id
}

output "ec2_instance_profile_name" {
  value = aws_iam_instance_profile.ec2.name
}

output "ec2_security_group_id" {
  value = aws_security_group.ec2.id
}

output "frontend_api_base" {
  value = local.backend_enabled ? "/api" : "Set FRONTEND_API_BASE after the EC2 backend is ready."
}

output "github_backend_deploy_role_arn" {
  value = local.github_backend_role_enabled ? aws_iam_role.github_backend_deploy[0].arn : "Set github_backend_repository and backend_ec2_instance_id to create this role."
}
