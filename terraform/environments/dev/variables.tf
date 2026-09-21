variable "aws_region" {
  description = "AWSリージョン"
  type        = string
  default     = "ap-northeast-1"
}

variable "project_name" {
  description = "リソース名のプレフィックスに使うプロジェクト名"
  type        = string
  default     = "g5"
}

variable "container_port" {
  description = "アプリコンテナがリッスンするポート"
  type        = number
  default     = 8080
}

variable "container_cpu" {
  description = "ECSタスクのCPUユニット(dev向けに最小構成)"
  type        = number
  default     = 256
}

variable "container_memory" {
  description = "ECSタスクのメモリ(MiB、dev向けに最小構成)"
  type        = number
  default     = 512
}

variable "desired_count" {
  description = "ECSサービスの希望タスク数"
  type        = number
  default     = 1
}

variable "log_retention_days" {
  description = "CloudWatch Logsの保持日数"
  type        = number
  default     = 14
}

variable "cognito_user_pool_arn" {
  description = "アクセストークンの検証に使う、github.com/k07g/g4 が所有するdev環境Cognitoユーザープールのarn。g5自身はCognitoリソースを持たないため、g4のTerraform出力やAWSコンソール/CLI(aws cognito-idp list-user-pools 等)から取得して指定する"
  type        = string
}

# --- g4のVPC/RDSを間借りするための変数 ---
# 独自DBの構築は一旦見送り(#7参照)、g4(https://github.com/k07g/g4)が
# 既に運用しているRDS PostgreSQLインスタンスを暫定的に共用する。g5の
# ECSタスクはg4のVPC・パブリックサブネットにそのままデプロイし(新規VPCは
# 作らない)、g4のRDSセキュリティグループに対してg5のECSタスクからの
# アクセスを許可するingressルールをこのTerraformから追加する(g4側の
# Terraformコード自体は変更しない。security_groups.tf参照)。
# これらの値はg4のTerraform出力に現状含まれないため、AWSコンソール/CLIで
# 取得して指定する。

variable "g4_vpc_id" {
  description = "g4のdev環境VPC ID。g5のECS/ALBはこのVPCにデプロイする"
  type        = string
}

variable "g4_public_subnet_ids" {
  description = "g4のdev環境パブリックサブネットID一覧。g5のECS/ALBをここに配置する"
  type        = list(string)
}

variable "g4_rds_security_group_id" {
  description = "g4のdev環境RDSインスタンスに付与されているセキュリティグループID。g5のECSサービスからの5432番アクセスを許可するingressルールをここに追加する"
  type        = string
}
