package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
)

type MCPServer struct {
	storage       *Storage
	index         *Index
	journal       *Journal
	sessions      *SessionManager
	access        *AccessTracker
	logger        *log.Logger
	activeSession string
}

func NewMCPServer(storage *Storage, index *Index, journal *Journal, sessions *SessionManager, access *AccessTracker) *MCPServer {
	return &MCPServer{
		storage:  storage,
		index:    index,
		journal:  journal,
		sessions: sessions,
		access:   access,
		logger:   log.New(os.Stderr, "[mcp] ", log.LstdFlags),
	}
}

func (m *MCPServer) Run() error {
	m.logger.Println("MCP Memory Server starting (stdio mode)")

	session := m.sessions.Start("mcp")
	m.activeSession = session.ID
	m.logger.Printf("Session started: %s", session.ID)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			m.writeError(nil, -32700, "Parse error")
			continue
		}

		m.handleRequest(req)
	}

	m.sessions.End(m.activeSession)
	m.logger.Println("Session ended, connection closed")

	return scanner.Err()
}

func (m *MCPServer) handleRequest(req JSONRPCRequest) {
	var id interface{}
	if req.ID != nil {
		json.Unmarshal(req.ID, &id)
	}

	switch req.Method {
	case "initialize":
		m.handleInitialize(id, req.Params)
	case "notifications/initialized":
		return
	case "tools/list":
		m.handleToolsList(id)
	case "tools/call":
		m.handleToolsCall(id, req.Params)
	case "resources/list":
		m.handleResourcesList(id)
	case "resources/read":
		m.handleResourcesRead(id, req.Params)
	case "prompts/list":
		m.handlePromptsList(id)
	case "prompts/get":
		m.handlePromptsGet(id, req.Params)
	case "ping":
		m.writeResult(id, map[string]interface{}{})
	default:
		if id != nil {
			m.writeError(id, -32601, fmt.Sprintf("Method not found: %s", req.Method))
		}
	}
}

func (m *MCPServer) handleInitialize(id interface{}, params json.RawMessage) {
	var init struct {
		ClientInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}
	if params != nil {
		json.Unmarshal(params, &init)
	}

	if init.ClientInfo.Name != "" {
		m.sessions.Identify(m.activeSession, init.ClientInfo.Name, "")
		m.logger.Printf("Client identified: %s %s", init.ClientInfo.Name, init.ClientInfo.Version)
	}

	m.writeResult(id, map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools":     map[string]interface{}{},
			"resources": map[string]interface{}{},
			"prompts":   map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "mcp-memory",
			"version": "2.0.0",
		},
	})
}

