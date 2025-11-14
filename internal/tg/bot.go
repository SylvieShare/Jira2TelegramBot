package tg

import (
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"telegram-bot-jira/internal/config"
	"telegram-bot-jira/internal/jira"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api             *tgbotapi.BotAPI
	log             *slog.Logger
	updCfg          tgbotapi.UpdateConfig
	dispatch        *Dispatcher
	jira            *jira.Client
	historyMessages *HistoryMessages
	ticketStore     *TicketStore
	cfg             config.Config
}

func New(api *tgbotapi.BotAPI, log *slog.Logger, cfg config.Config, d *Dispatcher, jiraClient *jira.Client) *Bot {
	return &Bot{
		api:             api,
		log:             log,
		updCfg:          tgbotapi.NewUpdate(0),
		dispatch:        d,
		jira:            jiraClient,
		historyMessages: NewHistoryMessages(cfg.HistoryMessagesLimit),
		ticketStore:     NewTicketStore(),
		cfg:             cfg,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	if err := b.initCommands(); err != nil {
		b.log.Warn("failed to initialize bot commands", "err", err)
	}

	b.updCfg.Timeout = 60
	updatesChan := b.api.GetUpdatesChan(b.updCfg)
	jobs := make(chan tgbotapi.Update, 1024)

	var wg sync.WaitGroup
	for i := 0; i < b.cfg.Workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for upd := range jobs {
				c := &Ctx{
					Std:              ctx,
					Bot:              b.api,
					Upd:              upd,
					Log:              b.log.With("tg-worker", id),
					Jira:             b.jira,
					HistoryMessages:  b.historyMessages,
					TicketStore:      b.ticketStore,
					ReopenStatus:     b.cfg.JiraReopenStatus,
					ReactionEmoji:    b.cfg.TelegramReactionEmoji,
					ProjectKeyRegexp: regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(b.cfg.JiraProjectKey) + `-\d+\b`),
				}
				err := b.dispatch.Dispatch(c)
				if err != nil {
					b.log.Error("failed to dispatch update", "err", err)
				}
			}
		}(i)
	}

	// Initial sync from aggregate issue, if configured
	if b.cfg.AggregateIssueKey != "" {
		err := b.syncAggregateFromJira(ctx)
		if err != nil {
			b.log.Error("Error load jira context issue", "key", b.cfg.AggregateIssueKey)
		}
	}

	// Background polling goroutine
	go b.pollTickets(ctx)

	for {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return nil
		case upd := <-updatesChan:
			jobs <- upd
		}
	}
}

func (b *Bot) initCommands() error {
	commands := []tgbotapi.BotCommand{
		{Command: "create_issue", Description: "Создать Jira задачу"},
		{Command: "status_issue", Description: "Узнать статус Jira задачи"},
	}

	_, err := b.api.Request(tgbotapi.NewSetMyCommands(commands...))
	return err
}

type reactionType struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji,omitempty"`
}

func (ctx *Ctx) ReactToMessage(msg *tgbotapi.Message) {
	if msg == nil {
		return
	}

	emoji := strings.TrimSpace(ctx.ReactionEmoji)
	if emoji != "" {
		payload, err := json.Marshal([]reactionType{
			{Type: "emoji", Emoji: emoji},
		})
		if err != nil {
			ctx.Log.Error("Failed to set message reaction", "emoji", emoji, "err", err)
			return
		}

		params := tgbotapi.Params{
			"chat_id":    strconv.FormatInt(msg.Chat.ID, 10),
			"message_id": strconv.Itoa(msg.MessageID),
			"reaction":   string(payload),
		}

		_, err = ctx.Bot.MakeRequest("setMessageReaction", params)
		if err != nil {
			ctx.Log.Error("Failed to set message reaction", "emoji", emoji, "err", err)
		}
	}
}
