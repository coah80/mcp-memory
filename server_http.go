package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type HTTPServer struct {
	storage  *Storage
	index    *Index
	journal  *Journal
	sessions *SessionManager
	access   *AccessTracker
	port     string
}

func NewHTTPServer(port string, storage *Storage, index *Index, journal *Journal, sessions *SessionManager, access *AccessTracker) *HTTPServer {
	return &HTTPServer{
		storage:  storage,
		index:    index,
		journal:  journal,
		sessions: sessions,
		access:   access,
		port:     port,
	}
}

func (h *HTTPServer) Run() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", h.handleHealth)

	mux.HandleFunc("/memories", h.handleListMemories)
	mux.HandleFunc("/memories/search", h.handleSearch)
	mux.HandleFunc("/memories/similar", h.handleSimilar)
	mux.HandleFunc("/memories/create", h.handleCreateMemory)
	mux.HandleFunc("/memories/autolog", h.handleAutoLog)
	mux.HandleFunc("/memories/index", h.handleIndex)
	mux.HandleFunc("/memories/rebuild", h.handleRebuildIndex)
	mux.HandleFunc("/memories/", h.handleMemory)

	mux.HandleFunc("/soul", h.handleSoul)
	mux.HandleFunc("/user", h.handleUser)

	mux.HandleFunc("/journal", h.handleJournal)

	mux.HandleFunc("/sessions", h.handleSessions)
	mux.HandleFunc("/sessions/start", h.handleSessionStart)
	mux.HandleFunc("/sessions/end", h.handleSessionEnd)
	mux.HandleFunc("/sessions/log", h.handleSessionLog)
	mux.HandleFunc("/sessions/identify", h.handleSessionIdentify)
	mux.HandleFunc("/sessions/", h.handleSessionGet)

	mux.HandleFunc("/rules", h.handleRules)
	mux.HandleFunc("/setup", h.handleSetup)
	mux.HandleFunc("/access", h.handleAccessStats)

	log.Printf("MCP Memory Server starting on :%s", h.port)
	log.Printf("Memory directory: %s", h.storage.MemoryDir)

	return http.ListenAndServe(":"+h.port, mux)
}

func (h *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "mcp-memory",
		"version": "2.0.0",
	})
}

func (h *HTTPServer) handleListMemories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.journal.Log("list", "memories", "Listed all memories")
	memories, err := h.storage.ListMemories()
	if err != nil {
		http.Error(w, "Failed to list memories", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, memories)
}

func (h *HTTPServer) handleMemory(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/memories/")
	if name == "" {
		http.Error(w, "Memory name required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		mem, err := h.storage.GetMemory(name)
		if err != nil {
			http.Error(w, "Memory not found", http.StatusNotFound)
			return
		}
		h.access.RecordAccess(mem.Name, r.Header.Get("X-Tool"))
		h.journal.Log("read", name, "Read memory")
		writeJSON(w, http.StatusOK, mem)

	case http.MethodDelete:
		if err := h.storage.DeleteMemory(name); err != nil {
			http.Error(w, "Memory not found", http.StatusNotFound)
			return
		}
		h.access.RemoveMemory(name)
		h.journal.Log("delete", name, "Deleted memory")
		go h.index.Build()
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *HTTPServer) handleCreateMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var mem Memory
	if err := json.NewDecoder(r.Body).Decode(&mem); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if mem.Name == "" {
		http.Error(w, "Memory name required", http.StatusBadRequest)
		return
	}

	if err := h.storage.CreateMemory(&mem); err != nil {
		http.Error(w, "Failed to write memory", http.StatusInternalServerError)
		return
	}

	h.access.RecordCreation(mem.Name)
	h.journal.Log("create", mem.Name, mem.Description)
	go h.index.Build()

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created", "name": mem.Name})
}

func (h *HTTPServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' required", http.StatusBadRequest)
		return
	}

	results := h.index.Search(query)
	for _, mem := range results {
		h.access.RecordSearchHit(mem.Name, query)
	}
	h.journal.Log("search", query, fmt.Sprintf("Found %d results", len(results)))

	writeJSON(w, http.StatusOK, results)
}

func (h *HTTPServer) handleSimilar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' required", http.StatusBadRequest)
		return
	}

	threshold := 0.3
	if t := r.URL.Query().Get("threshold"); t != "" {
		fmt.Sscanf(t, "%f", &threshold)
	}

	h.journal.Log("similar", query, "Checked for similar memories")
	results := h.index.FindSimilar(query, threshold)

	writeJSON(w, http.StatusOK, results)
}

