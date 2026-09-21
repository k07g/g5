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

variable "vpc_cidr" {
  description = "dev環境用VPCのCIDR"
  type        = string
  default     = "10.30.0.0/16"
}

variable "public_subnet_cidrs" {
  description = "パブリックサブネットのCIDR(ALB/ECS/DocumentDBをここに配置し、NAT Gatewayなしで運用する)"
  type        = list(string)
  default     = ["10.30.0.0/20", "10.30.16.0/20"]
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

variable "docdb_instance_class" {
  description = "DocumentDBインスタンスクラス。RDSと異なりバースト可能な無料利用枠(t4g.micro相当)が無く、db.t3.medium が最小構成のため、g4のRDSより固定費が高くなる点に注意"
  type        = string
  default     = "db.t3.medium"
}

variable "docdb_engine_version" {
  description = "DocumentDBのエンジンバージョン。利用可能なバージョンはリージョン/アカウントにより異なるため、apply時にエラーになる場合は `aws docdb describe-db-engine-versions --engine docdb` で確認して上書きすること"
  type        = string
  default     = "5.0.0"
}

variable "docdb_master_username" {
  description = "DocumentDBのマスターユーザー名"
  type        = string
  default     = "g5"
}

variable "mongodb_database" {
  description = "アプリが使うデータベース名(MONGODB_DATABASE)"
  type        = string
  default     = "g5"
}

variable "cognito_user_pool_arn" {
  description = "アクセストークンの検証に使う、github.com/k07g/g4 が所有するdev環境Cognitoユーザープールのarn。g5自身はCognitoリソースを持たないため、g4のTerraform出力やAWSコンソール/CLI(aws cognito-idp list-user-pools 等)から取得して指定する"
  type        = string
}
