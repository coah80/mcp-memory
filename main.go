package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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

type JournalEntry struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Details   string `json:"details,omitempty"`
}

type SimilarMemory struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Summary     string  `json:"summary"`
	Score       float64 `json:"score"`
}

type MemoryServer struct {
	memoryDir   string
	journalDir  string
	soulFile    string
	indexPath   string
	mu          sync.RWMutex
	indexCache  []MemoryIndex
	indexLoaded bool
}

func NewMemoryServer(memoryDir string) *MemoryServer {
	journalDir := filepath.Join(memoryDir, "journal")
	soulFile := filepath.Join(memoryDir, "SOUL.md")
	indexPath := filepath.Join(memoryDir, "index.json")

	os.MkdirAll(memoryDir, 0755)
	os.MkdirAll(journalDir, 0755)

	return &MemoryServer{
		memoryDir:  memoryDir,
		journalDir: journalDir,
		soulFile:   soulFile,
		indexPath:  indexPath,
	}
}

func (s *MemoryServer) logJournal(action, target, details string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	timestamp := time.Now().Format(time.RFC3339)
	date := time.Now().Format("2006-01-02")

	journalFile := filepath.Join(s.journalDir, date+".md")

	var content string
	if data, err := os.ReadFile(journalFile); err == nil {
		content = string(data)
	} else {
		content = fmt.Sprintf("# Journal - %s\n\n", date)
	}

	content += fmt.Sprintf("- **%s** [%s] %s: %s\n",
		time.Now().Format("15:04:05"), action, target, details)

	os.WriteFile(journalFile, []byte(content), 0644)

	logFile := filepath.Join(s.journalDir, "journal.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		entry := JournalEntry{
			Timestamp: timestamp,
			Action:    action,
			Target:    target,
			Details:   details,
		}
		entryJSON, _ := json.Marshal(entry)
		f.WriteString(string(entryJSON) + "\n")
	}
}

func (s *MemoryServer) extractKeywords(content string) []string {
	// Simple keyword extraction - split by common delimiters and filter
	words := strings.FieldsFunc(content, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '.' || r == ';' || r == ':'
	})

	keywordMap := make(map[string]bool)
	keywords := []string{}

	// Filter out common words and short words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true, "can": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"into": true, "through": true, "during": true, "before": true,
		"after": true, "above": true, "below": true, "between": true,
		"and": true, "but": true, "or": true, "nor": true, "not": true,
		"so": true, "yet": true, "both": true, "either": true, "neither": true,
		"each": true, "every": true, "all": true, "any": true, "few": true,
		"more": true, "most": true, "other": true, "some": true, "such": true,
		"no": true, "only": true, "own": true, "same": true, "than": true,
		"too": true, "very": true, "just": true, "because": true, "if": true,
		"when": true, "where": true, "how": true, "what": true, "which": true,
		"who": true, "whom": true, "this": true, "that": true, "these": true,
		"those": true, "i": true, "me": true, "my": true, "we": true,
		"our": true, "you": true, "your": true, "he": true, "him": true,
		"his": true, "she": true, "her": true, "it": true, "its": true,
		"they": true, "them": true, "their": true, "about": true, "up": true,
		"out": true, "then": true,
	}

	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 2 && !stopWords[word] && !keywordMap[word] {
			keywordMap[word] = true
			keywords = append(keywords, word)
			if len(keywords) >= 20 {
				break
			}
		}
	}

	return keywords
}

func (s *MemoryServer) generateSummary(content string) string {
	// Take first 200 characters as summary
	if len(content) > 200 {
		return content[:200] + "..."
	}
	return content
}

