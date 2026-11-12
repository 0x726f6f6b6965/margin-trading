package telegram

import (
	"context"
	"regexp"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var (
	// TierCommandPattern 比對 '/tier {number}'，number 僅允許 0/1/2/4；
	// 群組中 Telegram 會送出 '/tier@BotName 2'，故允許可選的 @BotName。
	// 第 1 個 capture group 為檔位數字。
	TierCommandPattern = regexp.MustCompile(`^/tier(?:@\w+)?\s+([0124])\s*$`)
	// EqualityCommandPattern 比對 '/amount {number}'，number 允許數字與小數點但不允許負數；
	// 群組中 Telegram 會送出 '/amount@BotName 1000000.5'，故允許可選的 @BotName。
	// 第 1 個 capture group 為金額數字。
	EqualityCommandPattern = regexp.MustCompile(`^/amount(?:@\w+)?\s+(\d+(?:\.\d+)?)\s*$`)
)

type service struct {
	client     *bot.Bot
	chatRoomID int64
}

func New(token string, chatRoomID int64) (*service, error) {
	c, err := bot.New(token)
	if err != nil {
		return nil, err
	}

	return new(service{
		client:     c,
		chatRoomID: chatRoomID,
	}), nil
}

func (s *service) SendMessage(ctx context.Context, text string) error {
	_, err := s.client.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: s.chatRoomID,
		Text:   text,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *service) RegisterTierHandler(handler func(ctx context.Context, b *bot.Bot, update *models.Update)) {
	s.client.RegisterHandlerRegexp(bot.HandlerTypeMessageText, TierCommandPattern, handler)
}

func (s *service) RegisterEqualityHandler(handler func(ctx context.Context, b *bot.Bot, update *models.Update)) {
	s.client.RegisterHandlerRegexp(bot.HandlerTypeMessageText, EqualityCommandPattern, handler)
}

func (s *service) Start(ctx context.Context) {
	s.client.Start(ctx)
}
