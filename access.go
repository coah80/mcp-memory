package main

import (
	"encoding/json"
	"math"
	"os"
	"sync"
	"time"
)

type AccessTracker struct {
	storage *Storage
	mu      sync.Mutex
	store   AccessStore
	loaded  bool
}

func NewAccessTracker(storage *Storage) *AccessTracker {
	return &AccessTracker{
		storage: storage,
		store:   AccessStore{Memories: make(map[string]*MemoryAccess)},
	}
}

func (at *AccessTracker) load() {
	if at.loaded {
		return
	}

	data, err := os.ReadFile(at.storage.AccessPath)
	if err != nil {
		at.loaded = true
		return
	}

	var store AccessStore
	if err := json.Unmarshal(data, &store); err != nil {
		at.loaded = true
		return
	}

	at.store = store
	if at.store.Memories == nil {
		at.store.Memories = make(map[string]*MemoryAccess)
	}
	at.loaded = true
}

func (at *AccessTracker) save() {
	data, _ := json.MarshalIndent(at.store, "", "  ")
	os.WriteFile(at.storage.AccessPath, data, 0644)
}

func (at *AccessTracker) RecordAccess(memoryName, tool string) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.load()

	now := time.Now().Format(time.RFC3339)

	access, ok := at.store.Memories[memoryName]
	if !ok {
		access = &MemoryAccess{
			Created: now,
			Tools:   make([]string, 0),
		}
		at.store.Memories[memoryName] = access
	}

	access.LastAccessed = now
	access.AccessCount++

	if !containsStr(access.Tools, tool) && tool != "" {
		access.Tools = append(access.Tools, tool)
	}

	at.save()
}

func (at *AccessTracker) RecordSearchHit(memoryName, query string) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.load()

	now := time.Now().Format(time.RFC3339)

	access, ok := at.store.Memories[memoryName]
	if !ok {
		access = &MemoryAccess{
			Created: now,
			Tools:   make([]string, 0),
		}
		at.store.Memories[memoryName] = access
	}

	access.SearchHits++
	access.RecentQuery = query
	access.LastAccessed = now

	at.save()
}

func (at *AccessTracker) RecordCreation(memoryName string) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.load()

	now := time.Now().Format(time.RFC3339)

	at.store.Memories[memoryName] = &MemoryAccess{
		Created:      now,
		LastAccessed: now,
		AccessCount:  0,
		Tools:        make([]string, 0),
	}

	at.save()
}

func (at *AccessTracker) RemoveMemory(memoryName string) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.load()

	delete(at.store.Memories, memoryName)
	at.save()
}

func (at *AccessTracker) ImportanceScore(memoryName string) float64 {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.load()

	access, ok := at.store.Memories[memoryName]
	if !ok {
		return 0
	}

	freq := math.Min(float64(access.AccessCount+access.SearchHits), 50) / 50.0

	lastAccessed, err := time.Parse(time.RFC3339, access.LastAccessed)
	recency := 0.0
	if err == nil {
		daysSince := time.Since(lastAccessed).Hours() / 24
		recency = math.Exp(-0.099 * daysSince)
	}

	diversity := math.Min(float64(len(access.Tools)), 5) / 5.0

	return freq*0.4 + recency*0.4 + diversity*0.2
}

func (at *AccessTracker) GetStaleMemories(daysThreshold int) []string {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.load()

	cutoff := time.Now().AddDate(0, 0, -daysThreshold)
	stale := make([]string, 0)

	for name, access := range at.store.Memories {
		lastAccessed, err := time.Parse(time.RFC3339, access.LastAccessed)
		if err != nil {
			continue
		}
		if lastAccessed.Before(cutoff) {
			stale = append(stale, name)
		}
	}

	return stale
}

func (at *AccessTracker) GetStats() AccessStore {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.load()

	out := AccessStore{Memories: make(map[string]*MemoryAccess, len(at.store.Memories))}
	for k, v := range at.store.Memories {
		cp := *v
		cp.Tools = make([]string, len(v.Tools))
		copy(cp.Tools, v.Tools)
		out.Memories[k] = &cp
	}
	return out
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
