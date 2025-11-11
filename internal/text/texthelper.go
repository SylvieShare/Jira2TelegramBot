package text

import (
	"fmt"
	"html"
	"strings"
	"time"
)

func EscapeMarkdownV2(text string) string {
	specialChars := []rune{'_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!'}
	escaped := text
	for _, ch := range specialChars {
		escaped = strings.ReplaceAll(escaped, string(ch), "\\"+string(ch))
	}
	return escaped
}

func CreatorWithSobakaInvis(text string) string {
	username := strings.TrimSpace(text)
	if username == "" {
		return ""
	}
	const zeroWidthBreak = "\u200B"
	return "@" + zeroWidthBreak + username
}

func IsReadyStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done", "closed", "resolved", "complete", "completed", "готов", "готово", "закрыт", "решена", "выполнена", "отменено":
		return true
	default:
		return false
	}
}

func GetStatusWithIcon(statusName string) string {
	if statusName == "" {
		return "Неизвестно"
	}
	switch strings.ToLower(statusName) {
	case "open", "открыт", "новая", "открыть", "to do", "к выполнению", "открыто повторно":
		return fmt.Sprintf("⚪ %s", statusName)
	case "in progress", "в работе", "выполняется":
		return fmt.Sprintf("🔵 %s", statusName)
	case "done", "закрыт", "решена", "выполнена", "готово":
		return fmt.Sprintf("🟢 %s", statusName)
	case "blocked", "отменено":
		return fmt.Sprintf("🔴 %s", statusName)
	case "in review", "на проверке":
		return fmt.Sprintf("🟣 %s", statusName)
	default:
		return statusName
	}
}

func GetPriorityWithIcon(priority string) string {
	switch strings.ToLower(priority) {
	case "highest", "наивысший":
		return fmt.Sprintf("🔴 %s", priority)
	case "high", "высокий":
		return fmt.Sprintf("🟠 %s", priority)
	case "medium", "средний":
		return fmt.Sprintf("🟡 %s", priority)
	case "low", "низкий":
		return fmt.Sprintf("🟢 %s", priority)
	case "lowest", "наинизший", "незначительный":
		return fmt.Sprintf("⚪ %s", priority)
	default:
		return priority
	}
}

func FormatDate(date time.Time) string {
	if date.IsZero() {
		return "Не указана"
	}
	return date.Format("02.01.2006 15:04")
}

func EscapeHTML(text string) string {
	return html.EscapeString(text)
}
