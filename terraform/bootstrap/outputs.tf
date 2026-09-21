output "state_bucket_name" {
  description = "GitHub Actions変数 TF_STATE_BUCKET に設定する値"
  value       = aws_s3_bucket.terraform_state.bucket
}

output "github_actions_role_arn" {
  description = "GitHub Actions変数 AWS_DEV_TERRAFORM_ROLE_ARN に設定する値"
  value       = aws_iam_role.terraform_ci.arn
}

output "ecr_repository_url" {
  description = "ECRリポジトリのURL(参考情報。GitHub Actions変数への設定は不要)"
  value       = aws_ecr_repository.app.repository_url
}

output "ecr_repository_name" {
  description = "GitHub Actions変数 ECR_REPOSITORY に設定する値"
  value       = aws_ecr_repository.app.name
}

output "github_actions_ecr_push_role_arn" {
  description = "GitHub Actions変数 AWS_ECR_PUSH_ROLE_ARN に設定する値"
  value       = aws_iam_role.ecr_push.arn
}
