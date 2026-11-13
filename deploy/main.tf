locals {
  project_root = abspath("${path.module}/..")
  build_dir    = "${path.module}/build"
  binary_path  = "${local.build_dir}/bootstrap"
  zip_path     = "${local.build_dir}/function.zip"
  goarch       = var.architecture == "arm64" ? "arm64" : "amd64"

  # Track hashes of all Go source and module files to trigger rebuilds
  source_files = setunion(
    fileset(local.project_root, "go.mod"),
    fileset(local.project_root, "go.sum"),
    fileset(local.project_root, "cmd/**/*.go"),
    fileset(local.project_root, "internal/**/*.go"),
  )
  source_hash = sha256(join("", [for f in sort(local.source_files) : filesha256("${local.project_root}/${f}")]))
}

# 1. Build ./cmd/main.go into Linux binary named "bootstrap" (required by provided.al2023)
resource "terraform_data" "build_lambda_binary" {
  triggers_replace = [
    local.source_hash,
    local.goarch,
  ]

  provisioner "local-exec" {
    working_dir = local.project_root
    command     = "mkdir -p ${abspath(local.build_dir)} && GOOS=linux GOARCH=${local.goarch} CGO_ENABLED=0 go build -tags lambda.norpc -ldflags='-s -w' -o ${abspath(local.binary_path)} ./cmd/main.go"
  }
}

# 2. Package the bootstrap binary into a zip archive
data "archive_file" "lambda_zip" {
  type        = "zip"
  source_file = local.binary_path
  output_path = local.zip_path

  depends_on = [terraform_data.build_lambda_binary]
}

# 3. IAM Role for Lambda Execution
data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda_exec" {
  name               = "${var.function_name}-exec-role"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

resource "aws_iam_role_policy_attachment" "lambda_basic_execution" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# 4. CloudWatch Log Group
resource "aws_cloudwatch_log_group" "lambda_logs" {
  name              = "/aws/lambda/${var.function_name}"
  retention_in_days = var.log_retention_in_days
}

# 5. AWS Lambda Function
resource "aws_lambda_function" "margin_trading" {
  function_name    = var.function_name
  role             = aws_iam_role.lambda_exec.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = [var.architecture]
  filename         = data.archive_file.lambda_zip.output_path
  source_code_hash = data.archive_file.lambda_zip.output_base64sha256
  timeout          = var.timeout
  memory_size      = var.memory_size

  environment {
    variables = {
      CHAT_ROOM = var.chat_room
      BOT       = var.telegram_bot_token
      TFP       = var.tfp
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.lambda_basic_execution,
    aws_cloudwatch_log_group.lambda_logs,
  ]
}

# 6. Optional EventBridge Schedule Rule
resource "aws_cloudwatch_event_rule" "schedule" {
  count               = var.schedule_expression != "" ? 1 : 0
  name                = "${var.function_name}-schedule"
  description         = "Schedule trigger for ${var.function_name}"
  schedule_expression = var.schedule_expression
}

resource "aws_cloudwatch_event_target" "lambda_target" {
  count     = var.schedule_expression != "" ? 1 : 0
  rule      = aws_cloudwatch_event_rule.schedule[0].name
  target_id = "${var.function_name}-target"
  arn       = aws_lambda_function.margin_trading.arn
}

resource "aws_lambda_permission" "allow_eventbridge" {
  count         = var.schedule_expression != "" ? 1 : 0
  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.margin_trading.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.schedule[0].arn
}

