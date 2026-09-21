data "aws_caller_identity" "current" {}

# --- Terraform state用バックエンド(S3。ロックはS3ネイティブロックを
#     使うため別途DynamoDBテーブルは不要) ---

resource "aws_s3_bucket" "terraform_state" {
  bucket = var.state_bucket_name

  # 誤ってstateバケットを削除してしまうと各環境のstateを失うため保護する
  lifecycle {
    prevent_destroy = true
  }

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
    Purpose   = "terraform-state"
  }
}

resource "aws_s3_bucket_versioning" "terraform_state" {
  bucket = aws_s3_bucket.terraform_state.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "terraform_state" {
  bucket = aws_s3_bucket.terraform_state.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "terraform_state" {
  bucket = aws_s3_bucket.terraform_state.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# --- GitHub Actions用 OIDC IAMロール ---
# CIから長期クレデンシャルを使わずAWSを操作できるようにする。
# ワークフロー側のjobsで`environment: dev`を指定しているため、GitHubが
# 発行するOIDCトークンのsubクレームは repo:OWNER/REPO:ref:refs/heads/BRANCH
# ではなく repo:OWNER/REPO:environment:ENV_NAME になる。そのため信頼関係も
# ブランチではなく github_actions_environment (既定: dev) に対して設定する。

data "tls_certificate" "github_actions" {
  count = var.create_github_oidc_provider ? 1 : 0
  url   = "https://token.actions.githubusercontent.com"
}

resource "aws_iam_openid_connect_provider" "github_actions" {
  count = var.create_github_oidc_provider ? 1 : 0

  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.github_actions[0].certificates[0].sha1_fingerprint]
}

locals {
  github_oidc_provider_arn = var.create_github_oidc_provider ? aws_iam_openid_connect_provider.github_actions[0].arn : var.existing_github_oidc_provider_arn
}

data "aws_iam_policy_document" "github_actions_trust" {
  statement {
    effect  = "Allow"
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
      values   = ["repo:${var.github_repository}:environment:${var.github_actions_environment}"]
    }
  }
}

resource "aws_iam_role" "terraform_ci" {
  name               = "${var.project_name}-dev-terraform-ci"
  assume_role_policy = data.aws_iam_policy_document.github_actions_trust.json

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

data "aws_iam_policy_document" "terraform_ci_permissions" {
  statement {
    sid    = "TerraformStateObjects"
    effect = "Allow"
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
    ]
    resources = ["${aws_s3_bucket.terraform_state.arn}/*"]
  }

  statement {
    sid       = "TerraformStateBucketList"
    effect    = "Allow"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.terraform_state.arn]
  }

  # stateのロックはS3ネイティブロック(use_lockfile)を使う。ロックファイルも
  # 同じバケット内のオブジェクトなので、上のTerraformStateObjects/
  # TerraformStateBucketListの権限だけで足り、追加の権限は不要。
}

resource "aws_iam_role_policy" "terraform_ci" {
  name   = "${var.project_name}-dev-terraform-ci"
  role   = aws_iam_role.terraform_ci.id
  policy = data.aws_iam_policy_document.terraform_ci_permissions.json
}

# dev環境にVPC/ALB/DocumentDB/ECSを構築するための追加権限。このロールは
# mainブランチのCIワークフローからのみAssumeRoleWithWebIdentityできるため
# 実行経路はCIに限定されるが、IAMロールの作成・PassRoleだけは権限昇格を
# 防ぐため dev 用の命名規則に一致するロールに限定する。
#
# 認証情報(サインアップ/サインイン)はgithub.com/k07g/g4が持つCognito
# ユーザープールを使い、このリポジトリではCognitoリソースを一切作成・
# 変更しないため、cognito-idp系の権限はここには含めない。
data "aws_iam_policy_document" "terraform_ci_dev_infra_permissions" {
  statement {
    sid       = "EC2Networking"
    effect    = "Allow"
    actions   = ["ec2:*"]
    resources = ["*"]
  }

  statement {
    # Amazon DocumentDBのAPI操作はRDSと同じコントロールプレーンを使い、
    # IAMアクションも独立した"docdb:"名前空間ではなく"rds:"名前空間で
    # 公開されている(専用のdocdb:名前空間は存在しない)。
    sid       = "DocumentDBManagement"
    effect    = "Allow"
    actions   = ["rds:*"]
    resources = ["*"]
  }

  statement {
    sid       = "ECSManagement"
    effect    = "Allow"
    actions   = ["ecs:*"]
    resources = ["*"]
  }

  statement {
    sid       = "ELBManagement"
    effect    = "Allow"
    actions   = ["elasticloadbalancing:*"]
    resources = ["*"]
  }

  statement {
    # DescribeLogGroupsはリスト系操作でありリソースレベル権限をサポート
    # しない(常にresource "*"が必要)ため、他のlogs操作とは別ステート
    # メントにする。
    sid       = "LogsDescribe"
    effect    = "Allow"
    actions   = ["logs:DescribeLogGroups"]
    resources = ["*"]
  }

  statement {
    sid    = "LogsManagement"
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:DeleteLogGroup",
      "logs:PutRetentionPolicy",
      "logs:TagResource",
      "logs:UntagResource",
      "logs:ListTagsForResource",
    ]
    resources = ["arn:aws:logs:*:${data.aws_caller_identity.current.account_id}:log-group:/ecs/${var.project_name}-dev*"]
  }

  statement {
    # DocumentDB/ECS/ELBをこのアカウントで初めて使う場合、各サービスの
    # service-linked roleが自動作成される。作成者にiam:CreateServiceLinkedRole
    # が必要なため、対象サービスに限定して許可する。
    sid     = "CreateAwsServiceLinkedRoles"
    effect  = "Allow"
    actions = ["iam:CreateServiceLinkedRole"]
    resources = [
      "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/aws-service-role/rds.amazonaws.com/AWSServiceRoleForRDS",
      "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/aws-service-role/ecs.amazonaws.com/AWSServiceRoleForECS",
      "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/aws-service-role/elasticloadbalancing.amazonaws.com/AWSServiceRoleForElasticLoadBalancing",
    ]

    condition {
      test     = "StringLike"
      variable = "iam:AWSServiceName"
      values = [
        "rds.amazonaws.com",
        "ecs.amazonaws.com",
        "elasticloadbalancing.amazonaws.com",
      ]
    }
  }

  statement {
    sid    = "SecretsManagerManagement"
    effect = "Allow"
    actions = [
      "secretsmanager:CreateSecret",
      "secretsmanager:DeleteSecret",
      "secretsmanager:DescribeSecret",
      "secretsmanager:GetSecretValue",
      "secretsmanager:PutSecretValue",
      "secretsmanager:UpdateSecret",
      "secretsmanager:TagResource",
      "secretsmanager:UntagResource",
      "secretsmanager:GetResourcePolicy",
    ]
    resources = ["arn:aws:secretsmanager:*:${data.aws_caller_identity.current.account_id}:secret:${var.project_name}/dev/*"]
  }

  statement {
    sid    = "IAMRoleManagementScoped"
    effect = "Allow"
    actions = [
      "iam:CreateRole",
      "iam:DeleteRole",
      "iam:GetRole",
      "iam:UpdateRole",
      "iam:UpdateAssumeRolePolicy",
      "iam:PutRolePolicy",
      "iam:DeleteRolePolicy",
      "iam:GetRolePolicy",
      "iam:ListRolePolicies",
      "iam:AttachRolePolicy",
      "iam:DetachRolePolicy",
      "iam:ListAttachedRolePolicies",
      "iam:TagRole",
      "iam:UntagRole",
      "iam:ListInstanceProfilesForRole",
      "iam:PassRole",
    ]
    resources = ["arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/${var.project_name}-dev-*"]
  }
}

