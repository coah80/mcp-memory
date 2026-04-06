package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type SessionManager struct {
	storage *Storage
	mu      sync.Mutex
	active  map[string]*Session
}

func NewSessionManager(storage *Storage) *SessionManager {
	return &SessionManager{
		storage: storage,
		active:  make(map[string]*Session),
	}
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if result == "" {
		return "unknown"
	}
	return result
}

func (sm *SessionManager) Start(tool string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	tool = sanitizeName(tool)
	id := fmt.Sprintf("%s-%d", tool, time.Now().UnixNano())
	session := &Session{
		ID:        id,
		Tool:      tool,
		StartedAt: time.Now().Format(time.RFC3339),
		Active:    true,
		Actions:   make([]SessionAction, 0),
	}

	sm.active[id] = session
	sm.save(session)

	return session
}

func (sm *SessionManager) LogAction(sessionID, actionType, target, details string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.active[sessionID]
	if !ok {
		return
	}

	session.Actions = append(session.Actions, SessionAction{
		Timestamp: time.Now().Format(time.RFC3339),
		Type:      actionType,
		Target:    target,
		Details:   details,
	})

	sm.save(session)
}

func (sm *SessionManager) End(sessionID string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.active[sessionID]
	if !ok {
		return nil
	}

	session.EndedAt = time.Now().Format(time.RFC3339)
	session.Active = false
	session.Summary = sm.buildSummary(session)

	sm.save(session)
	delete(sm.active, sessionID)

	return session
}

func (sm *SessionManager) Identify(sessionID, tool, model string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.active[sessionID]
	if !ok {
		return
	}

	if tool != "" {
		session.Tool = sanitizeName(tool)
	}
	if model != "" {
		session.Model = sanitizeName(model)
	}

	sm.save(session)
}

func (sm *SessionManager) buildSummary(session *Session) *SessionSummary {
	summary := &SessionSummary{
		Tool:        session.Tool,
		Model:       session.Model,
		ActionCount: len(session.Actions),
	}

	startTime, err1 := time.Parse(time.RFC3339, session.StartedAt)
	endTime, err2 := time.Parse(time.RFC3339, session.EndedAt)
	if err1 == nil && err2 == nil {
		summary.Duration = endTime.Sub(startTime).Round(time.Second).String()
	}

	created := make(map[string]bool)
	accessed := make(map[string]bool)
	queries := make(map[string]bool)
	allText := ""

	for _, action := range session.Actions {
		switch action.Type {
		case "memory_create", "create":
			created[action.Target] = true
		case "memory_get", "read", "memory_search", "search":
			accessed[action.Target] = true
		}

		if action.Type == "search" || action.Type == "memory_search" || action.Type == "similar" {
			queries[action.Target] = true
		}

		allText += action.Target + " " + action.Details + " "
	}

	for k := range created {
		summary.MemoriesCreated = append(summary.MemoriesCreated, k)
	}
	for k := range accessed {
		summary.MemoriesAccessed = append(summary.MemoriesAccessed, k)
	}
	for k := range queries {
		summary.SearchQueries = append(summary.SearchQueries, k)
	}

	summary.TopKeywords = ExtractKeywords(allText, 10)

	return summary
}

func (sm *SessionManager) save(session *Session) {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(sm.storage.SessionDir, session.ID+".json")
	os.WriteFile(path, data, 0600)
}

func (sm *SessionManager) GetSession(id string) (*Session, error) {
	sm.mu.Lock()
	if s, ok := sm.active[id]; ok {
		sm.mu.Unlock()
		return s, nil
	}
	sm.mu.Unlock()

	path := filepath.Join(sm.storage.SessionDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (sm *SessionManager) ListRecent(limit int) ([]*Session, error) {
	files, err := filepath.Glob(filepath.Join(sm.storage.SessionDir, "*.json"))
	if err != nil {
		return nil, err
	}

	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}

	sessions := make([]*Session, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func (sm *SessionManager) GetActiveSessions() []*Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessions := make([]*Session, 0, len(sm.active))
	for _, s := range sm.active {
		sessions = append(sessions, s)
	}
	return sessions
}

func (sm *SessionManager) RecentSummaryText(limit int) string {
	sessions, err := sm.ListRecent(limit)
	if err != nil || len(sessions) == 0 {
		return "No recent sessions."
	}

	var b strings.Builder
	for _, s := range sessions {
		toolInfo := s.Tool
		if s.Model != "" {
			toolInfo += " / " + s.Model
		}
		b.WriteString(fmt.Sprintf("## %s (%s)\n", s.ID, toolInfo))
		b.WriteString(fmt.Sprintf("Started: %s\n", s.StartedAt))
		if s.EndedAt != "" {
			b.WriteString(fmt.Sprintf("Ended: %s\n", s.EndedAt))
		}
		if s.Summary != nil {
			b.WriteString(fmt.Sprintf("Actions: %d, Duration: %s\n", s.Summary.ActionCount, s.Summary.Duration))
			if len(s.Summary.MemoriesCreated) > 0 {
				b.WriteString(fmt.Sprintf("Created: %s\n", strings.Join(s.Summary.MemoriesCreated, ", ")))
			}
			if len(s.Summary.MemoriesAccessed) > 0 {
				b.WriteString(fmt.Sprintf("Accessed: %s\n", strings.Join(s.Summary.MemoriesAccessed, ", ")))
			}
			if len(s.Summary.TopKeywords) > 0 {
				b.WriteString(fmt.Sprintf("Keywords: %s\n", strings.Join(s.Summary.TopKeywords, ", ")))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
