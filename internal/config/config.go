package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken       string
	LogLevel               string
	TelegramReactionEmoji  string
	UpdatesTimeout         int
	Workers                int
	JiraBaseURL            string
	JiraEmail              string
	JiraUserName           string
	JiraAPIToken           string
	JiraProjectKey         string
	JiraIssueType          string
	AggregateIssueKey      string
	JiraReopenStatus       string
	BotPollProcessInterval int
	HistoryMessagesLimit   int
	ClosedTicketTTLHours   int
}

func Load() Config {
	godotenv.Load()
	cfg := Config{
		TelegramBotToken:       os.Getenv("TELEGRAM_TOKEN"),
		LogLevel:               getenv("LOG_LEVEL", "info"),
		TelegramReactionEmoji:  getenv("TELEGRAM_REACTION_EMOJI", "👌"),
		UpdatesTimeout:         atoi(getenv("UPDATES_TIMEOUT", "60"), 60),
		Workers:                atoi(getenv("WORKERS", "4"), 4),
		JiraBaseURL:            getenv("JIRA_BASE_URL", ""),
		JiraEmail:              getenv("JIRA_EMAIL", ""),
		JiraUserName:           getenv("JIRA_USERNAME", ""),
		JiraAPIToken:           getenv("JIRA_API_TOKEN", ""),
		JiraProjectKey:         getenv("JIRA_PROJECT_KEY", ""),
		JiraIssueType:          getenv("JIRA_ISSUE_TYPE", "Task"),
		AggregateIssueKey:      getenv("AGGREGATE_ISSUE_KEY", ""),
		JiraReopenStatus:       strings.TrimSpace(getenv("JIRA_REOPEN_STATUS", "")),
		BotPollProcessInterval: atoi(getenv("POLL_INTERVAL_SECONDS", ""), 10),
		HistoryMessagesLimit:   atoi(getenv("HISTORY_MESSAGES_LIMIT", ""), 10),
		ClosedTicketTTLHours:   atoi(getenv("CLOSED_TICKET_TTL_HOURS", ""), 7*24),
	}
	if cfg.TelegramBotToken == "" {
		log.Fatal("TELEGRAM_TOKEN is required")
	}
	return cfg
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func atoi(s string, d int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return d
}