func (s *MemoryServer) buildIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	files, err := filepath.Glob(filepath.Join(s.memoryDir, "*.md"))
	if err != nil {
		return err
	}

	index := make([]MemoryIndex, 0, len(files))

	for _, file := range files {
		if filepath.Base(file) == "SOUL.md" || filepath.Base(file) == "README.md" {
			continue
		}

		mem, err := s.parseMemoryFile(file)
		if err != nil {
			continue
		}

		// Extract keywords from name, description, and content
		allText := mem.Name + " " + mem.Description + " " + mem.Content
		keywords := s.extractKeywords(allText)

		summary := s.generateSummary(mem.Content)

		index = append(index, MemoryIndex{
			Name:        mem.Name,
			Description: mem.Description,
			Type:        mem.Type,
			Keywords:    keywords,
			ModTime:     mem.ModTime,
			Summary:     summary,
		})
	}

	s.indexCache = index
	s.indexLoaded = true

	// Save index to disk
	indexJSON, _ := json.MarshalIndent(index, "", "  ")
	os.WriteFile(s.indexPath, indexJSON, 0644)

	return nil
}

func (s *MemoryServer) loadIndex() error {
	s.mu.RLock()
	if s.indexLoaded {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	// Try to load from disk first
	if data, err := os.ReadFile(s.indexPath); err == nil {
		var index []MemoryIndex
		if err := json.Unmarshal(data, &index); err == nil {
			s.mu.Lock()
			s.indexCache = index
			s.indexLoaded = true
			s.mu.Unlock()
			return nil
		}
	}

	// Build index if not found
	return s.buildIndex()
}

func (s *MemoryServer) findSimilar(query string, threshold float64) []SimilarMemory {
	if err := s.loadIndex(); err != nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	results := []SimilarMemory{}

	for _, idx := range s.indexCache {
		score := 0.0

		// Check name similarity
		nameLower := strings.ToLower(idx.Name)
		if strings.Contains(nameLower, queryLower) || strings.Contains(queryLower, nameLower) {
			score += 0.5
		}

		// Check description similarity
		descLower := strings.ToLower(idx.Description)
		if strings.Contains(descLower, queryLower) {
			score += 0.3
		}

		// Check keyword overlap
		keywordMatches := 0
		for _, qWord := range queryWords {
			for _, kWord := range idx.Keywords {
				if strings.Contains(kWord, qWord) || strings.Contains(qWord, kWord) {
					keywordMatches++
					break
				}
			}
		}
		if len(queryWords) > 0 {
			score += float64(keywordMatches) / float64(len(queryWords)) * 0.4
		}

		if score >= threshold {
			results = append(results, SimilarMemory{
				Name:        idx.Name,
				Description: idx.Description,
				Summary:     idx.Summary,
				Score:       score,
			})
		}
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Return top 3
	if len(results) > 3 {
		results = results[:3]
	}

	return results
}

func (s *MemoryServer) parseMemoryFile(path string) (*Memory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	description := ""
	memType := ""

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			content = strings.TrimSpace(parts[2])

			scanner := bufio.NewScanner(strings.NewReader(frontmatter))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "description:") {
					description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				} else if strings.HasPrefix(line, "type:") {
					memType = strings.TrimSpace(strings.TrimPrefix(line, "type:"))
				}
			}
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	return &Memory{
		Name:        name,
		Description: description,
		Type:        memType,
		Content:     content,
		ModTime:     info.ModTime().Format(time.RFC3339),
	}, nil
}

func (s *MemoryServer) handleListMemories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.logJournal("list", "memories", "Listed all memories")

	files, err := filepath.Glob(filepath.Join(s.memoryDir, "*.md"))
	if err != nil {
		http.Error(w, "Failed to list memories", http.StatusInternalServerError)
		return
	}

	memories := make([]*Memory, 0, len(files))
	for _, file := range files {
		if filepath.Base(file) == "SOUL.md" {
			continue
		}

		mem, err := s.parseMemoryFile(file)
		if err != nil {
			log.Printf("Error parsing %s: %v", file, err)
			continue
		}
		memories = append(memories, mem)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(memories)
}

func (s *MemoryServer) handleGetMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/memories/")
	if name == "" {
		http.Error(w, "Memory name required", http.StatusBadRequest)
		return
	}

	name = filepath.Base(name)
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}

	path := filepath.Join(s.memoryDir, name)
	mem, err := s.parseMemoryFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Memory not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to read memory", http.StatusInternalServerError)
		}
		return
	}

	s.logJournal("read", name, "Read memory")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mem)
}

