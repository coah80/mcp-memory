package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type Index struct {
	storage *Storage
	mu      sync.RWMutex
	cache   []MemoryIndex
	loaded  bool
}

func NewIndex(storage *Storage) *Index {
	return &Index{storage: storage}
}

var stopWords = map[string]bool{
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

func ExtractKeywords(content string, max int) []string {
	words := strings.FieldsFunc(content, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' ||
			r == '.' || r == ';' || r == ':' || r == '(' || r == ')' ||
			r == '[' || r == ']' || r == '{' || r == '}' || r == '"' ||
			r == '\'' || r == '`' || r == '#' || r == '*' || r == '-'
	})

	seen := make(map[string]bool)
	keywords := make([]string, 0, max)

	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 2 && !stopWords[word] && !seen[word] {
			seen[word] = true
			keywords = append(keywords, word)
			if len(keywords) >= max {
				break
			}
		}
	}

	return keywords
}

func GenerateSummary(content string, maxLen int) string {
	if len(content) > maxLen {
		return content[:maxLen] + "..."
	}
	return content
}

func (idx *Index) Build() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	files, err := filepath.Glob(filepath.Join(idx.storage.MemoryDir, "*.md"))
	if err != nil {
		return err
	}

	skip := map[string]bool{"SOUL.md": true, "USER.md": true, "README.md": true}
	index := make([]MemoryIndex, 0, len(files))

	for _, file := range files {
		if skip[filepath.Base(file)] {
			continue
		}

		mem, err := idx.storage.ParseMemoryFile(file)
		if err != nil {
			continue
		}

		allText := mem.Name + " " + mem.Description + " " + mem.Content
		keywords := ExtractKeywords(allText, 20)
		summary := GenerateSummary(mem.Content, 200)

		index = append(index, MemoryIndex{
			Name:        mem.Name,
			Description: mem.Description,
			Type:        mem.Type,
			Keywords:    keywords,
			ModTime:     mem.ModTime,
			Summary:     summary,
		})
	}

	idx.cache = index
	idx.loaded = true

	indexJSON, _ := json.MarshalIndent(index, "", "  ")
	os.WriteFile(idx.storage.IndexPath, indexJSON, 0600)

	return nil
}

func (idx *Index) Load() error {
	idx.mu.RLock()
	if idx.loaded {
		idx.mu.RUnlock()
		return nil
	}
	idx.mu.RUnlock()

	if data, err := os.ReadFile(idx.storage.IndexPath); err == nil {
		var cache []MemoryIndex
		if err := json.Unmarshal(data, &cache); err == nil {
			idx.mu.Lock()
			idx.cache = cache
			idx.loaded = true
			idx.mu.Unlock()
			return nil
		}
	}

	return idx.Build()
}

func (idx *Index) Search(query string) []*Memory {
	files, err := filepath.Glob(filepath.Join(idx.storage.MemoryDir, "*.md"))
	if err != nil {
		return nil
	}

	skip := map[string]bool{"SOUL.md": true, "USER.md": true, "README.md": true}
	results := make([]*Memory, 0)
	queryLower := strings.ToLower(query)

	for _, file := range files {
		if skip[filepath.Base(file)] {
			continue
		}
		mem, err := idx.storage.ParseMemoryFile(file)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(mem.Name), queryLower) ||
			strings.Contains(strings.ToLower(mem.Description), queryLower) ||
			strings.Contains(strings.ToLower(mem.Content), queryLower) {
			results = append(results, mem)
		}
	}

	journalFiles, _ := filepath.Glob(filepath.Join(idx.storage.JournalDir, "*.md"))
	for _, file := range journalFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(strings.ToLower(content), queryLower) {
			date := strings.TrimSuffix(filepath.Base(file), ".md")
			lines := strings.Split(content, "\n")
			var matchedLines []string
			for _, line := range lines {
				if strings.Contains(strings.ToLower(line), queryLower) {
					matchedLines = append(matchedLines, strings.TrimSpace(line))
				}
			}
			results = append(results, &Memory{
				Name:        "journal/" + date,
				Description: fmt.Sprintf("Journal entry from %s (%d matching lines)", date, len(matchedLines)),
				Type:        "journal",
				Content:     strings.Join(matchedLines, "\n"),
			})
		}
	}

	return results
}

func (idx *Index) FindSimilar(query string, threshold float64) []SimilarMemory {
	if err := idx.Load(); err != nil {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)
	results := make([]SimilarMemory, 0)

	for _, entry := range idx.cache {
		score := 0.0

		nameLower := strings.ToLower(entry.Name)
		if strings.Contains(nameLower, queryLower) || strings.Contains(queryLower, nameLower) {
			score += 0.5
		}

		descLower := strings.ToLower(entry.Description)
		if strings.Contains(descLower, queryLower) {
			score += 0.3
		}

		keywordMatches := 0
		for _, qWord := range queryWords {
			for _, kWord := range entry.Keywords {
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
				Name:        entry.Name,
				Description: entry.Description,
				Summary:     entry.Summary,
				Score:       score,
			})
		}
	}

	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > 5 {
		results = results[:5]
	}

	return results
}

type GrepMatch struct {
	Source  string `json:"source"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type GrepResult struct {
	File    string      `json:"file"`
	Matches []GrepMatch `json:"matches"`
}

func (idx *Index) Grep(pattern string, includeJournal bool) ([]GrepResult, error) {
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %v", err)
	}

	results := make([]GrepResult, 0)

	files, _ := filepath.Glob(filepath.Join(idx.storage.MemoryDir, "*.md"))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		matches := grepLines(re, string(data), filepath.Base(file))
		if len(matches) > 0 {
			results = append(results, GrepResult{
				File:    strings.TrimSuffix(filepath.Base(file), ".md"),
				Matches: matches,
			})
		}
	}

	if includeJournal {
		journalFiles, _ := filepath.Glob(filepath.Join(idx.storage.JournalDir, "*.md"))
		for _, file := range journalFiles {
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			matches := grepLines(re, string(data), filepath.Base(file))
			if len(matches) > 0 {
				results = append(results, GrepResult{
					File:    "journal/" + filepath.Base(file),
					Matches: matches,
				})
			}
		}
	}

	return results, nil
}

func grepLines(re *regexp.Regexp, content, source string) []GrepMatch {
	matches := make([]GrepMatch, 0)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if re.MatchString(line) {
			matches = append(matches, GrepMatch{
				Source:  source,
				Line:    i + 1,
				Content: strings.TrimSpace(line),
			})
		}
	}
	return matches
}

func (idx *Index) GetCache() []MemoryIndex {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	out := make([]MemoryIndex, len(idx.cache))
	copy(out, idx.cache)
	return out
}
