package handlers

import (
	"fmt"
	"strings"

	"telegram-bot-jira/internal/text"
	"telegram-bot-jira/internal/tg"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	actionReopen  = "reopen"
	actionStatus  = "status"
)

func Callback() tg.HandlerFunc {
	return func(ctx *tg.Ctx) error {
		cb := ctx.Upd.CallbackQuery
		if cb == nil {
			return nil
		}
		_ = ctx.Tg.EmptyCallback()

		data := strings.Split(cb.Data, "|")
		if len(data) == 0 {
			return nil
		}
		switch data[0] {
		case actionReopen:
			return handleReopenCallback(ctx, cb, data)
		case actionStatus:
			return handleStatusCallback(ctx, cb, data)
		default:
			return nil
		}
	}
}

func handleReopenCallback(ctx *tg.Ctx, cb *tgbotapi.CallbackQuery, parts []string) error {
	targetStatus := strings.TrimSpace(ctx.Params.ReopenStatus)
	if targetStatus == "" || len(parts) < 2 {
		return nil
	}
	issueKey := strings.TrimSpace(parts[1])
	if issueKey == "" {
		return nil
	}

	chatTitle := cb.Message.Chat.Title
	if ctx.TicketStore.Get(issueKey) == nil {
		return ctx.Tg.SendMessageHTML("⏳ Тикет <code>"+issueKey+"</code> слишком старый, его нельзя переоткрыть. Создайте новый через /create_issue.")
	}

	if err := ctx.Jira.TransitionIssueToStatus(ctx.Std, issueKey, targetStatus); err != nil {
		ctx.Log.Error("jira transition failed", "key", issueKey, "status", targetStatus, "err", err)
		_ = ctx.Tg.SendMessage(fmt.Sprintf("Не удалось переоткрыть тикет %s: %v", issueKey, err))
		return err
	}

	commentAuthor := text.BuildFullNameUser(cb.From)
	commentBody := text.TextJiraCommentReopen(commentAuthor, chatTitle)
	if err := ctx.Jira.AddComment(ctx.Std, issueKey, commentBody); err != nil {
		ctx.Log.Error("jira add comment failed", "key", issueKey, "err", err)
	}

	return processGetIssue(ctx, issueKey)
}

func handleStatusCallback(ctx *tg.Ctx, cb *tgbotapi.CallbackQuery, parts []string) error {
	if len(parts) < 2 {
		return nil
	}
	issueKey := strings.TrimSpace(parts[1])
	if issueKey == "" {
		return nil
	}

	return processGetIssue(ctx, issueKey)
}