resource "aws_iam_role_policy" "terraform_ci_dev_infra" {
  name   = "${var.project_name}-dev-terraform-ci-infra"
  role   = aws_iam_role.terraform_ci.id
  policy = data.aws_iam_policy_document.terraform_ci_dev_infra_permissions.json
}

# --- アプリのDockerイメージ用ECRリポジトリ ---

resource "aws_ecr_repository" "app" {
  name                 = var.ecr_repository_name
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_ecr_lifecycle_policy" "app" {
  repository = aws_ecr_repository.app.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "直近20件のイメージのみ保持する"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = 20
        }
        action = { type = "expire" }
      }
    ]
  })
}

# --- GitHub Actions用 ECR pushロール ---
# main へのpush(マージ)を直接トリガーに使うワークフロー向けなので、
# terraform_ci ロールとは分け、ブランチ(ref)ベースで信頼関係を設定する。

data "aws_iam_policy_document" "github_actions_docker_trust" {
  statement {
    effect  = "Allow"
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
      values   = ["repo:${var.github_repository}:ref:refs/heads/${var.github_actions_docker_branch}"]
    }
  }
}

resource "aws_iam_role" "ecr_push" {
  name               = "${var.project_name}-ecr-push"
  assume_role_policy = data.aws_iam_policy_document.github_actions_docker_trust.json

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

data "aws_iam_policy_document" "ecr_push_permissions" {
  statement {
    sid       = "ECRAuth"
    effect    = "Allow"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }

  statement {
    sid    = "ECRPush"
    effect = "Allow"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:GetDownloadUrlForLayer",
      "ecr:BatchGetImage",
      "ecr:PutImage",
      "ecr:InitiateLayerUpload",
      "ecr:UploadLayerPart",
      "ecr:CompleteLayerUpload",
    ]
    resources = [aws_ecr_repository.app.arn]
  }
}

resource "aws_iam_role_policy" "ecr_push" {
  name   = "${var.project_name}-ecr-push"
  role   = aws_iam_role.ecr_push.id
  policy = data.aws_iam_policy_document.ecr_push_permissions.json
}

# 同じロールを使って、pushしたイメージをdev環境のECSサービスにデプロイ
# する(新しいタスク定義リビジョンの登録とサービス更新)。
data "aws_iam_policy_document" "ecr_push_ecs_deploy_permissions" {
  statement {
    sid    = "ECSDeploy"
    effect = "Allow"
    actions = [
      "ecs:DescribeTaskDefinition",
      "ecs:RegisterTaskDefinition",
      "ecs:DescribeServices",
      "ecs:UpdateService",
    ]
    resources = ["*"]
  }

  statement {
    # register-task-definitionでタスク実行ロール/タスクロールを渡すために
    # 必要。devの命名規則に一致するロールのみに限定する。
    sid       = "PassEcsRoles"
    effect    = "Allow"
    actions   = ["iam:PassRole"]
    resources = ["arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/${var.project_name}-dev-*"]
  }
}

resource "aws_iam_role_policy" "ecr_push_ecs_deploy" {
  name   = "${var.project_name}-ecr-push-ecs-deploy"
  role   = aws_iam_role.ecr_push.id
  policy = data.aws_iam_policy_document.ecr_push_ecs_deploy_permissions.json
}
