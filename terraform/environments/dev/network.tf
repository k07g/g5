# NAT Gatewayを使わず、ALB・ECSタスク・DocumentDBをすべてパブリック
# サブネットに配置してdev環境の固定費を抑える構成。ECSタスクには
# パブリックIPを付与して直接ECR/Cognitoに到達させ、DocumentDBは
# セキュリティグループでECSタスクからの接続のみに限定する(DocumentDBは
# クラスタがVPC内にのみ存在する設計のため、RDSのpublicly_accessibleに
# 相当するフラグ自体が無い)。

resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name      = "${var.project_name}-dev"
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name      = "${var.project_name}-dev"
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

data "aws_availability_zones" "available" {
  state = "available"
}

resource "aws_subnet" "public" {
  count = length(var.public_subnet_cidrs)

  vpc_id                  = aws_vpc.this.id
  cidr_block              = var.public_subnet_cidrs[count.index]
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name      = "${var.project_name}-dev-public-${count.index}"
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }

  tags = {
    Name      = "${var.project_name}-dev-public"
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_route_table_association" "public" {
  count = length(aws_subnet.public)

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}
