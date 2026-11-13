output "lambda_function_name" {
  description = "Name of the deployed Lambda function"
  value       = aws_lambda_function.margin_trading.function_name
}

output "lambda_function_arn" {
  description = "ARN of the deployed Lambda function"
  value       = aws_lambda_function.margin_trading.arn
}

output "lambda_role_arn" {
  description = "IAM Role ARN used by the Lambda function"
  value       = aws_iam_role.lambda_exec.arn
}

output "cloudwatch_log_group" {
  description = "CloudWatch Log Group name for Lambda logs"
  value       = aws_cloudwatch_log_group.lambda_logs.name
}

