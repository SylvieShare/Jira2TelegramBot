package handlers

import (
	"telegram-bot-jira/internal/tg"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func Start() tg.HandlerFunc {
	return func(c *tg.Ctx) error {
		msg := tgbotapi.NewMessage(c.Upd.Message.Chat.ID, "Привет! Я бот на Go (long polling)")
		_, err := c.Bot.Send(msg)
		return err
	}
}

func Help() tg.HandlerFunc {
	return func(c *tg.Ctx) error {
		msg := tgbotapi.NewMessage(c.Upd.Message.Chat.ID, "Команды: /start /help")
		_, err := c.Bot.Send(msg)
		return err
	}
}
