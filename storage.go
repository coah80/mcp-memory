package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Storage struct {
	BaseDir    string
	MemoryDir  string
	JournalDir string
	SessionDir string
	SoulFile   string
	UserFile   string
	IndexPath  string
	AccessPath string
}

func NewStorage(baseDir string) *Storage {
	memDir := baseDir
	journalDir := filepath.Join(baseDir, "journal")
	sessionDir := filepath.Join(baseDir, "sessions")

	for _, d := range []string{memDir, journalDir, sessionDir} {
		os.MkdirAll(d, 0700)
	}

	return &Storage{
		BaseDir:    baseDir,
		MemoryDir:  memDir,
		JournalDir: journalDir,
		SessionDir: sessionDir,
		SoulFile:   filepath.Join(memDir, "SOUL.md"),
		UserFile:   filepath.Join(memDir, "USER.md"),
		IndexPath:  filepath.Join(memDir, "index.json"),
		AccessPath: filepath.Join(memDir, "access.json"),
	}
}

func (s *Storage) ParseMemoryFile(path string) (*Memory, error) {
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

func (s *Storage) CreateMemory(mem *Memory) error {
	name := filepath.Base(mem.Name)
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	mem.Name = name

	content := "---\n"
	if mem.Description != "" {
		content += fmt.Sprintf("description: %s\n", mem.Description)
	}
	if mem.Type != "" {
		content += fmt.Sprintf("type: %s\n", mem.Type)
	}
	content += "---\n\n"
	content += mem.Content

	return os.WriteFile(filepath.Join(s.MemoryDir, name), []byte(content), 0600)
}

func (s *Storage) GetMemory(name string) (*Memory, error) {
	name = filepath.Base(name)
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	return s.ParseMemoryFile(filepath.Join(s.MemoryDir, name))
}

func (s *Storage) ListMemories() ([]*Memory, error) {
	files, err := filepath.Glob(filepath.Join(s.MemoryDir, "*.md"))
	if err != nil {
		return nil, err
	}

	skip := map[string]bool{"SOUL.md": true, "USER.md": true, "README.md": true}
	memories := make([]*Memory, 0, len(files))

	for _, file := range files {
		if skip[filepath.Base(file)] {
			continue
		}
		mem, err := s.ParseMemoryFile(file)
		if err != nil {
			continue
		}
		memories = append(memories, mem)
	}

	return memories, nil
}

func (s *Storage) DeleteMemory(name string) error {
	name = filepath.Base(name)
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	return os.Remove(filepath.Join(s.MemoryDir, name))
}

func (s *Storage) ReadSpecialFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Storage) WriteSpecialFile(path string, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

func (s *Storage) EnsureDefaults() {
	if _, err := os.Stat(s.SoulFile); os.IsNotExist(err) {
		defaultSoul := `---
description: Persistent persona and identity
type: soul
---

# Soul

This is your persistent persona file. Define who you are across all AI tools.

## Identity
- Name: [Your name]
- Role: [Your role]

## Preferences
- Communication style: [How you like to communicate]
- Values: [What matters to you]

## Memory Notes
- Remember to [important things to remember]
- Always [things to always do]
- Never [things to never do]
`
		os.WriteFile(s.SoulFile, []byte(defaultSoul), 0600)
	}

	if _, err := os.Stat(s.UserFile); os.IsNotExist(err) {
		defaultUser := `---
description: Context about the human user
type: user
---

# User

Context about you that AI tools should know.

## About
- Name: [Your name]
- Role: [What you do]
- Timezone: [Your timezone]

## Preferences
- [Your working preferences]

## Current Focus
- [What you're working on]
`
		os.WriteFile(s.UserFile, []byte(defaultUser), 0600)
	}
}
