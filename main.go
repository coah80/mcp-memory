package main

import (
	_ "embed"
	"flag"
	"log"
	"os"
	"path/filepath"
)

//go:embed RULES.md
var embeddedRules string

//go:embed SETUP.md
var embeddedSetup string

func main() {
	mode := flag.String("mode", "", "Server mode: http or mcp (default: http)")
	flag.Parse()

	// also check env for mode
	if *mode == "" {
		*mode = os.Getenv("MCP_MODE")
	}
	if *mode == "" {
		*mode = "http"
	}

	memoryDir := os.Getenv("MEMORY_DIR")
	if memoryDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal("Failed to get home directory")
		}
		memoryDir = filepath.Join(home, ".mcp-memory", "memories")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	// core components
	storage := NewStorage(memoryDir)
	storage.EnsureDefaults()

	index := NewIndex(storage)
	journal := NewJournal(storage)
	sessions := NewSessionManager(storage)
	access := NewAccessTracker(storage)

	// build initial index
	go index.Build()

	switch *mode {
	case "mcp":
		// in mcp mode, all logging goes to stderr (stdout is protocol channel)
		log.SetOutput(os.Stderr)

		server := NewMCPServer(storage, index, journal, sessions, access)
		if err := server.Run(); err != nil {
			log.Fatalf("MCP server error: %v", err)
		}

	case "http":
		server := NewHTTPServer(port, storage, index, journal, sessions, access)
		if err := server.Run(); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}

	default:
		log.Fatalf("Unknown mode: %s (use 'http' or 'mcp')", *mode)
	}
}
