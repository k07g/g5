terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.64"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }

  # このbootstrapはTerraform stateを保存するためのS3バケット自体を作る
  # ため、循環依存を避けて意図的にローカルstateのまま運用する。
  # apply後に生成される terraform.tfstate は誤って削除しないよう
  # 大切に保管すること(このディレクトリはめったに変更しない想定)。
}

provider "aws" {
  region = var.aws_region
}
