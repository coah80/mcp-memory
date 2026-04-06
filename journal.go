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

type Journal struct {
	storage *Storage
	mu      sync.Mutex
}

func NewJournal(storage *Storage) *Journal {
	return &Journal{storage: storage}
}

func (j *Journal) Log(action, target, details string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	now := time.Now()
	date := now.Format("2006-01-02")
	journalFile := filepath.Join(j.storage.JournalDir, date+".md")

	var content string
	if data, err := os.ReadFile(journalFile); err == nil {
		content = string(data)
	} else {
		content = fmt.Sprintf("# Journal - %s\n\n", date)
	}

	content += fmt.Sprintf("- **%s** [%s] %s: %s\n",
		now.Format("15:04:05"), action, target, details)

	os.WriteFile(journalFile, []byte(content), 0644)

	logFile := filepath.Join(j.storage.JournalDir, "journal.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		entry := JournalEntry{
			Timestamp: now.Format(time.RFC3339),
			Action:    action,
			Target:    target,
			Details:   details,
		}
		entryJSON, _ := json.Marshal(entry)
		f.WriteString(string(entryJSON) + "\n")
	}
}

func (j *Journal) Read(date string) (string, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	journalFile := filepath.Join(j.storage.JournalDir, date+".md")
	data, err := os.ReadFile(journalFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("# Journal - %s\n\nNo entries yet.\n", date), nil
		}
		return "", err
	}
	return string(data), nil
}

func (j *Journal) ReadRecent(days int) ([]JournalEntry, error) {
	logFile := filepath.Join(j.storage.JournalDir, "journal.log")
	data, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	entries := make([]JournalEntry, 0)

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry JournalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		ts, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			continue
		}
		if ts.After(cutoff) {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func (j *Journal) ReadRecentDates(days int) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(j.storage.JournalDir, "*.md"))
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	dates := make([]string, 0)

	for _, f := range files {
		date := strings.TrimSuffix(filepath.Base(f), ".md")
		if date >= cutoff {
			dates = append(dates, date)
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	return dates, nil
}
