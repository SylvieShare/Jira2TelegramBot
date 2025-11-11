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

		return processGetIssue(c, key, c.Upd.Message.Chat.ID)
	}
}

func processGetIssue(c *tg.Ctx, key string, chatId int64) error {
	ticket := c.TicketStore.Get(key)
	if ticket == nil {
		msg := tgbotapi.NewMessage(chatId, "⚠️ Тикет <code>"+key+"</code> был создан не в этом боте")
		msg.ParseMode = tgbotapi.ModeHTML
		_, _ = c.Bot.Send(msg)
		return nil
	}

	info, err := c.Jira.GetIssueStatus(c.Std, key)
	if err != nil {
		if errors.Is(err, jira.ErrNotFound) {
			msg := tgbotapi.NewMessage(chatId, text.TextGetStatusNotFound(key))
			msg.ParseMode = tgbotapi.ModeHTML
			_, sendErr := c.Bot.Send(msg)
			if sendErr != nil {
				return sendErr
			}
			return nil
		}
		msgText := fmt.Sprintf("Не удалось получить информацию по тикету %s: %v", key, err)
		_, sendErr := c.Bot.Send(tgbotapi.NewMessage(chatId, msgText))
		if sendErr != nil {
			return sendErr
		}
		return nil
	}

	msg := tgbotapi.NewMessage(chatId, text.TextGetStatus(info, ticket.Name, ticket.CreatorUsername))
	msg.ParseMode = tgbotapi.ModeHTML

	var rows [][]tgbotapi.InlineKeyboardButton

	if c.ReopenStatus == "" || !text.IsReadyStatus(info.Status) {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Обновить статус", fmt.Sprintf("%s|%s", actionStatus, info.Key)),
		))
		// rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		// 	tgbotapi.NewInlineKeyboardButtonData("Дополнить информацию", fmt.Sprintf("%s|%s", actionAddInfo, info.Key)),
		// ))
	}

	if c.ReopenStatus != "" && text.IsReadyStatus(info.Status) {
		button := tgbotapi.NewInlineKeyboardButtonData("Переоткрыть", fmt.Sprintf("%s|%s", actionReopen, info.Key))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
	}
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)

	_, err = c.Bot.Send(msg)
	c.TicketStore.UpdateStatus(info.Key, info.Status)
	return err
}

func sendChatTicketsDigest(c *tg.Ctx) error {
	chatID := c.Upd.Message.Chat.ID
	var tickets []tg.CreatedTicket
	if c.TicketStore != nil {
		tickets = c.TicketStore.ListByChatID(chatID)
	}

	msg := tgbotapi.NewMessage(chatID, text.TextTelegramTicketsMessage(tickets, c.Upd.Message.Chat.Title))
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := c.Bot.Send(msg)
	return err
}
