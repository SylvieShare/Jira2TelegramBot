package tg

import (
	"strings"

	"telegram-bot-jira/internal/text"
)

type Dispatcher struct {
	OnCallback           HandlerFunc
	OnCreateIssue        HandlerFunc
	OnGetIssue           HandlerFunc
	OnReplyBotForComment HandlerFunc
	OnMediaGroup         HandlerFunc
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{}
}

func (d *Dispatcher) Dispatch(ctx *Ctx) error {
	update := ctx.Upd
	if update.Message != nil {
		message := update.Message
		// Проверяем создание задачи
		if strings.HasPrefix(message.Text, "/create_issue") || strings.HasPrefix(message.Text, "@"+ctx.Tg.SelfUserName()) {
			return d.OnCreateIssue(ctx)
		}

		// Проверяем статус задачи
		if strings.HasPrefix(message.Text, "/status_issue") {
			// || message.ReplyToMessage != nil && message.ReplyToMessage.From.UserName == ctx.Bot.Self.UserName
			return d.OnGetIssue(ctx)
		}

		if message.ReplyToMessage != nil {
			// Check if this message is part of a media group and needs batching
			if message.Photo != nil || message.Document != nil || message.Video != nil || message.Audio != nil || message.Voice != nil {
				return d.OnMediaGroup(ctx)
			}
			// Проверяем ответ на задачу - есть реплай, автор релпая бот, в сообщении есть ключ задачи и якорь для ответа
			if message.ReplyToMessage != nil && message.ReplyToMessage.From.UserName == ctx.Tg.SelfUserName() {
				if strings.Contains(message.ReplyToMessage.Text, text.TextAnchorReplyJiraToTelegram()) ||
					strings.Contains(message.ReplyToMessage.Text, text.TextAnchorReplyStatusToJira()) {
					return d.OnReplyBotForComment(ctx)
				}
			}
		}
		ctx.HistoryMessages.AddMessage(update.Message)
		return nil
	}
	if update.CallbackQuery != nil && d.OnCallback != nil {
		return d.OnCallback(ctx)
	}
	return nil
}