func (s *MemoryServer) handleCreateMemory(w http.ResponseWriter, r *http.Request) {
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

	mem.Name = filepath.Base(mem.Name)
	if !strings.HasSuffix(mem.Name, ".md") {
		mem.Name += ".md"
	}

	content := "---\n"
	if mem.Description != "" {
		content += fmt.Sprintf("description: %s\n", mem.Description)
	}
	if mem.Type != "" {
		content += fmt.Sprintf("type: %s\n", mem.Type)
	}
	content += "---\n\n"
	content += mem.Content

	path := filepath.Join(s.memoryDir, mem.Name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		http.Error(w, "Failed to write memory", http.StatusInternalServerError)
		return
	}

	s.logJournal("create", mem.Name, fmt.Sprintf("Created memory: %s", mem.Description))

	// Rebuild index after creating memory
	go s.buildIndex()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "created", "name": mem.Name})
}

func (s *MemoryServer) handleDeleteMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/memories/")
	if name == "" {
		http.Error(w, "Memory name required", http.StatusBadRequest)
		return
	}

	name = filepath.Base(name)
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}

	path := filepath.Join(s.memoryDir, name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Memory not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete memory", http.StatusInternalServerError)
		}
		return
	}

	s.logJournal("delete", name, "Deleted memory")

	// Rebuild index after deleting memory
	go s.buildIndex()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "name": name})
}

func (s *MemoryServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' required", http.StatusBadRequest)
		return
	}

	s.logJournal("search", query, "Searched memories")

	files, err := filepath.Glob(filepath.Join(s.memoryDir, "*.md"))
	if err != nil {
		http.Error(w, "Failed to search memories", http.StatusInternalServerError)
		return
	}

	results := make([]*Memory, 0)
	queryLower := strings.ToLower(query)

	for _, file := range files {
		if filepath.Base(file) == "SOUL.md" {
			continue
		}

		mem, err := s.parseMemoryFile(file)
		if err != nil {
			continue
		}

		if strings.Contains(strings.ToLower(mem.Name), queryLower) ||
			strings.Contains(strings.ToLower(mem.Description), queryLower) ||
			strings.Contains(strings.ToLower(mem.Content), queryLower) {
			results = append(results, mem)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *MemoryServer) handleSimilar(w http.ResponseWriter, r *http.Request) {
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

	s.logJournal("similar", query, "Checked for similar memories")

	results := s.findSimilar(query, threshold)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *MemoryServer) handleGetSoul(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.logJournal("read", "SOUL.md", "Read soul file")

	mem, err := s.parseMemoryFile(s.soulFile)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"name":    "SOUL",
				"content": "# Soul\n\nDefine your persistent persona here.\n",
			})
			return
		}
		http.Error(w, "Failed to read soul", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mem)
}

func (s *MemoryServer) handleUpdateSoul(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	content := "---\ndescription: Persistent persona and identity\ntype: soul\n---\n\n" + req.Content

	if err := os.WriteFile(s.soulFile, []byte(content), 0644); err != nil {
		http.Error(w, "Failed to write soul", http.StatusInternalServerError)
		return
	}

	s.logJournal("update", "SOUL.md", "Updated soul file")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (s *MemoryServer) handleGetJournal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	journalFile := filepath.Join(s.journalDir, date+".md")
	data, err := os.ReadFile(journalFile)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"date":    date,
				"content": fmt.Sprintf("# Journal - %s\n\nNo entries yet.\n", date),
			})
			return
		}
		http.Error(w, "Failed to read journal", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"date":    date,
		"content": string(data),
	})
}

