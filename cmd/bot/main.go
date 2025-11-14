package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"

	"telegram-bot-jira/internal/config"
	"telegram-bot-jira/internal/handlers"
	"telegram-bot-jira/internal/jira"
	"telegram-bot-jira/internal/logx"
	"telegram-bot-jira/internal/tg"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	cfg := config.Load()
	logger := logx.New(cfg.LogLevel)

	tgApi, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		panic(err)
	}
	jiraClient, err := jira.New(cfg)
	if err != nil {
		panic(err)
	}

	tgApi.Debug = cfg.LogLevel == "debug"
	logger.Info("bot authorized", slog.String("as", tgApi.Self.UserName))

	dispatcher := tg.NewDispatcher()
	dispatcher.OnCreateIssue = handlers.CreateIssue()
	dispatcher.OnGetIssue = handlers.GetIssue()
	dispatcher.OnCallback = handlers.Callback()
	dispatcher.OnReplyBotForComment = handlers.ReplyBotForComment()
	dispatcher.OnMediaGroup = handlers.MediaReplyBotForComment()

	b := tg.New(tgApi, logger, cfg, dispatcher, jiraClient)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := b.Run(ctx); err != nil {
		logger.Error("bot stopped", slog.Any("err", err))
	}
}