func (m *MCPServer) handleToolsList(id interface{}) {
	tools := []MCPTool{
		{
			Name:        "memory_create",
			Description: "Create or update a persistent memory. Memories persist across sessions and tools.",
			InputSchema: jsonSchema(map[string]interface{}{
				"name":        prop("string", "Memory name (used as filename)"),
				"content":     prop("string", "Memory content (markdown)"),
				"description": prop("string", "Short one-line description (required, used in memory_list and search)"),
				"type":        prop("string", "Memory type: user, project, feedback, reference, auto"),
			}, "name", "content", "description"),
		},
		{
			Name:        "memory_get",
			Description: "Read a specific memory by name.",
			InputSchema: jsonSchema(map[string]interface{}{
				"name": prop("string", "Memory name"),
			}, "name"),
		},
		{
			Name:        "memory_search",
			Description: "Search memories by keyword. Returns matching memories with content.",
			InputSchema: jsonSchema(map[string]interface{}{
				"query": prop("string", "Search query"),
			}, "query"),
		},
		{
			Name:        "memory_list",
			Description: "List all memories with lightweight index info (no full content).",
			InputSchema: jsonSchema(map[string]interface{}{}, ""),
		},
		{
			Name:        "memory_delete",
			Description: "Delete a memory by name.",
			InputSchema: jsonSchema(map[string]interface{}{
				"name": prop("string", "Memory name to delete"),
			}, "name"),
		},
		{
			Name:        "memory_similar",
			Description: "Find memories similar to a query using keyword overlap scoring.",
			InputSchema: jsonSchema(map[string]interface{}{
				"query":     prop("string", "Query to find similar memories for"),
				"threshold": prop("number", "Minimum similarity score 0-1 (default 0.3)"),
			}, "query"),
		},
		{
			Name:        "journal_log",
			Description: "Log an entry to today's journal. Use this to record decisions, observations, and progress.",
			InputSchema: jsonSchema(map[string]interface{}{
				"entry": prop("string", "Journal entry text"),
			}, "entry"),
		},
		{
			Name:        "journal_read",
			Description: "Read journal entries for a specific date.",
			InputSchema: jsonSchema(map[string]interface{}{
				"date": prop("string", "Date in YYYY-MM-DD format (default: today)"),
			}),
		},
		{
			Name:        "soul_update",
			Description: "Update the SOUL.md persona/identity file.",
			InputSchema: jsonSchema(map[string]interface{}{
				"content": prop("string", "New soul content (markdown)"),
			}, "content"),
		},
		{
			Name:        "user_update",
			Description: "Update the USER.md file with context about the human user.",
			InputSchema: jsonSchema(map[string]interface{}{
				"content": prop("string", "New user context content (markdown)"),
			}, "content"),
		},
		{
			Name:        "session_log",
			Description: "Log an action to the current session for tracking.",
			InputSchema: jsonSchema(map[string]interface{}{
				"action":  prop("string", "Action type (e.g. task_complete, decision, observation)"),
				"target":  prop("string", "What the action relates to"),
				"details": prop("string", "Additional details"),
			}, "action", "target"),
		},
		{
			Name:        "session_identify",
			Description: "Identify which AI model is being used in this session. Call this at the start of every session.",
			InputSchema: jsonSchema(map[string]interface{}{
				"model": prop("string", "Model identifier (e.g. claude-sonnet-4-6, gpt-4o, gemini-2.5-pro)"),
			}, "model"),
		},
		{
			Name:        "setup_guide",
			Description: "Run this to set up mcp-memory as your primary memory system. Returns instructions you MUST follow: add rules to your tool, migrate existing memories, and switch to using mcp-memory exclusively.",
			InputSchema: jsonSchema(map[string]interface{}{}),
		},
		{
			Name:        "memory_grep",
			Description: "Grep through all memories and journals with a regex pattern. Returns matching lines with file and line number, like grep.",
			InputSchema: jsonSchema(map[string]interface{}{
				"pattern":         prop("string", "Regex pattern to search for (case-insensitive)"),
				"include_journal": prop("boolean", "Also search journal entries (default: true)"),
			}, "pattern"),
		},
		{
			Name:        "self_update",
			Description: "Update mcp-memory to the latest version. Downloads and replaces the current binary.",
			InputSchema: jsonSchema(map[string]interface{}{}),
		},
	}

	m.writeResult(id, map[string]interface{}{"tools": tools})
}

