package tg

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"telegram-bot-jira/internal/common"
	"telegram-bot-jira/internal/jira"
	"telegram-bot-jira/internal/store"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Ctx struct {
	Std             context.Context
	Tg              *BotTgAction
	Upd             tgbotapi.Update
	Log             *slog.Logger
	Jira            *jira.Client
	HistoryMessages *HistoryMessages
	TicketStore     *store.TicketStore
	Params          CtxParams
}

type CtxParams struct {
	ReopenStatus     string
	ProjectKey       string
	ProjectKeyRegexp *regexp.Regexp
	reactionEmoji    string
	errorChatId      int64
}

type BotTgAction struct {
	ctx   *Ctx
	tgApi *tgbotapi.BotAPI
}

type reactionType struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji,omitempty"`
}

func (bot *BotTgAction) SelfUserName() string {
	return bot.tgApi.Self.UserName
}

func (bot *BotTgAction) CurrentChatId() int64 {
	if bot.ctx.Upd.Message != nil && bot.ctx.Upd.Message.Chat != nil {
		return bot.ctx.Upd.Message.Chat.ID
	} else if bot.ctx.Upd.CallbackQuery != nil && bot.ctx.Upd.CallbackQuery.Message != nil && bot.ctx.Upd.CallbackQuery.Message.Chat != nil {
		return bot.ctx.Upd.CallbackQuery.Message.Chat.ID
	}
	return 0
}

func (bot *BotTgAction) TryGetTicketKeyInMessage() string {
	if bot.ctx.Upd.Message == nil {
		bot.ctx.Log.Error("cannot get message from udp")
		return ""
	}
	text := bot.ctx.Upd.Message.Text
	return bot.ctx.Params.ProjectKeyRegexp.FindString(text)
}

func (bot *BotTgAction) SendMessageErrorChat(text string) error {
	chatId := bot.ctx.Params.errorChatId
	if chatId != 0 {
		return nil
	}
	if bot.ctx.Upd.Message == nil || bot.ctx.Upd.Message.Chat == nil {
		bot.ctx.Log.Error("cannot send message: chat not found")
		return nil
	}
	_, err := bot.tgApi.Send(tgbotapi.NewMessage(chatId, text))
	return err
}

func (bot *BotTgAction) SendMessage(text string) error {
	_, err := bot.tgApi.Send(tgbotapi.NewMessage(bot.CurrentChatId(), text))
	return err
}

func (bot *BotTgAction) SendMessageHTML(text string, buttons ...[]tgbotapi.InlineKeyboardButton) error {
	msg := tgbotapi.NewMessage(bot.CurrentChatId(), text)
	msg.ParseMode = tgbotapi.ModeHTML
	if len(buttons) > 0 {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	}
	_, err := bot.tgApi.Send(msg)
	return err
}

func (bot *BotTgAction) ReactCurrentMessageIsRead() {
	bot.ReactMessageIsRead(bot.ctx.Upd.Message)
}

func (bot *BotTgAction) ReactMessageIsRead(message *tgbotapi.Message) {
	if bot.ctx.Upd.Message == nil {
		return
	}
	msg := bot.ctx.Upd.Message

	emoji := strings.TrimSpace(bot.ctx.Params.reactionEmoji)
	if emoji != "" {
		payload, err := json.Marshal([]reactionType{
			{Type: "emoji", Emoji: emoji},
		})
		if err != nil {
			bot.ctx.Log.Error("Failed to set message reaction", "emoji", emoji, "err", err)
			return
		}

		params := tgbotapi.Params{
			"chat_id":    strconv.FormatInt(msg.Chat.ID, 10),
			"message_id": strconv.Itoa(msg.MessageID),
			"reaction":   string(payload),
		}

		_, err = bot.tgApi.MakeRequest("setMessageReaction", params)
		if err != nil {
			bot.ctx.Log.Error("Failed to set message reaction", "emoji", emoji, "err", err)
		}
	}
}

func (tg *BotTgAction) ExtractFile(message *tgbotapi.Message) ([]common.FileInfo, error) {
	if message == nil {
		return nil, fmt.Errorf("message is nil")
	}
	var fileUrls []common.FileInfo

	// Handle photo messages
	if len(message.Photo) > 0 {
		// Get the largest photo (last element in the slice)
		photo := message.Photo[len(message.Photo)-1]

		fileID := photo.FileID
		fileURL, err := tg.tgApi.GetFileDirectURL(fileID)
		if err != nil {
			return fileUrls, fmt.Errorf("failed to get photo URL: %w", err)
		}

		fileUrls = append(fileUrls, common.FileInfo{Url: fileURL})
	}

	// Handle document messages (all types)
	if message.Document != nil {
		fileID := message.Document.FileID
		fileURL, err := tg.tgApi.GetFileDirectURL(fileID)
		if err != nil {
			return fileUrls, fmt.Errorf("failed to get document URL: %w", err)
		}

		fileUrls = append(fileUrls, common.FileInfo{Url: fileURL, Name: message.Document.FileName})
	}

	// Handle video messages
	if message.Video != nil {
		fileID := message.Video.FileID
		fileURL, err := tg.tgApi.GetFileDirectURL(fileID)
		if err != nil {
			return fileUrls, fmt.Errorf("failed to get video URL: %w", err)
		}

		fileUrls = append(fileUrls, common.FileInfo{Url: fileURL, Name: message.Video.FileName})
	}

	// Handle audio messages
	if message.Audio != nil {
		fileID := message.Audio.FileID
		fileURL, err := tg.tgApi.GetFileDirectURL(fileID)
		if err != nil {
			return fileUrls, fmt.Errorf("failed to get audio URL: %w", err)
		}

		fileUrls = append(fileUrls, common.FileInfo{Url: fileURL, Name: message.Audio.FileName})
	}

	// Handle voice messages
	if message.Voice != nil {
		fileID := message.Voice.FileID
		fileURL, err := tg.tgApi.GetFileDirectURL(fileID)
		if err != nil {
			return fileUrls, fmt.Errorf("failed to get voice URL: %w", err)
		}

		fileUrls = append(fileUrls, common.FileInfo{Url: fileURL})
	}

	return fileUrls, nil
}
