package tg

import "telegram-bot-jira/internal/store"

type TicketStore = store.TicketStore
type CreatedTicket = store.CreatedTicket

func NewTicketStore() *TicketStore {
	return store.NewTicketStore()
}
