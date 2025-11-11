package tg

import (
	"encoding/json"
	"strconv"
	"strings"

	"telegram-bot-jira/internal/text"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Dispatcher struct {
	OnCallback    HandlerFunc
	OnCreateIssue HandlerFunc
	OnGetIssue    HandlerFunc
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		// OnCommand: make(map[string]HandlerFunc),
	}
}

func (d *Dispatcher) Dispatch(ctx *Ctx) error {
	update := ctx.Upd
	if update.Message != nil {
		message := update.Message
		chatTitle := message.Chat.Title
		// Проверяем создание задачи
		if strings.HasPrefix(message.Text, "/create_issue") || strings.HasPrefix(message.Text, "@"+ctx.Bot.Self.UserName) {
			return d.OnCreateIssue(ctx)
		}

		// Проверяем ответ на задачу - есть реплай, автор релпая бот, в сообщении есть ключ задачи и якорь для ответа
		if message.ReplyToMessage != nil && message.ReplyToMessage.From.UserName == ctx.Bot.Self.UserName {
			if strings.Contains(message.ReplyToMessage.Text, text.TextAnchorReplyJiraToTelegram()) {
				keyInMessageReply := ctx.ProjectKeyRegexp.FindString(message.ReplyToMessage.Text)
				if keyInMessageReply != "" {
					err := ctx.Jira.AddComment(ctx.Std, keyInMessageReply, text.TextJiraCommentUserFromTelegram(message.Text, message.From, chatTitle, message.ReplyToMessage.Text))
					if err != nil {
						return err
					}
					reactToMessage(ctx, update.Message)
					return nil
				}
			}
			if strings.Contains(message.ReplyToMessage.Text, text.TextAnchorReplyStatusToJira()) {
				keyInMessageReply := ctx.ProjectKeyRegexp.FindString(message.ReplyToMessage.Text)
				if keyInMessageReply != "" {
					err := ctx.Jira.AddComment(ctx.Std, keyInMessageReply, text.TextJiraCommentUserFromTelegram(message.Text, message.From, chatTitle, ""))
					if err != nil {
						return err
					}
					reactToMessage(ctx, update.Message)
					return nil
				}
			}
		}

		// Проверяем статус задачи
		if strings.HasPrefix(message.Text, "/status_issue") {
			// || message.ReplyToMessage != nil && message.ReplyToMessage.From.UserName == ctx.Bot.Self.UserName
			return d.OnGetIssue(ctx)
		}
		ctx.HistoryMessages.AddMessage(update.Message)
		return nil
	}
	if update.CallbackQuery != nil && d.OnCallback != nil {
		return d.OnCallback(ctx)
	}
	return nil
}

func IsBotMentioned(message *tgbotapi.Message, botUsername string) bool {
	if message == nil || message.Entities == nil || message.Text == "" || botUsername == "" {
		return false
	}

	target := "@" + botUsername
	for _, entity := range message.Entities {
		switch entity.Type {
		case "mention":
			if mention := substringRunes(message.Text, entity.Offset, entity.Length); mention == target {
				return true
			}
		}
	}
	return false
}

func substringRunes(text string, offset, length int) string {
	if offset < 0 || length <= 0 {
		return ""
	}
	runes := []rune(text)
	if offset >= len(runes) {
		return ""
	}
	end := offset + length
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[offset:end])
}

func reactToMessage(ctx *Ctx, msg *tgbotapi.Message) {
	if msg == nil {
		return
	}

	emoji := strings.TrimSpace(ctx.ReactionEmoji)
	if emoji != "" {
		if err := setMessageReaction(ctx.Bot, msg.Chat.ID, msg.MessageID, emoji); err != nil {
			ctx.Log.Error("Failed to set message reaction", "emoji", emoji, "err", err)
		}
	}
}

type reactionType struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji,omitempty"`
}

func setMessageReaction(bot *tgbotapi.BotAPI, chatID int64, messageID int, emoji string) error {
	payload, err := json.Marshal([]reactionType{
		{Type: "emoji", Emoji: emoji},
	})
	if err != nil {
		return err
	}

	params := tgbotapi.Params{
		"chat_id":    strconv.FormatInt(chatID, 10),
		"message_id": strconv.Itoa(messageID),
		"reaction":   string(payload),
	}

	_, err = bot.MakeRequest("setMessageReaction", params)
	return err
}