func (m *MCPServer) handleToolsCall(id interface{}, params json.RawMessage) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		m.writeError(id, -32602, "Invalid params")
		return
	}

	var args map[string]interface{}
	if call.Arguments != nil {
		json.Unmarshal(call.Arguments, &args)
	}
	if args == nil {
		args = make(map[string]interface{})
	}

	getStr := func(key string) string {
		v, _ := args[key].(string)
		return v
	}
	getFloat := func(key string, def float64) float64 {
		v, ok := args[key].(float64)
		if !ok {
			return def
		}
		return v
	}

	var result string
	isError := false

	switch call.Name {
	case "memory_create":
		name := getStr("name")
		content := getStr("content")
		if name == "" || content == "" {
			result = "Error: name and content are required"
			isError = true
			break
		}
		mem := &Memory{
			Name:        name,
			Content:     content,
			Description: getStr("description"),
			Type:        getStr("type"),
		}
		if err := m.storage.CreateMemory(mem); err != nil {
			result = fmt.Sprintf("Error creating memory: %v", err)
			isError = true
		} else {
			m.access.RecordCreation(mem.Name)
			m.journal.Log("create", mem.Name, mem.Description)
			m.sessions.LogAction(m.activeSession, "memory_create", mem.Name, mem.Description)
			go m.index.Build()
			result = fmt.Sprintf("Created memory: %s", mem.Name)
		}

	case "memory_get":
		name := getStr("name")
		if name == "" {
			result = "Error: name is required"
			isError = true
			break
		}
		mem, err := m.storage.GetMemory(name)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			m.access.RecordAccess(mem.Name, "mcp")
			m.journal.Log("read", mem.Name, "Read memory")
			m.sessions.LogAction(m.activeSession, "memory_get", mem.Name, "")
			data, _ := json.MarshalIndent(mem, "", "  ")
			result = string(data)
		}

	case "memory_search":
		query := getStr("query")
		if query == "" {
			result = "Error: query is required"
			isError = true
			break
		}
		results := m.index.Search(query)
		for _, r := range results {
			m.access.RecordSearchHit(r.Name, query)
		}
		m.journal.Log("search", query, fmt.Sprintf("Found %d results", len(results)))
		m.sessions.LogAction(m.activeSession, "memory_search", query, fmt.Sprintf("%d results", len(results)))
		data, _ := json.MarshalIndent(results, "", "  ")
		result = string(data)

	case "memory_list":
		if err := m.index.Load(); err != nil {
			m.index.Build()
		}
		cache := m.index.GetCache()
		m.journal.Log("list", "index", fmt.Sprintf("%d memories", len(cache)))
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%d memories:\n\n", len(cache)))
		for _, entry := range cache {
			typeTag := ""
			if entry.Type != "" {
				typeTag = " [" + entry.Type + "]"
			}
			b.WriteString(fmt.Sprintf("- **%s**%s: %s\n", strings.TrimSuffix(entry.Name, ".md"), typeTag, entry.Description))
		}
		result = b.String()

	case "memory_delete":
		name := getStr("name")
		if name == "" {
			result = "Error: name is required"
			isError = true
			break
		}
		if err := m.storage.DeleteMemory(name); err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			m.access.RemoveMemory(name)
			m.journal.Log("delete", name, "Deleted memory")
			m.sessions.LogAction(m.activeSession, "memory_delete", name, "")
			go m.index.Build()
			result = fmt.Sprintf("Deleted: %s", name)
		}

	case "memory_similar":
		query := getStr("query")
		if query == "" {
			result = "Error: query is required"
			isError = true
			break
		}
		threshold := getFloat("threshold", 0.3)
		similar := m.index.FindSimilar(query, threshold)
		m.journal.Log("similar", query, fmt.Sprintf("Found %d similar", len(similar)))
		data, _ := json.MarshalIndent(similar, "", "  ")
		result = string(data)

	case "journal_log":
		entry := getStr("entry")
		if entry == "" {
			result = "Error: entry is required"
			isError = true
			break
		}
		m.journal.Log("note", "journal", entry)
		m.sessions.LogAction(m.activeSession, "journal_log", "journal", entry)
		result = "Logged to journal"

	case "journal_read":
		date := getStr("date")
		content, err := m.journal.Read(date)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			result = content
		}

	case "soul_update":
		content := getStr("content")
		if content == "" {
			result = "Error: content is required"
			isError = true
			break
		}
		full := "---\ndescription: Persistent persona and identity\ntype: soul\n---\n\n" + content
		if err := m.storage.WriteSpecialFile(m.storage.SoulFile, full); err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			m.journal.Log("update", "SOUL.md", "Updated soul")
			result = "Soul updated"
		}

	case "user_update":
		content := getStr("content")
		if content == "" {
			result = "Error: content is required"
			isError = true
			break
		}
		full := "---\ndescription: Context about the human user\ntype: user\n---\n\n" + content
		if err := m.storage.WriteSpecialFile(m.storage.UserFile, full); err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
		} else {
			m.journal.Log("update", "USER.md", "Updated user context")
			result = "User context updated"
		}

	case "session_log":
		action := getStr("action")
		target := getStr("target")
		details := getStr("details")
		m.sessions.LogAction(m.activeSession, action, target, details)
		m.journal.Log(action, target, details)
		result = "Logged"

	case "session_identify":
		model := getStr("model")
		if model == "" {
			result = "Error: model is required"
			isError = true
			break
		}
		m.sessions.Identify(m.activeSession, "", model)
		m.logger.Printf("Model identified: %s", model)
		result = fmt.Sprintf("Session updated — model: %s", model)

	case "setup_guide":
		result = embeddedSetup

	case "memory_grep":
		pattern := getStr("pattern")
		if pattern == "" {
			result = "Error: pattern is required"
			isError = true
			break
		}
		includeJournal := true
		if v, ok := args["include_journal"].(bool); ok {
			includeJournal = v
		}
		grepResults, err := m.index.Grep(pattern, includeJournal)
		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
			isError = true
			break
		}
		if len(grepResults) == 0 {
			result = "No matches found."
		} else {
			var b strings.Builder
			totalMatches := 0
			for _, gr := range grepResults {
				for _, match := range gr.Matches {
					b.WriteString(fmt.Sprintf("%s:%d: %s\n", gr.File, match.Line, match.Content))
					totalMatches++
				}
			}
			b.WriteString(fmt.Sprintf("\n%d matches across %d files", totalMatches, len(grepResults)))
			result = b.String()
		}
		m.sessions.LogAction(m.activeSession, "memory_grep", pattern, fmt.Sprintf("%d results", len(grepResults)))

	case "self_update":
		execPath, err := os.Executable()
		if err != nil {
			result = "Error: couldn't find current binary path"
			isError = true
			break
		}
		platform := runtime.GOOS + "-" + runtime.GOARCH
		url := "https://mcps.coah80.com/mcp-memory/mcp-memory-" + platform
		m.logger.Printf("Updating from %s", url)

		resp, err := http.Get(url)
		if err != nil {
			result = fmt.Sprintf("Error downloading update: %v", err)
			isError = true
			break
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			result = fmt.Sprintf("Error: server returned %d", resp.StatusCode)
			isError = true
			break
		}

		tmpPath := execPath + ".new"
		out, err := os.Create(tmpPath)
		if err != nil {
			result = fmt.Sprintf("Error creating temp file: %v", err)
			isError = true
			break
		}

		_, err = io.Copy(out, resp.Body)
		out.Close()
		if err != nil {
			os.Remove(tmpPath)
			result = fmt.Sprintf("Error writing update: %v", err)
			isError = true
			break
		}

		os.Chmod(tmpPath, 0755)
		if err := os.Rename(tmpPath, execPath); err != nil {
			os.Remove(tmpPath)
			result = fmt.Sprintf("Error replacing binary: %v", err)
			isError = true
			break
		}

		m.journal.Log("self_update", platform, "Updated to latest version")
		result = fmt.Sprintf("Updated mcp-memory (%s). Restart to use new version.", platform)

	default:
		m.writeError(id, -32601, fmt.Sprintf("Unknown tool: %s", call.Name))
		return
	}

	m.writeResult(id, map[string]interface{}{
		"content": []MCPContent{{Type: "text", Text: result}},
		"isError": isError,
	})
}

