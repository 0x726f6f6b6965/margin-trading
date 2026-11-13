# Margin Trading Bot

An automated Taiwan futures position-sizing and leverage management bot written in Go. It evaluates daily market close indicators on Yuanta Taiwan 50 ETF (`0050.TW`) via Yahoo Finance, interacts with the user through Telegram to confirm current leverage tier and account equity, and computes target TAIFEX futures contract allocations using a 4-tier asymmetric state machine.

Supports both standalone local execution and serverless scheduled execution on **AWS Lambda** (provisioned via **Terraform** and triggered by **Amazon EventBridge**).

---

## Features

- **4-Tier Asymmetric Leverage State Machine**:
  - **`0x` (Cash / Hedge)**: Bear market main drawdown defense.
  - **`1x` (Defensive Base)**: Bull market pullback support near the 60-day moving average (`60MA`).
  - **`2x` (Intermediate Buffer / Dip Buy)**: Left-side extreme oversold dip-buying below `60MA` or right-side transition buffer.
  - **`4x` (Full Bullish Trend)**: Full-speed trend following above `20MA` and `60MA`, with dynamic equity-band rebalancing ($\pm 10\%$).
- **Automated Technical Indicator Pipeline**:
  - Fetches daily OHLC data for `0050.TW` from Yahoo Finance.
  - Computes `20MA`, `60MA`, `60MA` slope & bias percentage, and 6-day / 14-day Wilder's RSI.
- **TAIFEX Futures Contract Sizing**:
  - Calculates optimal contract counts for **Mini-TAIEX (`MTX`, NT\$50/pt)**, **Micro-TAIEX (`TMF`, NT\$10/pt)**, and **Mini-TSMC (`QFF`, 100 shares/contract)**, alongside actual total exposure and effective leverage.
- **Interactive Telegram Workflow**:
  - Prompts the user in a Telegram chat room for the current tier (`/tier`) and account equity (`/amount` in millions NTD) before running the daily evaluation and reporting the position decision.
- **Serverless AWS Deployment**:
  - Built-in Terraform configuration under `deploy/` for packaging the Go binary (`provided.al2023` on `arm64`), deploying to AWS Lambda, and scheduling weekday runs at **13:30 UTC+8** via EventBridge.

---

## Project Structure

```text
.
├── cmd/
│   └── main.go                        # Application entrypoint (CLI & AWS Lambda handler)
├── internal/
│   └── service/
│       ├── leverage/                  # 4-tier leverage state machine & market data logic
│       │   ├── condition.go           # Technical indicator & transition conditions
│       │   ├── def.go                 # Types, tiers, TAIFEX contract specs, and decisions
│       │   ├── service.go             # State machine transitions & contract allocation
│       │   ├── utils.go               # Yahoo Finance data fetching & MA/RSI calculations
│       │   └── utils_test.go          # Unit tests for market indicator utilities
│       └── telegram/
│           └── service.go             # Telegram bot wrapper & command regex handlers
├── deploy/                            # Terraform infrastructure for AWS Lambda
│   ├── main.tf                        # Lambda build, archive, IAM, Log Group, EventBridge
│   ├── variables.tf                   # Configurable deployment variables
│   ├── outputs.tf                     # Deployed resource outputs
│   ├── versions.tf                    # Terraform & provider version constraints
│   └── terraform.tfvars.example       # Example variable values
├── go.mod
└── go.sum
```

---

## Prerequisites

- **Go** `1.27+`
- **Telegram Bot Token** (from [@BotFather](https://t.me/BotFather)) and target **Chat ID**
- *(Optional, for cloud deployment)* **Terraform** `1.5+` and configured **AWS CLI** credentials

---

## Environment Variables

Create a `.env` file in the repository root when running locally:

```dotenv
BOT=your_telegram_bot_token
CHAT_ROOM=-1004224867347
TFP=30000
```

| Variable | Description |
| :--- | :--- |
| `BOT` | Telegram Bot API token |
| `CHAT_ROOM` | Target Telegram chat / group ID |
| `TFP` | Near-month TAIEX Futures closing points (e.g. `30000`) |

---

## Running Locally

1. Install dependencies:
   ```bash
   go mod download
   ```

2. Run the bot:
   ```bash
   go run ./cmd/main.go
   ```

3. Follow the prompts in your Telegram chat room:
   - Send `/tier <0|1|2|4>` to set your current leverage tier (e.g., `/tier 2`).
   - Send `/amount <millions>` to set your current account equity in millions NTD (e.g., `/amount 1.5` for NT\$1,500,000).
   - The bot will evaluate `0050.TW` market data and reply with the state transition path, trigger reason, recommended `MTX`/`TMF`/`QFF` contracts, total exposure, and effective leverage.

---

## Deploying to AWS Lambda with Terraform

The `deploy/` directory contains a complete Terraform setup that automatically cross-compiles `./cmd/main.go` for Linux (`provided.al2023`), zips the `bootstrap` binary, and provisions the AWS Lambda function and EventBridge schedule.

1. Initialize and configure variables:
   ```bash
   cd deploy
   cp terraform.tfvars.example terraform.tfvars
   ```
   Edit `terraform.tfvars` with your `telegram_bot_token`, `chat_room`, and `tfp`.

2. Initialize Terraform and deploy:
   ```bash
   terraform init
   terraform plan
   terraform apply
   ```

### Schedule Configuration

- **Default Production Schedule** (Monday–Friday at 13:30 UTC+8 / 05:30 UTC):
  ```hcl
  schedule_expression = "cron(30 5 ? * MON-FRI *)"
  ```
- **Quick Testing Schedule** (trigger every 3 minutes):
  ```hcl
  schedule_expression = "rate(3 minutes)"
  ```
- **Manual One-Off Invocation via AWS CLI**:
  ```bash
  aws lambda invoke \
    --function-name margin-trading-bot \
    --region ap-northeast-1 \
    response.json
  ```

---

## Running Tests

```bash
go test ./...
```

