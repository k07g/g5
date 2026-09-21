resource "aws_lb" "app" {
  name               = "${var.project_name}-dev"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = var.g4_public_subnet_ids

  # dev環境なので削除保護は無効(作り直しやすさを優先)
  enable_deletion_protection = false

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_lb_target_group" "app" {
  name        = "${var.project_name}-dev"
  port        = var.container_port
  protocol    = "HTTP"
  vpc_id      = var.g4_vpc_id
  target_type = "ip"

  health_check {
    path                = "/healthz"
    matcher             = "200"
    interval            = 30
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }

  tags = {
    Project   = var.project_name
    ManagedBy = "terraform"
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.app.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app.arn
  }
}
