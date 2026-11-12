package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/AmpyFin/yfinance-go"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/joho/godotenv"

	"github.com/0x726f6f6b6965/margin-trading/internal/service/leverage"
	"github.com/0x726f6f6b6965/margin-trading/internal/service/telegram"
)

var (
	tier          leverage.LeverageTier = leverage.Tier0x
	symbol        string                = "0050.TW"
	tierCh        chan leverage.LeverageTier
	amountCh      chan float64
	chatRoom      int64
	telegarmToken string
	tfp           float64
	equity        float64
)

func init() {
	// .env lives at the repo root; also try the parent dir when run from cmd/.
	if err := godotenv.Load(); err != nil {
		if err := godotenv.Load("../.env"); err != nil {
			log.Println("no .env file found, using existing environment variables")
		}
	}

	chatRoom, _ = strconv.ParseInt(os.Getenv("CHAT_ROOM"), 10, 64)
	telegarmToken = os.Getenv("BOT")
	tfp, _ = strconv.ParseFloat(os.Getenv("TFP"), 64)
}

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(run)
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tierCh = make(chan leverage.LeverageTier, 1)
	amountCh = make(chan float64, 1)

	client := yfinance.NewClient()

	bot, err := telegram.New(telegarmToken, chatRoom)
	if err != nil {
		return err
	}

	bot.RegisterTierHandler(tierHandler)
	bot.RegisterEqualityHandler(equalityHandler)

	go bot.Start(runCtx)

	if err := bot.SendMessage(runCtx, "請先使用'/tier'來設定目前level"); err != nil {
		return err
	}

	select {
	case <-runCtx.Done():
		return runCtx.Err()
	case tier = <-tierCh:
	}

	if err := bot.SendMessage(runCtx, "請先使用'/amount'來設定目前amount(百萬)"); err != nil {
		return err
	}

	select {
	case <-runCtx.Done():
		return runCtx.Err()
	case equity = <-amountCh:
	}

	m := leverage.NewLeverageStateMachine(tier, equity)

	data, err := leverage.GetDailyMarketInputFromYahoo(runCtx, client, symbol)
	if err != nil {
		return err
	}
	data.TaiexFuturesPrice = tfp
	data.AccountEquity = equity
	ans := m.EvaluateDailyClose(data)

	return bot.SendMessage(runCtx, fmt.Sprintf("%s\n%s\n%s",
		ans.TransitionPath, ans.Reason, ans.Description(),
	))
}

func tierHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	data := strings.Split(update.Message.Text, " ")
	if len(data) != 2 {
		return
	}
	switch data[1] {
	case "0":
		tier = leverage.Tier0x
	case "1":
		tier = leverage.Tier1x
	case "2":
		tier = leverage.Tier2x
	case "4":
		tier = leverage.Tier4x
	default:
		tier = leverage.Tier0x
	}
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatRoom,
		Text:      fmt.Sprintf("set tier to %s", data[1]),
		ParseMode: models.ParseModeMarkdown,
	})

	select {
	case tierCh <- tier:
	default:
	}
}

func equalityHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	match := telegram.EqualityCommandPattern.FindStringSubmatch(update.Message.Text)
	if len(match) != 2 {
		return
	}
	amount, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return
	}
	money := amount * 1000000.00

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatRoom,
		Text:   fmt.Sprintf("set equity to %s millions", match[1]),
	})

	select {
	case amountCh <- money:
	default:
	}
}
