package text

import (
	"fmt"
	"strings"
	"time"

	"telegram-bot-jira/internal/jira"

	"telegram-bot-jira/internal/store"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ------------------ COMMON ------------------

// BuildFullNameUser строит отображаемое имя пользователя Telegram.
func BuildFullNameUser(user *tgbotapi.User) string {
	if user == nil {
		return "Unknown"
	}
	return user.FirstName + " " + user.LastName + " (@" + user.UserName + ")"
}

func TextAnchorReplyJiraToTelegram() string {
	return "Для ответа прикрепите это сообщение"
}

func TextAnchorReplyStatusToJira() string {
	return "‼️ Чтобы добавить информацию к заявке, ОБЯЗАТЕЛЬНО ответьте на это сообщение‼️"
}

// ------------------ TELEGRAM ------------------

// TextErrorCreateTicket возвращает человеко-понятное описание ошибки создания тикета.
func TextErrorCreateTicket(err error) string {
	return "Не удалось создать тикет"
}

// TextErrorCreateTicket возвращает человеко-понятное описание ошибки создания тикета.
func TextErrorCreateTicketDebug(err error) string {
	msg := "неизвестная ошибка"
	if err != nil {
		msg = err.Error()
	}
	return fmt.Sprintf("Не удалось создать тикет.\n\nДетали:\n`%s`", msg)
}

// TextTicketCreatedHTML сообщение о создании тикета (HTML).
func TextTicketCreatedHTML(title, issueKey, url string) string {
	return fmt.Sprintf(
		"🎉 <b>Задача успешно создана</b>\n\n"+
			"📚 <b>Название:</b> <code>%s</code>\n"+
			"🗝️ <b>Ключ:</b> <code>%s</code>\n"+
			"🔗 <b>Ссылка:</b> <a href=\"%s\">%s</a>",
		EscapeHTML(title),
		EscapeHTML(issueKey),
		url,
		EscapeHTML(url),
	)
}

// BuildUserMentionHTML формирует HTML-упоминание пользователя.
func BuildUserMentionHTML(userID int64, username, displayName string) string {
	if strings.TrimSpace(username) != "" {
		return "@" + EscapeHTML(username)
	}
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = "пользователь"
	}
	return fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", userID, EscapeHTML(name))
}

// TextTicketClosedHTML сообщение о закрытии тикета (HTML) с упоминанием автора.
func TextTicketClosedHTML(key, status, url, userCreator string) string {
	return fmt.Sprintf(
		"✅ <b>Тикет закрыт</b>\n\n"+
			"🗝️ <b>Ключ:</b> <code>%s</code>\n"+
			"📌 <b>Статус:</b> %s\n"+
			"🔗 <b>Ссылка:</b> <a href=\"%s\">%s</a>\n\n"+
			"@%s, тикет закрыт.",
		EscapeHTML(key),
		EscapeHTML(status),
		url,
		EscapeHTML(url),
		userCreator,
	)
}

// TextGetStatus выводит краткую информацию о тикете (HTML).
func TextGetStatus(issue *jira.IssueStatus, summary, author string) string {
	if summary == "" {
		summary = issue.Summary
	}
	status := issue.Status
	assignee := issue.Assignee
	if strings.TrimSpace(assignee) == "" {
		assignee = "Не назначен"
	}
	created := issue.Created
	updated := issue.Updated

	return fmt.Sprintf(
		"📚 <b>Название:</b> <code>%s</code>\n"+
			"🗝️ <b>Ключ:</b> <code>%s</code>\n\n"+
			"📌 <b>Статус:</b> %s\n"+
			"👤 <b>Ответственный:</b> %s\n\n"+
			"🕑 <b>Создан:</b> %s\n"+
			"♻️ <b>Обновлён:</b> %s\n\n"+
			"✍️ <b>Автор:</b> @%s\n\n"+
			"<b>%s</b>",
		EscapeHTML(summary),
		EscapeHTML(issue.Key),
		GetStatusWithIcon(status),
		EscapeHTML(assignee),
		FormatDate(created),
		FormatDate(updated),
		author,
		TextAnchorReplyStatusToJira(),
	)
}

func TextTelegramTicketsMessage(tickets []store.CreatedTicket, chatTitle string) string {
	var b strings.Builder
	b.WriteString("🗂 <b>Тикеты этого чата</b>")

	b.WriteString("\n\n")

	if len(tickets) == 0 {
		b.WriteString("В этом чате ещё не создано ни одного тикета.\n")
	} else {
		activeTickets := make([]store.CreatedTicket, 0, len(tickets))
		readyTickets := make([]store.CreatedTicket, 0)
		for _, ticket := range tickets {
			if IsReadyStatus(ticket.Status) {
				readyTickets = append(readyTickets, ticket)
				continue
			}
			activeTickets = append(activeTickets, ticket)
		}

		writeTicketLine := func(ticket store.CreatedTicket) {
			name := strings.TrimSpace(ticket.Name)
			if name == "" {
				name = TextTitleIssue("")
			}
			b.WriteString(fmt.Sprintf(
				"• <code>%s</code> — %s — %s\n",
				EscapeHTML(ticket.Key),
				EscapeHTML(name),
				EscapeHTML(GetStatusWithIcon(ticket.Status)),
			))
		}

		for _, ticket := range activeTickets {
			writeTicketLine(ticket)
		}

		if len(readyTickets) > 0 {
			if len(activeTickets) > 0 {
				b.WriteString("\n")
			}
			b.WriteString("<b>Тикеты в статусе «Готов»</b>\n")
			for _, ticket := range readyTickets {
				writeTicketLine(ticket)
			}
		}
	}

	b.WriteString("\nОтправьте <code>/status_issue TEC-123</code>, чтобы посмотреть детали конкретного тикета.")
	return b.String()
}

