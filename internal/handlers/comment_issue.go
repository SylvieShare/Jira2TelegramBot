package handlers

import (
	"telegram-bot-jira/internal/text"
	"telegram-bot-jira/internal/tg"

)

func ReplyBotForComment() tg.HandlerFunc {
	return func(ctx *tg.Ctx) error {
		message := ctx.Upd.Message
		keyInMessageReply := ctx.ProjectKeyRegexp.FindString(message.ReplyToMessage.Text)
		if keyInMessageReply != "" {
			commentText := text.TextJiraCommentUserFromTelegram(message.Text, message.From, message.Chat.Title, message.ReplyToMessage.Text)
			commentErr := ctx.Jira.AddComment(ctx.Std, keyInMessageReply, commentText)

			if commentErr != nil {
				ctx.Log.Error("Failed to add comment", "error", commentErr)
				return commentErr
			}

			ctx.Log.Info("Comment added successfully")
			ctx.ReactToMessage(message)
			return nil
		}
		return nil
	}
}
