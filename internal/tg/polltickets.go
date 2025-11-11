package tg

import (
	"context"
	"strings"
	"telegram-bot-jira/internal/jira"
	"telegram-bot-jira/internal/text"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) pollTickets(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(b.cfg.BotPollProcessInterval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tickets := b.ticketStore.ListAll()
			for i := range tickets {
				ticket := &tickets[i]
				processComments(ctx, b, ticket)
				processCheckStatus(b, ctx, ticket)
			}
			// periodic sync of aggregate issue only if data changed
			if b.ticketStore.DirtyAndReset() {
				b.updateAggregateToJira(ctx)
			}
		}
	}
}

func processComments(ctx context.Context, b *Bot, ticket *CreatedTicket) {
	if comments, err := b.jira.GetComments(ctx, ticket.Key); err != nil {
		b.log.Error("Failed get issue comments", "key", ticket.Key, "error", err)
	} else {
		newLastCommentAt := ticket.LastCommentAt
		for i := range comments {
			comment := &comments[i]
			if comment.Created.Time.After(ticket.LastCommentAt) {
				processComment(ctx, b, ticket, comment)
				if comment.Created.Time.After(newLastCommentAt) {
					newLastCommentAt = comment.Created.Time
				}
			}
		}
		if ticket.LastCommentAt != newLastCommentAt {
			newLastCommentAt = newLastCommentAt.Add(time.Second)
			b.ticketStore.UpdateLastCommentAt(ticket.Key, newLastCommentAt)
		}
	}
}

func processComment(ctx context.Context, b *Bot, ticket *CreatedTicket, comment *jira.Comment) {
	targetUserName := b.cfg.JiraUserName

	textComment, hasPrefix := strings.CutPrefix(comment.Body.Text, "/tg")
	if !(hasPrefix || targetUserName != "" && strings.Contains(comment.RenderedBody, targetUserName)) {
		return
	}
	textComment = strings.TrimSpace(textComment)

	author := strings.TrimSpace(comment.Author.DisplayName)
	if author == "" {
		author = strings.TrimSpace(comment.Author.Email)
	}

	msgText := text.TextCommentJiraToTelegram(ticket.Key, ticket.CreatorUsername, author, textComment)
	msg := tgbotapi.NewMessage(ticket.ChatID, msgText)
	msg.ParseMode = tgbotapi.ModeHTML
	if _, err := b.api.Send(msg); err != nil {
		b.log.Error("Failed notify mention", "key", ticket.Key, "error", err)
		return
	}

	// if err := b.jira.AddCommentReaction(ctx, ticket.Key, comment.ID, "white_check_mark"); err != nil {
	// 	b.log.Error("Failed to react to Jira comment", "key", ticket.Key, "comment_id", comment.ID, "error", err)
	// }
}

func processCheckStatus(b *Bot, ctx context.Context, ticket *CreatedTicket) {
	ticketActual, err := b.jira.GetIssueStatus(ctx, ticket.Key)
	if err != nil {
		b.log.Info("Failed get issue status", "key", ticket.Key)
		return
	}
	if text.IsReadyStatus(ticketActual.Status) {
		retentionHours := b.cfg.ClosedTicketTTLHours
		if retentionHours <= 0 {
			retentionHours = 3 * 24
		}
		expireBefore := time.Now().Add(-time.Duration(retentionHours) * time.Hour)
		if ticketActual.Updated.Before(expireBefore) {
			b.log.Info("Removing stale closed ticket", "key", ticketActual.Key, "updated", ticketActual.Updated)
			b.ticketStore.Delete(ticket.Key)
			return
		}
	}
	if ticketActual.Status == ticket.Status {
		return
	}
	b.log.Info("Issue status updated", "key", ticketActual.Key, "status", ticketActual.Status)
	ticket.Status = ticketActual.Status
	checkTicketIsClosing(b, ticket)
	b.ticketStore.UpdateStatus(ticket.Key, ticketActual.Status)
}

func checkTicketIsClosing(b *Bot, ticket *CreatedTicket) {
	if text.IsReadyStatus(ticket.Status) {
		url := b.jira.BrowseURL(ticket.Key)
		txt := text.TextTicketClosedHTML(ticket.Key, ticket.Status, url, ticket.CreatorUsername)
		msg := tgbotapi.NewMessage(ticket.ChatID, txt)
		if b.cfg.JiraReopenStatus != "" {
			callbackData := "reopen|" + ticket.Key
			button := tgbotapi.NewInlineKeyboardButtonData("Переоткрыть", callbackData)
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(button))
		}
		msg.ParseMode = tgbotapi.ModeHTML
		_, _ = b.api.Send(msg)
	}
}
