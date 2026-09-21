data "aws_caller_identity" "current" {}

locals {
  ecr_repository_url = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.aws_region}.amazonaws.com/${var.project_name}"
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/${var.project_name}-dev"
  retention_in_days = var.log_retention_days

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_ecs_cluster" "this" {
  name = "${var.project_name}-dev"

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

# --- 実行ロール: ECRからのpull、CloudWatch Logsへの書き込み、
#     Secrets Managerからのシークレット取得(コンテナ起動時)に使う ---

data "aws_iam_policy_document" "ecs_task_execution_trust" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ecs_task_execution" {
  name               = "${var.project_name}-dev-ecs-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_execution_trust.json

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_iam_role_policy_attachment" "ecs_task_execution_managed" {
  role       = aws_iam_role.ecs_task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "ecs_task_execution_secrets" {
  statement {
    effect = "Allow"
    actions = [
      "secretsmanager:GetSecretValue",
    ]
    resources = [
      aws_secretsmanager_secret.mongodb_uri.arn,
    ]
  }
}

resource "aws_iam_role_policy" "ecs_task_execution_secrets" {
  name   = "${var.project_name}-dev-ecs-execution-secrets"
  role   = aws_iam_role.ecs_task_execution.id
  policy = data.aws_iam_policy_document.ecs_task_execution_secrets.json
}

# --- タスクロール: アプリ自身(Goプロセス)がAWS SDK経由でCognitoの
#     GetUserを呼ぶために使う(内蔵のECSコンテナ認証情報プロバイダ経由)。
#     g5はCognitoユーザープール自体を持たず、github.com/k07g/g4が所有する
#     プールに対してアクセストークンを検証するだけなので、必要な権限は
#     GetUserのみでよい(サインアップ/サインイン等はg4側の責務)。 ---

data "aws_iam_policy_document" "ecs_task_trust" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ecs_task" {
  name               = "${var.project_name}-dev-ecs-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_task_trust.json

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

data "aws_iam_policy_document" "ecs_task_cognito" {
  statement {
    effect    = "Allow"
    actions   = ["cognito-idp:GetUser"]
    resources = [var.cognito_user_pool_arn]
  }
}

resource "aws_iam_role_policy" "ecs_task_cognito" {
  name   = "${var.project_name}-dev-ecs-task-cognito"
  role   = aws_iam_role.ecs_task.id
  policy = data.aws_iam_policy_document.ecs_task_cognito.json
}

# --- タスク定義・サービス ---
# image は初回applyの時点ではまだECRに存在しない(bootstrapでリポジトリを
# 作っただけ)。docker-publish/deployワークフローが新しいイメージをpushする
# たびに新しいリビジョンを登録してサービスを更新するため、Terraformが
# 管理するのはこの初期リビジョンのみで良い。ecs_serviceのlifecycleで
# task_definitionへの変更を無視し、CIによる更新をTerraform applyが
# 巻き戻さないようにする。

resource "aws_ecs_task_definition" "app" {
  family                   = "${var.project_name}-dev"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = tostring(var.container_cpu)
  memory                   = tostring(var.container_memory)
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([
    {
      name  = var.project_name
      image = "${local.ecr_repository_url}:latest"

      portMappings = [
        {
          containerPort = var.container_port
          protocol      = "tcp"
        }
      ]

      environment = [
        { name = "PORT", value = tostring(var.container_port) },
        { name = "AUTH_PROVIDER", value = "cognito" },
        { name = "AWS_REGION", value = var.aws_region },
        { name = "MONGODB_DATABASE", value = var.mongodb_database },
      ]

      secrets = [
        { name = "MONGODB_URI", valueFrom = aws_secretsmanager_secret.mongodb_uri.arn },
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.app.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "ecs"
        }
      }
    }
  ])

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_ecs_service" "app" {
  name            = "${var.project_name}-dev"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.app.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.public[*].id
    security_groups  = [aws_security_group.ecs_service.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.app.arn
    container_name   = var.project_name
    container_port   = var.container_port
  }

  # デプロイ(新しいイメージのpush)はCIがtask_definitionの新しいリビジョンを
  # 登録してサービスを更新する形で行うため、Terraformはそれを上書きしない。
  lifecycle {
    ignore_changes = [task_definition, desired_count]
  }

  depends_on = [aws_lb_listener.http]

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}