func (h *HTTPServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.index.Load(); err != nil {
		http.Error(w, "Failed to load index", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, h.index.GetCache())
}

func (h *HTTPServer) handleRebuildIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.index.Build(); err != nil {
		http.Error(w, "Failed to rebuild index", http.StatusInternalServerError)
		return
	}
	h.journal.Log("rebuild", "index", "Rebuilt memory index")
	writeJSON(w, http.StatusOK, map[string]string{"status": "rebuilt"})
}

func (h *HTTPServer) handleAutoLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Task   string `json:"task"`
		Result string `json:"result"`
		SaveAs string `json:"save_as,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.journal.Log("complete", req.Task, req.Result)

	if req.SaveAs != "" {
		mem := &Memory{
			Name:        req.SaveAs,
			Description: fmt.Sprintf("Auto-saved: %s", req.Task),
			Type:        "auto",
			Content:     fmt.Sprintf("## Task\n%s\n\n## Result\n%s\n", req.Task, req.Result),
		}
		h.storage.CreateMemory(mem)
		h.access.RecordCreation(mem.Name)
		go h.index.Build()
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged"})
}

func (h *HTTPServer) handleSoul(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		content, err := h.storage.ReadSpecialFile(h.storage.SoulFile)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]string{"name": "SOUL", "content": "# Soul\n\nDefine your persistent persona here.\n"})
			return
		}
		h.journal.Log("read", "SOUL.md", "Read soul file")
		writeJSON(w, http.StatusOK, map[string]string{"name": "SOUL", "content": content})

	case http.MethodPost:
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		full := "---\ndescription: Persistent persona and identity\ntype: soul\n---\n\n" + req.Content
		if err := h.storage.WriteSpecialFile(h.storage.SoulFile, full); err != nil {
			http.Error(w, "Failed to write soul", http.StatusInternalServerError)
			return
		}
		h.journal.Log("update", "SOUL.md", "Updated soul file")
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *HTTPServer) handleUser(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		content, err := h.storage.ReadSpecialFile(h.storage.UserFile)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]string{"name": "USER", "content": "# User\n\nDefine your context here.\n"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"name": "USER", "content": content})

	case http.MethodPost:
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		full := "---\ndescription: Context about the human user\ntype: user\n---\n\n" + req.Content
		if err := h.storage.WriteSpecialFile(h.storage.UserFile, full); err != nil {
			http.Error(w, "Failed to write user", http.StatusInternalServerError)
			return
		}
		h.journal.Log("update", "USER.md", "Updated user context")
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *HTTPServer) handleJournal(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		date := r.URL.Query().Get("date")
		content, err := h.journal.Read(date)
		if err != nil {
			http.Error(w, "Failed to read journal", http.StatusInternalServerError)
			return
		}
		if date == "" {
			date = "today"
		}
		writeJSON(w, http.StatusOK, map[string]string{"date": date, "content": content})

	case http.MethodPost:
		var req struct {
			Entry string `json:"entry"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		h.journal.Log("note", "journal", req.Entry)
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *HTTPServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessions, err := h.sessions.ListRecent(20)
	if err != nil {
		http.Error(w, "Failed to list sessions", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *HTTPServer) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Tool string `json:"tool"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Tool = "http"
	}
	if req.Tool == "" {
		req.Tool = "http"
	}

	session := h.sessions.Start(req.Tool)
	h.journal.Log("session_start", session.ID, req.Tool)

	writeJSON(w, http.StatusCreated, session)
}

func (h *HTTPServer) handleSessionEnd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	session := h.sessions.End(req.SessionID)
	if session == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	h.journal.Log("session_end", session.ID, fmt.Sprintf("%d actions", len(session.Actions)))
	writeJSON(w, http.StatusOK, session)
}

func (h *HTTPServer) handleSessionLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Action    string `json:"action"`
		Target    string `json:"target"`
		Details   string `json:"details"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.sessions.LogAction(req.SessionID, req.Action, req.Target, req.Details)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged"})
}

func (h *HTTPServer) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/sessions/")
	session, err := h.sessions.GetSession(id)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *HTTPServer) handleSessionIdentify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Tool      string `json:"tool"`
		Model     string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	h.sessions.Identify(req.SessionID, req.Tool, req.Model)
	writeJSON(w, http.StatusOK, map[string]string{"status": "identified"})
}

func (h *HTTPServer) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/markdown")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, embeddedRules)
}

func (h *HTTPServer) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/markdown")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, embeddedSetup)
}

func (h *HTTPServer) handleAccessStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.access.GetStats())
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
