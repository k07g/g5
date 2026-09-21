terraform {
  # S3 backend の use_lockfile (S3ネイティブロック) は Terraform 1.10+ が必要
  required_version = ">= 1.10.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.64"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }

  # CI(GitHub Actions)からterraform applyを実行するため、S3 backendで
  # stateを永続化する。bucket/key/region はこのファイルに直書きせず、
  # `terraform init -backend-config=...` で渡す(ローカルではbackend.hcl、
  # CIでは環境変数/GitHub Actions変数を利用)。値の生成方法は
  # ../../bootstrap を参照。ロックはDynamoDBではなくS3ネイティブロック
  # (use_lockfile)を使うため、別途ロックテーブルは不要。
  backend "s3" {
    use_lockfile = true
  }
}

provider "aws" {
  region = var.aws_region
}
