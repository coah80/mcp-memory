package main

import (
	"encoding/json"
)

type Memory struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	ModTime     string `json:"mod_time"`
}

type MemoryIndex struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Keywords    []string `json:"keywords"`
	ModTime     string   `json:"mod_time"`
	Summary     string   `json:"summary"`
}

type SimilarMemory struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Summary     string  `json:"summary"`
	Score       float64 `json:"score"`
}

type JournalEntry struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Details   string `json:"details,omitempty"`
}

type Session struct {
	ID        string          `json:"id"`
	Tool      string          `json:"tool"`
	Model     string          `json:"model,omitempty"`
	StartedAt string          `json:"started_at"`
	EndedAt   string          `json:"ended_at,omitempty"`
	Active    bool            `json:"active"`
	Actions   []SessionAction `json:"actions"`
	Summary   *SessionSummary `json:"summary,omitempty"`
}

type SessionAction struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Target    string `json:"target"`
	Details   string `json:"details,omitempty"`
}

type SessionSummary struct {
	Tool             string   `json:"tool"`
	Model            string   `json:"model,omitempty"`
	MemoriesCreated  []string `json:"memories_created,omitempty"`
	MemoriesAccessed []string `json:"memories_accessed,omitempty"`
	SearchQueries    []string `json:"search_queries,omitempty"`
	TopKeywords      []string `json:"top_keywords,omitempty"`
	ActionCount      int      `json:"action_count"`
	Duration         string   `json:"duration"`
}

type AccessStore struct {
	Memories map[string]*MemoryAccess `json:"memories"`
}

type MemoryAccess struct {
	Created      string   `json:"created"`
	LastAccessed string   `json:"last_accessed"`
	AccessCount  int      `json:"access_count"`
	SearchHits   int      `json:"search_hits"`
	Tools        []string `json:"tools"`
	RecentQuery  string   `json:"recent_query,omitempty"`
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type MCPResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text"`
}
