package tg

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

type HistoryMessages struct {
	historyMapByChatId map[int64][]tgbotapi.Message
	limit              int
}

func NewHistoryMessages(limit int) *HistoryMessages {
	if limit <= 0 {
		limit = 10
	}
	return &HistoryMessages{
		historyMapByChatId: make(map[int64][]tgbotapi.Message),
		limit:              limit,
	}
}

func (h *HistoryMessages) AddMessage(message *tgbotapi.Message) {
    if h == nil || message == nil || message.Chat == nil {
        return
    }
    chatID := message.Chat.ID
    messages := append(h.historyMapByChatId[chatID], *message)
    if h.limit > 0 && len(messages) > h.limit {
        messages = messages[1:]
    }
    h.historyMapByChatId[chatID] = messages
}

func (h *HistoryMessages) GetMessages(chatId int64) []tgbotapi.Message {
    if h == nil {
        return nil
    }
    return h.historyMapByChatId[chatId]
}