func (m *MCPServer) handleResourcesList(id interface{}) {
	resources := []MCPResource{
		{
			URI:         "memory://soul",
			Name:        "Soul",
			Description: "Your persistent identity and personality across all AI tools",
			MimeType:    "text/markdown",
		},
		{
			URI:         "memory://user",
			Name:        "User Context",
			Description: "Context about the human user: role, preferences, current focus",
			MimeType:    "text/markdown",
		},
		{
			URI:         "memory://journal/today",
			Name:        "Today's Journal",
			Description: "Today's activity log",
			MimeType:    "text/markdown",
		},
		{
			URI:         "memory://sessions/recent",
			Name:        "Recent Sessions",
			Description: "Summaries of recent sessions across all tools",
			MimeType:    "text/markdown",
		},
		{
			URI:         "memory://index",
			Name:        "Memory Index",
			Description: "Lightweight index of all memories (names, types, summaries)",
			MimeType:    "application/json",
		},
		{
			URI:         "memory://rules",
			Name:        "Memory Rules",
			Description: "Instructions for AI tools on how to use mcp-memory effectively",
			MimeType:    "text/markdown",
		},
	}

	m.writeResult(id, map[string]interface{}{"resources": resources})
}

func (m *MCPServer) handleResourcesRead(id interface{}, params json.RawMessage) {
	var req struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		m.writeError(id, -32602, "Invalid params")
		return
	}

	var content string
	var mimeType string

	switch req.URI {
	case "memory://soul":
		data, err := m.storage.ReadSpecialFile(m.storage.SoulFile)
		if err != nil {
			content = "No soul file configured yet."
		} else {
			content = data
		}
		mimeType = "text/markdown"

	case "memory://user":
		data, err := m.storage.ReadSpecialFile(m.storage.UserFile)
		if err != nil {
			content = "No user file configured yet."
		} else {
			content = data
		}
		mimeType = "text/markdown"

	case "memory://journal/today":
		data, _ := m.journal.Read("")
		content = data
		mimeType = "text/markdown"

	case "memory://sessions/recent":
		content = m.sessions.RecentSummaryText(5)
		mimeType = "text/markdown"

	case "memory://index":
		if err := m.index.Load(); err != nil {
			m.index.Build()
		}
		cache := m.index.GetCache()
		data, _ := json.MarshalIndent(cache, "", "  ")
		content = string(data)
		mimeType = "application/json"

	case "memory://rules":
		content = embeddedRules
		mimeType = "text/markdown"

	default:
		m.writeError(id, -32602, fmt.Sprintf("Unknown resource: %s", req.URI))
		return
	}

	m.writeResult(id, map[string]interface{}{
		"contents": []MCPResourceContent{{
			URI:      req.URI,
			MimeType: mimeType,
			Text:     content,
		}},
	})
}

