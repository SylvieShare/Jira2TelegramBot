package tg

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// updateAggregateToJira rebuilds the ADF table and updates the aggregate issue description.
func (b *Bot) updateAggregateToJira(ctx context.Context) {
	if b.cfg.AggregateIssueKey == "" {
		return
	}
	// Filter: include open and closed updated within a week
	all := b.ticketStore.ListAll()

	// Build table with Russian headers and proper UTF-8
	header := []string{"Ключ", "Статус", "Название", "Чат ID", "Автор", "Последний коммент"}
	rows := make([][]string, 0, len(all))
	for _, ticket := range all {
		// link := b.jira.BrowseURL(ticket.Key)
		lastCommentAt := ""
		if !ticket.LastCommentAt.IsZero() {
			lastCommentAt = ticket.LastCommentAt.Format("02.01.2006 15:04:05")
		}
		rows = append(rows, []string{ticket.Key, ticket.Status, ticket.Name, int64ToString(ticket.ChatID), ticket.CreatorUsername, lastCommentAt})
	}
	// Make column 4 (index) clickable link
	doc := buildADFTable(header, rows, -1)

	err := b.jira.UpdateIssueDescriptionADF(ctx, b.cfg.AggregateIssueKey, doc)
	if err != nil {
		b.log.Error("Error update jira context ticket", slog.String("key", b.cfg.AggregateIssueKey), slog.Any("err", err))
	} else {
		b.log.Info("Update jira context ticket", slog.String("key", b.cfg.AggregateIssueKey), slog.Int("size", len(all)))
	}
}

// syncAggregateFromJira fetches aggregate issue description and populates ticket store.
func (b *Bot) syncAggregateFromJira(ctx context.Context) error {
	doc, err := b.jira.GetIssueDescriptionADF(ctx, b.cfg.AggregateIssueKey)
	if err != nil || doc == nil {
		return err
	}
	tickets := parseAggregateADF(doc)
	b.ticketStore.Init(tickets)
	b.log.Info("Load jira context issue", slog.String("key", b.cfg.AggregateIssueKey), slog.Int("size", len(tickets)))
	return nil
}

func buildTableRow(cells []string, header bool) any {
	cellType := "tableCell"
	if header {
		cellType = "tableHeader"
	}
	rowCells := []any{}
	for _, c := range cells {
		rowCells = append(rowCells, map[string]any{
			"type":    cellType,
			"attrs":   map[string]any{},
			"content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": c}}}},
		})
	}
	return map[string]any{"type": "tableRow", "content": rowCells}
}

func int64ToString(v int64) string { return fmt.Sprintf("%d", v) }

// parseAggregateADF reads tickets from a simple ADF table as produced above.
func parseAggregateADF(doc map[string]any) []CreatedTicket {
	var out []CreatedTicket
	if doc == nil {
		return out
	}
	// Find first table node and read its rows (first row is header)
	content, _ := doc["content"].([]any)
	for _, node := range content {
		m, _ := node.(map[string]any)
		if m["type"] != "table" {
			continue
		}
		rows, _ := m["content"].([]any)
		for i, r := range rows {
			if i == 0 { // header row
				continue
			}
			rm, _ := r.(map[string]any)
			if rm["type"] != "tableRow" {
				continue
			}
			cells, _ := rm["content"].([]any)
			if len(cells) < 4 {
				continue
			}
			val := func(i int) string {
				if len(cells) <= i {
					return ""
				}
				cm, _ := cells[i].(map[string]any)
				parr, _ := cm["content"].([]any)
				if len(parr) == 0 {
					return ""
				}
				pm0, _ := parr[0].(map[string]any)
				carr, _ := pm0["content"].([]any)
				if len(carr) == 0 {
					return ""
				}
				tm, _ := carr[0].(map[string]any)
				s, _ := tm["text"].(string)
				return s
			}
			chatID := parseInt64(val(3))
			lastCommentAt := parseLocalTime(val(5))
			out = append(out, CreatedTicket{
				Key:             val(0),
				Status:          val(1),
				Name:            val(2),
				ChatID:          chatID,
				CreatorUsername: val(4),
				LastCommentAt:   lastCommentAt,
			})
		}
		break
	}
	return out
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func parseLocalTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.ParseInLocation("02.01.2006 15:04:05", s, time.Local); err == nil {
		return t
	}
	t, _ := time.Parse("02.01.2006 15:04:05", s)
	return t
}

// buildADFTable builds ADF table; linkCol is index of column to render as clickable link (-1 if none).
func buildADFTable(header []string, rows [][]string, linkCol int) map[string]any {
	doc := map[string]any{"type": "doc", "version": 1, "content": []any{}}
	appendBlock := func(b any) { doc["content"] = append(doc["content"].([]any), b) }
	// Heading
	appendBlock(map[string]any{
		"type":    "heading",
		"attrs":   map[string]any{"level": 2},
		"content": []any{map[string]any{"type": "text", "text": "Список тикетов бота"}},
	})
	// Table rows
	tableRows := []any{buildTableRow(header, true)}
	for _, r := range rows {
		// Build body row with optional link marks on linkCol
		cells := []any{}
		for i, c := range r {
			textNode := map[string]any{"type": "text", "text": c}
			if i == linkCol && strings.TrimSpace(c) != "" {
				textNode["marks"] = []any{map[string]any{"type": "link", "attrs": map[string]any{"href": c}}}
			}
			cell := map[string]any{
				"type":    "tableCell",
				"attrs":   map[string]any{},
				"content": []any{map[string]any{"type": "paragraph", "content": []any{textNode}}},
			}
			cells = append(cells, cell)
		}
		tableRows = append(tableRows, map[string]any{"type": "tableRow", "content": cells})
	}
	table := map[string]any{"type": "table", "attrs": map[string]any{}, "content": tableRows}
	appendBlock(table)
	return doc
}
