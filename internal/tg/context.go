package tg

import (
	"context"
	"log/slog"
	"regexp"

	"telegram-bot-jira/internal/jira"
	"telegram-bot-jira/internal/store"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Ctx struct {
	Std              context.Context
	Bot              *tgbotapi.BotAPI
	Upd              tgbotapi.Update
	Log              *slog.Logger
	Jira             *jira.Client
	HistoryMessages  *HistoryMessages
	TicketStore      *store.TicketStore
	ReopenStatus     string
	ReactionEmoji    string
	ProjectKeyRegexp *regexp.Regexp
}
