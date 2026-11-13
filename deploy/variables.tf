variable "aws_region" {
  description = "AWS deployment region"
  type        = string
  default     = "ap-northeast-1"
}

variable "function_name" {
  description = "AWS Lambda function name"
  type        = string
  default     = "margin-trading-bot"
}

variable "architecture" {
  description = "Lambda CPU architecture (arm64 or x86_64)"
  type        = string
  default     = "arm64"

  validation {
    condition     = contains(["arm64", "x86_64"], var.architecture)
    error_message = "architecture must be either 'arm64' or 'x86_64'."
  }
}

variable "timeout" {
  description = "Lambda execution timeout in seconds (max 900)"
  type        = number
  default     = 300
}

variable "memory_size" {
  description = "Lambda memory size in MB"
  type        = number
  default     = 128
}

variable "log_retention_in_days" {
  description = "CloudWatch log retention in days"
  type        = number
  default     = 14
}

variable "chat_room" {
  description = "Telegram chat room ID (CHAT_ROOM)"
  type        = string
}

variable "telegram_bot_token" {
  description = "Telegram Bot Token (BOT)"
  type        = string
  sensitive   = true
}

variable "tfp" {
  description = "Taiex Futures Price (TFP)"
  type        = string
  default     = "0"
}

variable "schedule_expression" {
  description = "EventBridge schedule expression in UTC (default: Mon-Fri 13:30 UTC+8 = 05:30 UTC)"
  type        = string
  default     = "cron(30 5 ? * MON-FRI *)"
}

