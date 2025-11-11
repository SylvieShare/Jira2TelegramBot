package store

import (
	"sync"
	"time"
)

type CreatedTicket struct {
	Key             string
	Name            string
	Status          string
	ChatID          int64
	CreatorUsername string
	LastCommentAt   time.Time
}

type TicketStore struct {
	mu    sync.RWMutex
	byKey map[string]CreatedTicket
	dirty bool
}

func (s *TicketStore) Has(key string) {
	panic("unimplemented")
}

func NewTicketStore() *TicketStore {
	return &TicketStore{byKey: make(map[string]CreatedTicket), dirty: false}
}

func (s *TicketStore) Add(chatID int64, key, status, name, username string) {
	if s == nil || key == "" {
		return
	}
	s.mu.Lock()
	ticket := s.byKey[key]
	ticket.Key = key
	ticket.Name = name
	ticket.Status = status
	ticket.ChatID = chatID
	ticket.CreatorUsername = username
	s.byKey[key] = ticket
	s.mu.Unlock()
}

func (s *TicketStore) Delete(key string) {
	if s == nil || key == "" {
		return
	}
	s.mu.Lock()
	if _, ok := s.byKey[key]; ok {
		delete(s.byKey, key)
		s.dirty = true
	}
	s.mu.Unlock()
}

func (s *TicketStore) Get(key string) *CreatedTicket {
	if s == nil || key == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	ticket, b := s.byKey[key]
	if b {
		return &ticket
	}
	return nil
}

func (s *TicketStore) Init(tickets []CreatedTicket) {
	if s == nil || tickets == nil {
		return
	}
	s.mu.Lock()
	for _, ticket := range tickets {
		s.byKey[ticket.Key] = ticket
	}
	s.mu.Unlock()
}

// ListAll returns a snapshot of all tickets across all chats.
func (s *TicketStore) ListAll() []CreatedTicket {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]CreatedTicket, 0, len(s.byKey))
	for _, ticket := range s.byKey {
		out = append(out, ticket)
	}
	return out
}

// ListByChatID returns tickets that belong to the provided chat.
func (s *TicketStore) ListByChatID(chatID int64) []CreatedTicket {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]CreatedTicket, 0, len(s.byKey))
	for _, ticket := range s.byKey {
		if ticket.ChatID == chatID {
			out = append(out, ticket)
		}
	}
	return out
}

func (s *TicketStore) AddOrUpdate(ticket *CreatedTicket) {
	if s == nil || ticket == nil {
		return
	}
	s.mu.Lock()
	s.byKey[ticket.Key] = *ticket
	s.mu.Unlock()
}

func (s *TicketStore) UpdateLastCommentAt(key string, lastCommentAt time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ticket, b := s.byKey[key]
	if !b {
		return
	}
	if !lastCommentAt.IsZero() && !lastCommentAt.Equal(ticket.LastCommentAt) {
		ticket.LastCommentAt = lastCommentAt
		s.dirty = true
		s.byKey[key] = ticket
	}
}

func (s *TicketStore) UpdateStatus(key, status string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ticket, b := s.byKey[key]
	if !b {
		return
	}
	changed := false
	if status != "" && ticket.Status != status {
		ticket.Status = status
		changed = true
	}
	s.byKey[key] = ticket
	if changed {
		s.dirty = true
	}
}

// DirtyAndReset atomically returns dirty and resets it to false.
func (s *TicketStore) DirtyAndReset() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	d := s.dirty
	s.dirty = false
	s.mu.Unlock()
	return d
}
