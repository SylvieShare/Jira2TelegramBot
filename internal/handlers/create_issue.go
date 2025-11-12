package handlers

import (
	"strings"

	"telegram-bot-jira/internal/text"
	"telegram-bot-jira/internal/tg"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func CreateIssue() tg.HandlerFunc {
	return func(c *tg.Ctx) error {
		messagesInHistory := c.HistoryMessages.GetMessages(c.Upd.Message.Chat.ID)
		titleIssue := text.TextTitleIssue(c.Upd.Message.Chat.Title)
		descriptionADF := text.TextDescriptionADF(titleIssue, messagesInHistory, "")
		key, _, err := c.Jira.CreateIssue(c.Std, titleIssue, descriptionADF)
		if err != nil {
			_, sendErr := c.Bot.Send(tgbotapi.NewMessage(c.Upd.Message.Chat.ID, text.TextErrorCreateTicket(err)))
			if sendErr != nil {
				return sendErr
			}
			return nil
		}
		c.Log.Info("Issue created", "key", key)

		payload := strings.TrimSpace(tg.StripCommandText(c.Upd.Message.Text))
		payload, _ = strings.CutPrefix(payload, "@"+c.Bot.Self.UserName)
		storeUsername := ""
		if c.Upd.Message.From != nil {
			storeUsername = c.Upd.Message.From.UserName
		}
		storeName := ""
		if payload != "" {
			fields := strings.Fields(payload)
			for i, f := range fields {
				if strings.HasPrefix(f, "@") && len(f) > 1 {
					storeUsername = strings.TrimPrefix(f, "@")
					fields = append(fields[:i], fields[i+1:]...)
					break
				}
			}
			storeName = strings.TrimSpace(strings.Join(fields, " "))
		}

		c.TicketStore.Add(c.Upd.Message.Chat.ID, key, "", storeName, storeUsername)

		return processGetIssue(c, key, c.Upd.Message.Chat.ID)
	}
}
