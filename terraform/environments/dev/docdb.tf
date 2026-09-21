# Amazon DocumentDB(MongoDB互換)。RDSと違い"publicly_accessible"に
# 相当する設定は無く、クラスタは常にVPC内のみに存在する。またRDSの
# db.t4g.microのようなバースト可能な最小構成が無く、db.t3.mediumが
# 実質的な最小インスタンスクラスのため、g4のRDS(db.t4g.micro)より
# 固定費が高くなる。

resource "aws_docdb_subnet_group" "this" {
  name       = "${var.project_name}-dev"
  subnet_ids = aws_subnet.public[*].id

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "random_password" "docdb" {
  length  = 32
  special = false
}

resource "aws_docdb_cluster" "app" {
  cluster_identifier = "${var.project_name}-dev"
  engine             = "docdb"
  engine_version     = var.docdb_engine_version

  master_username = var.docdb_master_username
  master_password = random_password.docdb.result

  db_subnet_group_name   = aws_docdb_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.docdb.id]
  storage_encrypted      = true

  # dev環境なので作り直しやすさを優先する。backup_retention_periodは
  # RDSと異なり0を指定できず1〜35が必須のため最小値の1にする。
  backup_retention_period = 1
  deletion_protection     = false
  skip_final_snapshot     = true
  apply_immediately       = true

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_docdb_cluster_instance" "app" {
  identifier         = "${var.project_name}-dev-0"
  cluster_identifier = aws_docdb_cluster.app.id
  instance_class     = var.docdb_instance_class
  apply_immediately  = true

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

# アプリが単一のMONGODB_URIとして読めるよう、接続文字列をまとめて
# Secrets Managerに保存する(ECSタスク定義からsecretsとして参照する)。
# DocumentDBはデフォルトでTLSが必須のため、Dockerイメージに焼き込んだ
# AmazonのCA証明書バンドル(terraform/../../Dockerfile参照)をtlsCAFileで
# 指定する。またDocumentDBは常にレプリカセット(rs0固定)として振る舞い、
# retryable writesをサポートしないため明示的に無効化する必要がある。
resource "aws_secretsmanager_secret" "mongodb_uri" {
  name                    = "${var.project_name}/dev/mongodb-uri"
  recovery_window_in_days = 0

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_secretsmanager_secret_version" "mongodb_uri" {
  secret_id = aws_secretsmanager_secret.mongodb_uri.id
  secret_string = join("", [
    "mongodb://${var.docdb_master_username}:${urlencode(random_password.docdb.result)}",
    "@${aws_docdb_cluster.app.endpoint}:${aws_docdb_cluster.app.port}/",
    "?tls=true&tlsCAFile=/etc/ssl/certs/rds-global-bundle.pem&replicaSet=rs0&retryWrites=false",
  ])
}
