package handlers

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"telegram-bot-jira/internal/jira"
	"telegram-bot-jira/internal/text"
	"telegram-bot-jira/internal/tg"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func GetIssue() tg.HandlerFunc {
	return func(c *tg.Ctx) error {
		project := c.Jira.ProjectKey()
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(project) + `-\d+\b`)

		raw := tg.StripCommandText(c.Upd.Message.Text)

		var key string
		if m := re.FindString(raw); m != "" {
			key = strings.ToUpper(m)
		}

		if key == "" && c.Upd.Message.ReplyToMessage != nil {
			if m := re.FindString(c.Upd.Message.ReplyToMessage.Text); m != "" {
				key = strings.ToUpper(m)
			}
		}

		if key == "" {
			return sendChatTicketsDigest(c)
		}

		return processGetIssue(c, key)
	}
}

func processGetIssue(c *tg.Ctx, key string) error {
	ticket := c.TicketStore.Get(key)
	if ticket == nil {
		return c.Tg.SendMessageHTML("⚠️ Тикет <code>" + key + "</code> был создан не в этом боте")
	}

	info, err := c.Jira.GetIssueStatus(c.Std, key)
	if err != nil {
		if errors.Is(err, jira.ErrNotFound) {
			return c.Tg.SendMessageHTML(text.TextGetStatusNotFound(key))
		}
		return c.Tg.SendMessage(fmt.Sprintf("Не удалось получить информацию по тикету %s: %v", key, err))
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	if c.Params.ReopenStatus != "" && text.IsReadyStatus(info.Status) {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Переоткрыть", fmt.Sprintf("%s|%s", actionReopen, info.Key)),
		))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Обновить статус", fmt.Sprintf("%s|%s", actionStatus, info.Key)),
		))
	}

	c.TicketStore.UpdateStatus(info.Key, info.Status)
	return c.Tg.SendMessageHTML(text.TextGetStatus(info, ticket.Name, ticket.CreatorUsername), rows...)
}

func sendChatTicketsDigest(c *tg.Ctx) error {
	chatID := c.Upd.Message.Chat.ID
	var tickets []tg.CreatedTicket
	tickets = c.TicketStore.ListByChatID(chatID)

	return c.Tg.SendMessageHTML(text.TextTelegramTicketsMessage(tickets, c.Upd.Message.Chat.Title))
}
