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
	actionAddInfo = "addinfo"
)

func Callback() tg.HandlerFunc {
	return func(ctx *tg.Ctx) error {
		cb := ctx.Upd.CallbackQuery
		if cb == nil {
			return nil
		}
		_, _ = ctx.Bot.Request(tgbotapi.NewCallback(cb.ID, ""))

		data := strings.Split(cb.Data, "|")
		if len(data) == 0 {
			return nil
		}
		switch data[0] {
		case actionReopen:
			return handleReopenCallback(ctx, cb, data)
		case actionStatus:
			return handleStatusCallback(ctx, cb, data)
		case actionAddInfo:
			return handleAddInfoCallback(ctx, cb, data)
		default:
			return nil
		}
	}
}

func handleReopenCallback(ctx *tg.Ctx, cb *tgbotapi.CallbackQuery, parts []string) error {
	targetStatus := strings.TrimSpace(ctx.ReopenStatus)
	if targetStatus == "" || len(parts) < 2 {
		return nil
	}
	issueKey := strings.TrimSpace(parts[1])
	if issueKey == "" {
		return nil
	}

	chatID := cb.Message.Chat.ID
	chatTitle := cb.Message.Chat.Title
	if ctx.TicketStore.Get(issueKey) == nil {
		msg := tgbotapi.NewMessage(chatID, "⏳ Тикет <code>"+issueKey+"</code> слишком старый, его нельзя переоткрыть. Создайте новый через /create_issue.")
		msg.ParseMode = tgbotapi.ModeHTML
		_, _ = ctx.Bot.Send(msg)
		return nil
	}

	if err := ctx.Jira.TransitionIssueToStatus(ctx.Std, issueKey, targetStatus); err != nil {
		ctx.Log.Error("jira transition failed", "key", issueKey, "status", targetStatus, "err", err)
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Не удалось переоткрыть тикет %s: %v", issueKey, err))
		_, _ = ctx.Bot.Send(msg)
		return err
	}

	commentAuthor := text.BuildFullNameUser(cb.From)
	commentBody := text.TextJiraCommentReopen(commentAuthor, chatTitle)
	if err := ctx.Jira.AddComment(ctx.Std, issueKey, commentBody); err != nil {
		ctx.Log.Error("jira add comment failed", "key", issueKey, "err", err)
	}

	return processGetIssue(ctx, issueKey, chatID)
}

func handleStatusCallback(ctx *tg.Ctx, cb *tgbotapi.CallbackQuery, parts []string) error {
	if len(parts) < 2 {
		return nil
	}
	issueKey := strings.TrimSpace(parts[1])
	if issueKey == "" {
		return nil
	}

	chatID := cb.Message.Chat.ID
	return processGetIssue(ctx, issueKey, chatID)
}

func handleAddInfoCallback(ctx *tg.Ctx, cb *tgbotapi.CallbackQuery, parts []string) error {
	chatID := cb.Message.Chat.ID
	var issueKey string
	if len(parts) > 1 {
		issueKey = strings.TrimSpace(parts[1])
	}

	msgText := ""
	if issueKey != "" {
		msgText = fmt.Sprintf("%s\n%s",
			issueKey,
			text.TextAnchorReplyStatusToJira(),
		)
	}

	_, err := ctx.Bot.Send(tgbotapi.NewMessage(chatID, msgText))
	return err
}
