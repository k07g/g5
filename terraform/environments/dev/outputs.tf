output "aws_region" {
  value = var.aws_region
}

output "api_url" {
  description = "デプロイされたAPIのURL(ALBのAWS提供ドメイン、HTTP)"
  value       = "http://${aws_lb.app.dns_name}"
}

output "database_url_secret_arn" {
  description = "DATABASE_URLの値を手動で投入するSecrets ManagerシークレットのARN(terraform/README.md参照)"
  value       = aws_secretsmanager_secret.database_url.arn
}

output "ecs_cluster_name" {
  value = aws_ecs_cluster.this.name
}

output "ecs_service_name" {
  value = aws_ecs_service.app.name
}

output "ecs_task_definition_family" {
  value = aws_ecs_task_definition.app.family
}
