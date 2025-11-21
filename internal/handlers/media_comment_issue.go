package handlers

import (
	"strings"
	"sync"
	"time"

	"telegram-bot-jira/internal/text"
	"telegram-bot-jira/internal/tg"
	"telegram-bot-jira/internal/common"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MediaGroup represents a group of messages from the same media group
type MediaGroup struct {
	Messages  []*tgbotapi.Message
	Timer     *time.Timer
	Processed bool
}

func MediaReplyBotForComment() tg.HandlerFunc {
	mediaGroups := make(map[string]*MediaGroup)
	var mu sync.Mutex
	return func(ctx *tg.Ctx) error {
		message := ctx.Upd.Message
		groupID := message.MediaGroupID

		if groupID == "" {
			processMediaGroup(ctx, &MediaGroup{
				Messages: []*tgbotapi.Message{message},
			})
			return nil
		}

		mu.Lock()
		defer mu.Unlock()
		var group *MediaGroup
		// Check if we already have this group
		var exists bool
		if group, exists = mediaGroups[groupID]; exists {
			// Add message to existing group
			group.Messages = append(group.Messages, message)

			// Reset the timer to give more time for additional messages
			if group.Timer != nil {
				group.Timer.Stop()
			}
		} else {
			// Create new media group
			group = &MediaGroup{
				Messages:  []*tgbotapi.Message{message},
				Processed: false,
			}

			mediaGroups[groupID] = group
		}

		// Set a timer to process the group after a short delay
		group.Timer = time.AfterFunc(3000*time.Millisecond, func() {
			mu.Lock()
			group, exists := mediaGroups[groupID]
			if !exists || group.Processed {
				mu.Unlock()
				return
			}
			group.Processed = true
			if group.Timer != nil {
				group.Timer.Stop()
			}
			delete(mediaGroups, groupID)
			mu.Unlock()
			processMediaGroup(ctx, group)
		})

		return nil
	}
}

func processMediaGroup(ctx *tg.Ctx, group *MediaGroup) {
	// Process multiple messages as a group
	// Find the first message that has a reply to the bot
	var messageWithReplay *tgbotapi.Message
	var allTexts []string
	var allFiles []common.FileInfo

	for _, msg := range group.Messages {
		// Collect text from all messages
		if msg.Text != "" {
			allTexts = append(allTexts, msg.Text)
		} else if msg.Caption != "" {
			allTexts = append(allTexts, msg.Caption)
		}

		// Collect file URLs from all messages
		files, err := ctx.Tg.ExtractFile(msg)
		if err == nil {
			allFiles = append(allFiles, files...)
		}

		// Find the message that replies to the bot
		if msg.ReplyToMessage != nil && msg.ReplyToMessage.From.UserName == ctx.Tg.SelfUserName() {
			messageWithReplay = msg
		}
	}

	// If we found a reply message, process the group
	if messageWithReplay != nil {
		// Combine all texts
		combinedText := strings.Join(allTexts, ", ")
		replyText := messageWithReplay.ReplyToMessage.Text
		key := ctx.Params.ProjectKeyRegexp.FindString(replyText)
		if key == "" {
			ctx.Log.Error("Failed to find project key in reply text", "replyText", replyText)
			return
		}
		commentErr := ctx.Jira.AddCommentWithEmbeddedFiles(ctx.Std,
			key,
			text.TextJiraCommentUserFromTelegram(combinedText, messageWithReplay.From, messageWithReplay.Chat.Title, replyText),
			allFiles)
		if commentErr != nil {
			ctx.Log.Error("Failed to add comment for media group", "error", commentErr)
		} else {
			ctx.Tg.ReactMessageIsRead(messageWithReplay)
		}
	}
}
