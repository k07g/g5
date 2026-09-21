# ALB/ECSのセキュリティグループはg5自身が作成するが、いずれもg4の既存VPC
# (var.g4_vpc_id)内に作成する(g5専用のVPCは持たない。variables.tf参照)。

resource "aws_security_group" "alb" {
  name        = "${var.project_name}-dev-alb"
  description = "Allow inbound HTTP from the internet"
  vpc_id      = var.g4_vpc_id

  tags = {
    Name      = "${var.project_name}-dev-alb"
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_vpc_security_group_ingress_rule" "alb_http" {
  security_group_id = aws_security_group.alb.id
  description       = "HTTP from anywhere"
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 80
  to_port           = 80
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "alb_all" {
  security_group_id = aws_security_group.alb.id
  description       = "Allow all outbound"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

resource "aws_security_group" "ecs_service" {
  name        = "${var.project_name}-dev-ecs-service"
  description = "ECS service tasks"
  vpc_id      = var.g4_vpc_id

  tags = {
    Name      = "${var.project_name}-dev-ecs-service"
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_vpc_security_group_ingress_rule" "ecs_from_alb" {
  security_group_id            = aws_security_group.ecs_service.id
  description                  = "App traffic from the ALB only"
  referenced_security_group_id = aws_security_group.alb.id
  from_port                    = var.container_port
  to_port                      = var.container_port
  ip_protocol                  = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "ecs_all" {
  security_group_id = aws_security_group.ecs_service.id
  description       = "Allow all outbound (ECR pull, Cognito, RDS)"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

# g4のRDSセキュリティグループ(g4のTerraformが所有)に対して、g5のECS
# サービスからのアクセスを許可するingressルールをこちらから追加する。
# セキュリティグループ本体ではなく個別のルールリソースなので、g4側の
# Terraformコード・stateには一切触れずに済む(セキュリティグループIDだけ
# 知っていればよい)。
resource "aws_vpc_security_group_ingress_rule" "g4_rds_from_g5_ecs" {
  security_group_id            = var.g4_rds_security_group_id
  description                  = "PostgreSQL from the g5 ECS service (temporary shared-RDS arrangement, see #7)"
  referenced_security_group_id = aws_security_group.ecs_service.id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
}
