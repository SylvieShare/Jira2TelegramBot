package tg

import (
	"context"
	"log/slog"
	"regexp"
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
				ctx := &Ctx{
					Std:             ctx,
					Upd:             upd,
					Log:             b.log.With("tg-worker", id),
					Jira:            b.jira,
					HistoryMessages: b.historyMessages,
					TicketStore:     b.ticketStore,
					Params: CtxParams{
						ReopenStatus:     b.cfg.JiraReopenStatus,
						ProjectKey:       b.cfg.JiraProjectKey,
						ProjectKeyRegexp: regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(b.cfg.JiraProjectKey) + `-\d+\b`),
						reactionEmoji:    b.cfg.TelegramReactionEmoji,
						errorChatId:      int64(b.cfg.ErrorChatID),
					},
				}
				ctx.Tg = &BotTgAction{
					ctx:              ctx,
				}
				err := b.dispatch.Dispatch(ctx)
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
