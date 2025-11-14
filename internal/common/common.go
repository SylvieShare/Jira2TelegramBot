package common

import (
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type FileInfo struct {
	Url  string
	Name string
}

// ExtractFile extracts file information from a single Telegram message
func ExtractFile(bot *tgbotapi.BotAPI, logger *slog.Logger, message *tgbotapi.Message) ([]FileInfo, error) {
	if message == nil {
		return nil, fmt.Errorf("message is nil")
	}
	var fileUrls []FileInfo

	// Handle photo messages
	if len(message.Photo) > 0 {
		// Get the largest photo (last element in the slice)
		photo := message.Photo[len(message.Photo)-1]

		fileID := photo.FileID
		fileURL, err := bot.GetFileDirectURL(fileID)
		if err != nil {
			return fileUrls, fmt.Errorf("failed to get photo URL: %w", err)
		}

		fileUrls = append(fileUrls, FileInfo{Url: fileURL})
		if logger != nil {
			logger.Info("Extracted photo URL", "url", fileURL)
		}
	}

	// Handle document messages (all types)
	if message.Document != nil {
		fileID := message.Document.FileID
		fileURL, err := bot.GetFileDirectURL(fileID)
		if err != nil {
			return fileUrls, fmt.Errorf("failed to get document URL: %w", err)
		}

		fileUrls = append(fileUrls, FileInfo{Url: fileURL, Name: message.Document.FileName})
		if logger != nil {
			logger.Info("Extracted document URL", "url", fileURL, "filename", message.Document.FileName, "mimeType", message.Document.MimeType)
		}
	}

	// Handle video messages
	if message.Video != nil {
		fileID := message.Video.FileID
		fileURL, err := bot.GetFileDirectURL(fileID)
		if err != nil {
			return fileUrls, fmt.Errorf("failed to get video URL: %w", err)
		}

		fileUrls = append(fileUrls, FileInfo{Url: fileURL, Name: message.Video.FileName})
		if logger != nil {
			logger.Info("Extracted video URL", "url", fileURL)
		}
	}

	// Handle audio messages
	if message.Audio != nil {
		fileID := message.Audio.FileID
		fileURL, err := bot.GetFileDirectURL(fileID)
		if err != nil {
			return fileUrls, fmt.Errorf("failed to get audio URL: %w", err)
		}

		fileUrls = append(fileUrls, FileInfo{Url: fileURL, Name: message.Audio.FileName})
		if logger != nil {
			logger.Info("Extracted audio URL", "url", fileURL)
		}
	}

	// Handle voice messages
	if message.Voice != nil {
		fileID := message.Voice.FileID
		fileURL, err := bot.GetFileDirectURL(fileID)
		if err != nil {
			return fileUrls, fmt.Errorf("failed to get voice URL: %w", err)
		}

		fileUrls = append(fileUrls, FileInfo{Url: fileURL})
		if logger != nil {
			logger.Info("Extracted voice URL", "url", fileURL)
		}
	}

	return fileUrls, nil
}

func IsBotMentioned(message *tgbotapi.Message, botUsername string) bool {
	if message == nil || message.Entities == nil || message.Text == "" || botUsername == "" {
		return false
	}

	target := "@" + botUsername
	for _, entity := range message.Entities {
		switch entity.Type {
		case "mention":
			if mention := substringRunes(message.Text, entity.Offset, entity.Length); mention == target {
				return true
			}
		}
	}
	return false
}

func substringRunes(text string, offset, length int) string {
	if offset < 0 || length <= 0 {
		return ""
	}
	runes := []rune(text)
	if offset >= len(runes) {
		return ""
	}
	end := offset + length
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[offset:end])
}