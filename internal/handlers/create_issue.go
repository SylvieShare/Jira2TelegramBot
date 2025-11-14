package handlers

import (
	"bytes"
	"net/http"
	"strings"

	"telegram-bot-jira/internal/common"
	"telegram-bot-jira/internal/text"
	"telegram-bot-jira/internal/tg"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// extractFilesFromHistory extracts all file URLs from the message history
func extractFilesFromHistory(ctx *tg.Ctx, messages []tgbotapi.Message) ([]common.FileInfo, error) {
	var allFiles []common.FileInfo
	
	for _, msg := range messages {
		files, err := common.ExtractFile(ctx.Bot, ctx.Log, &msg)
		if err != nil {
			ctx.Log.Warn("Failed to extract file from message", "error", err)
			continue
		}
		allFiles = append(allFiles, files...)
	}
	
	return allFiles, nil
}

func CreateIssue() tg.HandlerFunc {
	return func(c *tg.Ctx) error {
		messagesInHistory := c.HistoryMessages.GetMessages(c.Upd.Message.Chat.ID)
		titleIssue := text.TextTitleIssue(c.Upd.Message.Chat.Title)
		descriptionADF := text.TextDescriptionADF(titleIssue, messagesInHistory, "")
		key, _, err := c.Jira.CreateIssue(c.Std, titleIssue, descriptionADF)
		if err != nil {
			_, sendErr := c.Bot.Send(tgbotapi.NewMessage(c.Upd.Message.Chat.ID, text.TextErrorCreateTicket(err)))
			if sendErr != nil {
				return sendErr
			}
			return nil
		}
		c.Log.Info("Issue created", "key", key)

		// Extract and attach files from message history
		files, err := extractFilesFromHistory(c, messagesInHistory)
		if err != nil {
			c.Log.Error("Failed to extract files from history", "error", err)
		} else if len(files) > 0 {
			// Add files as attachments to the created issue
			for _, file := range files {
				if file.Url != "" {
					// Download the file
					resp, err := http.Get(file.Url)
					if err != nil {
						c.Log.Error("Failed to download file", "url", file.Url, "error", err)
						continue
					}
					defer resp.Body.Close()
					
					if resp.StatusCode != http.StatusOK {
						c.Log.Error("Failed to download file", "url", file.Url, "status", resp.StatusCode)
						continue
					}
					
					var buf bytes.Buffer
					_, err = buf.ReadFrom(resp.Body)
					if err != nil {
						c.Log.Error("Failed to read file data", "url", file.Url, "error", err)
						continue
					}
					
					// Determine filename
					filename := file.Name
					if filename == "" {
						filename = "telegram_attachment"
					}
					
					// Upload as attachment
					attachmentId, err := c.Jira.AddAttachment(c.Std, key, filename, buf.Bytes())
					if err != nil {
						c.Log.Error("Failed to upload attachment", "filename", filename, "error", err)
						continue
					}
					
					c.Log.Info("Successfully uploaded attachment", "filename", filename, "attachmentId", attachmentId)
				}
			}
		}

		payload := strings.TrimSpace(tg.StripCommandText(c.Upd.Message.Text))
		payload, _ = strings.CutPrefix(payload, "@"+c.Bot.Self.UserName)
		storeUsername := ""
		if c.Upd.Message.From != nil {
			storeUsername = c.Upd.Message.From.UserName
		}
		storeName := ""
		if payload != "" {
			fields := strings.Fields(payload)
			for i, f := range fields {
				if strings.HasPrefix(f, "@") && len(f) > 1 {
					storeUsername = strings.TrimPrefix(f, "@")
					fields = append(fields[:i], fields[i+1:]...)
					break
				}
			}
			storeName = strings.TrimSpace(strings.Join(fields, " "))
		}

		c.TicketStore.Add(c.Upd.Message.Chat.ID, key, "", storeName, storeUsername)

		return processGetIssue(c, key, c.Upd.Message.Chat.ID)
	}
}
