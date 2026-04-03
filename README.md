# mcp-memory

Persistent memory server for AI coding tools. Run it as a background service, and any AI tool -- Claude Code, Cursor, Claude Desktop, Codex, OpenCode, Windsurf, ForgeCode, or anything that can make HTTP calls -- gets persistent memory across sessions.

Written in Go. Zero dependencies. Single binary.

## Quick Install

### macOS / Linux
```bash
curl -fsSL https://mcps.coah80.com/mcp-memory/install.sh | bash
```

### Windows (PowerShell)
```powershell
irm https://mcps.coah80.com/mcp-memory/install.ps1 | iex
```

### From Source
```bash
go build -o mcp-memory main.go
./mcp-memory
```

## How It Works

mcp-memory runs as a lightweight HTTP server on `localhost:8090`. AI tools store and retrieve memories via simple REST calls. Memories persist as markdown files with YAML frontmatter in `~/.mcp-memory/memories/`.

```
Your AI Tool  --HTTP-->  mcp-memory (localhost:8090)  -->  ~/.mcp-memory/memories/
```

## Setup by Tool

### Claude Code
Add to your project's `CLAUDE.md` or `~/.claude/CLAUDE.md`:
```markdown
# Memory
You have access to a persistent memory server at http://localhost:8090.
- List memories: GET http://localhost:8090/memories
- Read memory: GET http://localhost:8090/memories/{name}
- Create memory: POST http://localhost:8090/memories/create (JSON body: name, description, type, content)
- Search: GET http://localhost:8090/memories/search?q={query}
- Delete: DELETE http://localhost:8090/memories/{name}
```

### Cursor
Add to `.cursor/rules` or `.cursorrules`:
```
You have access to a persistent memory server at http://localhost:8090.
Use curl to interact with it:
- curl http://localhost:8090/memories (list all)
- curl http://localhost:8090/memories/{name} (read one)
- curl -X POST http://localhost:8090/memories/create -d '{"name":"...","content":"..."}' (create)
- curl http://localhost:8090/memories/search?q=query (search)
```

### Claude Desktop / Any MCP Client
You can wrap mcp-memory as an MCP tool server. See [examples/mcp-wrapper/](examples/mcp-wrapper/) for a reference implementation.

### Codex / OpenCode / Windsurf / ForgeCode / Any AI Tool
Any tool that can execute shell commands or make HTTP requests works. Point it at `http://localhost:8090` and use the API below.

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/memories` | List all memories |
| `GET` | `/memories/{name}` | Get a specific memory |
| `POST` | `/memories/create` | Create or update a memory |
| `DELETE` | `/memories/{name}` | Delete a memory |
| `GET` | `/memories/search?q=` | Full-text search |
| `GET` | `/memories/similar?q=&threshold=0.3` | Find similar memories |
| `GET` | `/memories/index` | Get lightweight index (for token-efficient listing) |
| `POST` | `/memories/rebuild` | Rebuild search index |
| `GET` | `/soul` | Get SOUL.md (persistent persona) |
| `POST` | `/soul` | Update SOUL.md |
| `GET` | `/journal?date=YYYY-MM-DD` | Get journal entries |
| `POST` | `/journal` | Append journal entry |
| `POST` | `/memories/autolog` | Log a completed task |

### Create Memory
```bash
curl -X POST http://localhost:8090/memories/create \
  -H "Content-Type: application/json" \
  -d '{
    "name": "user-preferences",
    "description": "User coding style preferences",
    "type": "user",
    "content": "Prefers functional style, uses TypeScript, dark theme"
  }'
```

### Search
```bash
curl "http://localhost:8090/memories/search?q=typescript"
```

### Memory Format
Memories are stored as markdown files with YAML frontmatter:
```markdown
---
description: User coding style preferences
type: user
---

Prefers functional style, uses TypeScript, dark theme
```

## Configuration

| Env Variable | Default | Description |
|-------------|---------|-------------|
| `PORT` | `8090` | Server port |
| `MEMORY_DIR` | `~/.mcp-memory/memories` | Memory storage directory |

Config file location: `~/.config/mcp-memory/config.json`

## Features

- **Zero dependencies** -- Single Go binary, no runtime needed
- **File-based storage** -- Memories are plain markdown files, version-controllable
- **Keyword indexing** -- Fast search without loading full file contents
- **Similarity detection** -- Find related memories by keyword overlap
- **Journal system** -- Daily action logs for audit trail
- **SOUL.md** -- Persistent persona/identity file
- **Cross-platform** -- Linux, macOS, Windows (amd64 + arm64)
- **Service support** -- systemd, launchd, Windows NSSM

## Building

```bash
# Single platform
go build -o mcp-memory main.go

# All platforms
./build.sh 1.0.0
```

## License

MIT