// TextGetStatusNotFound — если тикет не найден.
func TextGetStatusNotFound(issueKey string) string {
	return fmt.Sprintf("Тикет <code>%s</code> не найден", issueKey)
}

// TextTitleIssue — заголовок тикета по названию чата.
func TextTitleIssue(chatTitle string) string {
	if chatTitle != "" {
		return "Обращение из Telegram \"" + chatTitle + "\""
	}
	return "Обращение из Telegram"
}

func TextCommentJiraToTelegram(key, ticketAuthor, commentAuthor, text string) string {
	return fmt.Sprintf("📬 Комментарий по <code>%s</code>\n"+
		"👤 от %s для @%s\n\n"+
		"💬 <b>%s</b>\n\n"+
		"📣 %s",
		EscapeHTML(key),
		EscapeHTML(commentAuthor),
		EscapeHTML(ticketAuthor),
		EscapeHTML(text),
		TextAnchorReplyJiraToTelegram(),
	)
}

// ------------------ JIRA ------------------

// TextJiraCommentReopen — текст комментария о переоткрытии в Jira.
func TextJiraCommentReopen(userName, chatTitle string) string {
	if chatTitle != "" {
		return fmt.Sprintf("👤 Пользователь: %s в чате %s\nЗапросил переоткрытие.", userName, chatTitle)
	}
	return fmt.Sprintf("👤 Пользователь: %s запросил переоткрытие.", userName)

}

func TextJiraCommentUserFromTelegram(text string, user *tgbotapi.User, chatTitle, replyText string) string {
	replyClean := ""
	// replyStatus := false
	if strings.Contains(replyText, TextAnchorReplyJiraToTelegram()) {
		// replyStatus = true
		if replyText != "" {
			lines := strings.Split(replyText, "\n")
			if len(lines) > 5 {
				replyClean = strings.TrimSpace(strings.Join(lines[3:len(lines)-2], "\n"))
			} else {
				replyClean = ""
			}
		}
	}

	var b strings.Builder
	b.WriteString("💬 Сообщение из Telegram")
	if chatTitle != "" {
		b.WriteString(" (" + chatTitle + ")\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("👤 Автор: %s\n", BuildFullNameUser(user)))
	b.WriteString(text)

	if replyClean != "" {
		b.WriteString("\n\n🔁 Ответ на: ")
		b.WriteString(replyClean)
	}

	return b.String()
}

// TextDescriptionADF собирает ADF-документ Jira для описания задачи на основе истории чата.
func TextDescriptionADF(titleIssue string, historyMessages []tgbotapi.Message, urlChat string) map[string]any {
	doc := map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{},
	}
	appendBlock := func(b any) { doc["content"] = append(doc["content"].([]any), b) }

	if titleIssue != "" {
		appendBlock(map[string]any{
			"type":    "heading",
			"attrs":   map[string]any{"level": 3},
			"content": []any{map[string]any{"type": "text", "text": "Тема: " + titleIssue}},
		})
	}

	if len(historyMessages) == 0 {
		appendBlock(map[string]any{
			"type":    "paragraph",
			"content": []any{map[string]any{"type": "text", "text": "История сообщений пуста"}},
		})
		return doc
	}

	chat := historyMessages[0].Chat
	chatTitle := strings.TrimSpace(chat.Title)
	headingText := "Чат из переписки в Telegram"
	if chatTitle != "" {
		headingText += ": " + chatTitle
	}
	appendBlock(map[string]any{
		"type":    "heading",
		"attrs":   map[string]any{"level": 3},
		"content": []any{map[string]any{"type": "text", "text": headingText}},
	})
	if strings.TrimSpace(urlChat) != "" && chatTitle != "" {
		appendBlock(map[string]any{
			"type": "paragraph",
			"content": []any{
				map[string]any{
					"type":  "text",
					"text":  chatTitle,
					"marks": []any{map[string]any{"type": "link", "attrs": map[string]any{"href": urlChat}}},
				},
			},
		})
	}

	for _, m := range historyMessages {
		if m.Text == "" {
			continue
		}
		ts := int64(m.Date)
		dateTime := time.Unix(ts, 0).In(time.Local).Format("02.01.06 15:04")
		user := BuildFullNameUser(m.From)
		appendBlock(map[string]any{
			"type":    "paragraph",
			"content": []any{map[string]any{"type": "text", "text": dateTime + " — " + user + ":", "marks": []any{map[string]any{"type": "strong"}}}},
		})
		appendBlock(map[string]any{
			"type":  "panel",
			"attrs": map[string]any{"panelType": "info"},
			"content": []any{map[string]any{
				"type":    "paragraph",
				"content": []any{map[string]any{"type": "text", "text": m.Text}},
			}},
		})
	}
	appendBlock(map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Сформировано автоматически из переписки Telegram", "marks": []any{map[string]any{"type": "em"}}}}})
	return doc
}