func (s *MemoryServer) handleAppendJournal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Entry string `json:"entry"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.logJournal("note", "journal", req.Entry)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "logged"})
}

func (s *MemoryServer) handleAutoLog(w http.ResponseWriter, r *http.Request) {
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

	s.logJournal("complete", req.Task, req.Result)

	// If save_as is provided, save as a memory
	if req.SaveAs != "" {
		mem := Memory{
			Name:        req.SaveAs,
			Description: fmt.Sprintf("Auto-saved: %s", req.Task),
			Type:        "auto",
			Content:     fmt.Sprintf("## Task\n%s\n\n## Result\n%s\n", req.Task, req.Result),
		}

		mem.Name = filepath.Base(mem.Name)
		if !strings.HasSuffix(mem.Name, ".md") {
			mem.Name += ".md"
		}

		content := "---\n"
		content += fmt.Sprintf("description: %s\n", mem.Description)
		content += fmt.Sprintf("type: %s\n", mem.Type)
		content += "---\n\n"
		content += mem.Content

		path := filepath.Join(s.memoryDir, mem.Name)
		os.WriteFile(path, []byte(content), 0644)

		// Rebuild index
		go s.buildIndex()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "logged"})
}

func (s *MemoryServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.loadIndex(); err != nil {
		http.Error(w, "Failed to load index", http.StatusInternalServerError)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.indexCache)
}

func (s *MemoryServer) handleRebuildIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.buildIndex(); err != nil {
		http.Error(w, "Failed to rebuild index", http.StatusInternalServerError)
		return
	}

	s.logJournal("rebuild", "index", "Rebuilt memory index")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "rebuilt"})
}

func (s *MemoryServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "mcp-memory"})
}

func main() {
	memoryDir := os.Getenv("MEMORY_DIR")
	if memoryDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal("Failed to get home directory")
		}
		memoryDir = filepath.Join(home, ".mcp-memory", "memories")
	}

	server := NewMemoryServer(memoryDir)

	// Create default SOUL.md if it doesn't exist
	if _, err := os.Stat(server.soulFile); os.IsNotExist(err) {
		defaultSoul := `---
description: Persistent persona and identity
type: soul
---

# Soul

This is your persistent persona file. Define who you are across all AI tools.

## Identity
- Name: [Your name]
- Role: [Your role]
- Personality: [Your personality traits]

## Preferences
- Communication style: [How you like to communicate]
- Topics of interest: [What you care about]
- Values: [What matters to you]

## Memory Notes
- Remember to [important things to remember]
- Always [things to always do]
- Never [things to never do]
`
		os.WriteFile(server.soulFile, []byte(defaultSoul), 0644)
	}

	// Create default README if directory is empty
	files, _ := filepath.Glob(filepath.Join(memoryDir, "*.md"))
	if len(files) == 0 {
		readme := filepath.Join(memoryDir, "README.md")
		content := `# MCP Memory

This is your unified memory directory. All AI tools can read and write memories here.

## Memory Format

Memories are markdown files with YAML frontmatter:

` + "```markdown" + `
---
description: Memory description
type: user
---

Memory content goes here...
` + "```" + `

## Special Files

- **SOUL.md**: Your persistent persona and identity
- **journal/**: Daily logs of all actions
- **index.json**: Lightweight memory index for fast lookups
- **README.md**: This file

## Supported Tools

- Claude Code
- OpenCode
- Cursor
- Any tool that supports MCP
`
		os.WriteFile(readme, []byte(content), 0644)
	}

	// Build initial index
	go server.buildIndex()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.handleHealth)
	mux.HandleFunc("/memories", server.handleListMemories)
	mux.HandleFunc("/memories/search", server.handleSearch)
	mux.HandleFunc("/memories/similar", server.handleSimilar)
	mux.HandleFunc("/memories/create", server.handleCreateMemory)
	mux.HandleFunc("/memories/autolog", server.handleAutoLog)
	mux.HandleFunc("/memories/index", server.handleIndex)
	mux.HandleFunc("/memories/rebuild", server.handleRebuildIndex)
	mux.HandleFunc("/memories/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			server.handleGetMemory(w, r)
		case http.MethodDelete:
			server.handleDeleteMemory(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/soul", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			server.handleGetSoul(w, r)
		case http.MethodPost:
			server.handleUpdateSoul(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/journal", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			server.handleGetJournal(w, r)
		case http.MethodPost:
			server.handleAppendJournal(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	log.Printf("MCP Memory Server starting on :%s", port)
	log.Printf("Memory directory: %s", memoryDir)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