func (m *MCPServer) handlePromptsList(id interface{}) {
	prompts := []map[string]interface{}{
		{
			"name":        "recall",
			"description": "Recall everything known about a topic from memory",
			"arguments": []map[string]interface{}{
				{"name": "topic", "description": "Topic to recall", "required": true},
			},
		},
		{
			"name":        "session_review",
			"description": "Review what happened in recent sessions",
			"arguments":   []map[string]interface{}{},
		},
		{
			"name":        "wake_up",
			"description": "Start of session briefing: recent context, soul, journal",
			"arguments":   []map[string]interface{}{},
		},
	}

	m.writeResult(id, map[string]interface{}{"prompts": prompts})
}

func (m *MCPServer) handlePromptsGet(id interface{}, params json.RawMessage) {
	var req struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		m.writeError(id, -32602, "Invalid params")
		return
	}

	var messages []map[string]interface{}

	switch req.Name {
	case "recall":
		topic := req.Arguments["topic"]
		results := m.index.Search(topic)
		similar := m.index.FindSimilar(topic, 0.2)

		var content strings.Builder
		content.WriteString(fmt.Sprintf("# Recalling: %s\n\n", topic))

		if len(results) > 0 {
			content.WriteString("## Direct Matches\n\n")
			for _, mem := range results {
				content.WriteString(fmt.Sprintf("### %s\n%s\n\n%s\n\n", mem.Name, mem.Description, mem.Content))
			}
		}

		if len(similar) > 0 {
			content.WriteString("## Related Memories\n\n")
			for _, s := range similar {
				content.WriteString(fmt.Sprintf("- **%s** (%.0f%%): %s\n", s.Name, s.Score*100, s.Summary))
			}
		}

		if len(results) == 0 && len(similar) == 0 {
			content.WriteString("No memories found about this topic.\n")
		}

		messages = []map[string]interface{}{
			{"role": "user", "content": map[string]interface{}{"type": "text", "text": content.String()}},
		}

	case "session_review":
		text := m.sessions.RecentSummaryText(10)
		messages = []map[string]interface{}{
			{"role": "user", "content": map[string]interface{}{"type": "text", "text": "# Recent Session Review\n\n" + text}},
		}

	case "wake_up":
		var content strings.Builder
		content.WriteString("# Session Briefing\n\n")

		soul, err := m.storage.ReadSpecialFile(m.storage.SoulFile)
		if err == nil {
			content.WriteString("## Soul\n\n")
			content.WriteString(soul)
			content.WriteString("\n\n")
		}

		content.WriteString("## Recent Sessions\n\n")
		content.WriteString(m.sessions.RecentSummaryText(3))

		journal, _ := m.journal.Read("")
		if !strings.Contains(journal, "No entries yet") {
			content.WriteString("## Today's Journal\n\n")
			content.WriteString(journal)
		}

		messages = []map[string]interface{}{
			{"role": "user", "content": map[string]interface{}{"type": "text", "text": content.String()}},
		}

	default:
		m.writeError(id, -32602, fmt.Sprintf("Unknown prompt: %s", req.Name))
		return
	}

	m.writeResult(id, map[string]interface{}{
		"description": fmt.Sprintf("Prompt: %s", req.Name),
		"messages":    messages,
	})
}

func (m *MCPServer) writeResult(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintln(os.Stdout, string(data))
}

func (m *MCPServer) writeError(id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintln(os.Stdout, string(data))
}

func prop(typeName, desc string) map[string]interface{} {
	return map[string]interface{}{"type": typeName, "description": desc}
}

func jsonSchema(properties map[string]interface{}, required ...string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	req := make([]string, 0)
	for _, r := range required {
		if r != "" {
			req = append(req, r)
		}
	}
	if len(req) > 0 {
		schema["required"] = req
	}
	return schema
}
