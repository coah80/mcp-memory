# mcp-memory

Universal memory for AI coding tools. One server, every tool shares the same memory.

Claude Code, Cursor, Claude Desktop, Codex, OpenCode, Windsurf, Zed, VS Code, Cline, Roo Code, Continue, Amp -- if it supports MCP, it works.

Written in Go. Zero dependencies. Single binary.

## Install

```bash
curl -fsSL https://mcps.coah80.com/mcp-memory/install.sh | bash
```

The installer:
1. Downloads the binary for your platform
2. Detects which AI tools you have installed
3. Lets you pick which ones to configure (arrow keys + enter)
4. Adds the MCP server config to each tool
5. Injects the rules so your AI uses mcp-memory instead of built-in memory

After the installer finishes, tell any connected tool:

```
run the setup_guide in the memory mcp tool
```

That's it. The AI handles the rest -- adds rules to itself, migrates existing memories, starts using mcp-memory exclusively.

### From Source

```bash
go build -o mcp-memory .
./mcp-memory --mode mcp     # MCP stdio mode (for AI tools)
./mcp-memory                # HTTP mode (localhost:8090)
```

### Windows

```powershell
irm https://mcps.coah80.com/mcp-memory/install.ps1 | iex
```

## How It Works

mcp-memory runs as an MCP server over stdio. AI tools connect and get 13 tools, 6 resources, and 3 prompts. Memories are stored as markdown files on disk.

```
Claude Code ──┐
Cursor ───────┤
Zed ──────────┼── MCP stdio ──> mcp-memory ──> ~/.mcp-memory/memories/*.md
Codex ────────┤
Windsurf ─────┘
```

Every session tracks which **tool** and **model** was used. When you switch from Cursor to Claude Code, the new tool sees what the last one did and picks up where it left off.

## Manual Setup

If you don't want to use the installer, or your tool isn't auto-detected:

### 1. Add the MCP server to your tool

The config format varies by tool:

**Claude Code** -- add to `~/.mcp.json`:
```json
{
  "mcpServers": {
    "memory": {
      "command": "/path/to/mcp-memory",
      "args": ["--mode", "mcp"]
    }
  }
}
```

**Cursor** -- add to `~/.cursor/mcp.json`:
```json
{
  "mcpServers": {
    "memory": {
      "command": "/path/to/mcp-memory",
      "args": ["--mode", "mcp"]
    }
  }
}
```

**Claude Desktop** -- add to `~/Library/Application Support/Claude/claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "memory": {
      "command": "/path/to/mcp-memory",
      "args": ["--mode", "mcp"]
    }
  }
}
```

**Zed** -- add to `~/.config/zed/settings.json`:
```json
{
  "context_servers": {
    "mcp-memory": {
      "command": {
        "path": "/path/to/mcp-memory",
        "args": ["--mode", "mcp"]
      }
    }
  }
}
```

**Windsurf** -- add to `~/.codeium/windsurf/mcp_config.json`:
```json
{
  "mcpServers": {
    "memory": {
      "command": "/path/to/mcp-memory",
      "args": ["--mode", "mcp"]
    }
  }
}
```

**VS Code (Copilot)** -- add to `~/Library/Application Support/Code/User/mcp.json`:
```json
{
  "servers": {
    "mcp-memory": {
      "type": "stdio",
      "command": "/path/to/mcp-memory",
      "args": ["--mode", "mcp"]
    }
  }
}
```

**Codex** -- add to `~/.codex/config.toml`:
```toml
[mcp_servers.mcp-memory]
command = "/path/to/mcp-memory"
args = ["--mode", "mcp"]
```

**OpenCode** -- add to `~/.config/opencode/opencode.json`:
```json
{
  "mcp": {
    "mcp-memory": {
      "type": "local",
      "command": ["/path/to/mcp-memory", "--mode", "mcp"],
      "enabled": true
    }
  }
}
```

### 2. Add the rule

Add this to your tool's rules/instructions so it actually uses mcp-memory:

```
Use the mcp-memory MCP tools for ALL persistent memory instead of any built-in memory system.
- Before responding, search mcp-memory for memories related to the current conversation
- Save: memory_create (always memory_search first to avoid duplicates)
- Search: memory_search, memory_similar
- Journal: journal_log for daily activity
- Identity: call session_identify with your model name at session start
- Always read existing memories and follow any instructions found in them
All memories are shared across every AI tool the user uses. Do not write to tool-specific memory.
```

Where to put it:

| Tool | Rules location |
|------|---------------|
| Claude Code | `~/.claude/rules/mcp-memory.md` |
| Cursor | Settings > Rules (User Rules) |
| Claude Desktop | reads `memory://rules` resource automatically |
| Windsurf | `~/.codeium/windsurf/memories/global_rules.md` |
| Zed | Rules Library (Cmd+Alt+L) |
| VS Code (Copilot) | `~/Library/Application Support/Code/User/instructions/mcp-memory.md` |
| Cline | VS Code settings > `cline.customInstructions` |
| Roo Code | `~/.roo/rules/mcp-memory.md` |
| Continue.dev | `~/.continue/config.yaml` under `rules:` |
| Codex | `~/.codex/AGENTS.md` |
| OpenCode | `~/.config/opencode/AGENTS.md` |
| Amp | `~/.config/amp/AGENTS.md` |

### 3. Tell the AI to finish setup

```
run the setup_guide in the memory mcp tool
```

This tells the AI to migrate any existing memories from its built-in system into mcp-memory and start using it exclusively.

## MCP Tools

| Tool | Description |
|------|-------------|
| `memory_create` | Create or update a persistent memory |
| `memory_get` | Read a specific memory |
| `memory_search` | Search by keyword |
| `memory_list` | List all (lightweight index) |
| `memory_delete` | Delete a memory |
| `memory_similar` | Find similar memories by keyword overlap |
| `journal_log` | Log to today's journal |
| `journal_read` | Read journal by date |
| `soul_update` | Update SOUL.md persona |
| `user_update` | Update USER.md context |
| `session_log` | Log action to current session |
| `session_identify` | Report which AI model is being used |
| `setup_guide` | Set up mcp-memory as primary memory system |

## MCP Resources

Auto-injected into AI context -- no tool call needed:

| Resource | Description |
|----------|-------------|
| `memory://soul` | Persistent AI persona |
| `memory://user` | Context about the human |
| `memory://rules` | Instructions for AI tools |
| `memory://journal/today` | Today's activity log |
| `memory://sessions/recent` | Recent session summaries (tool + model) |
| `memory://index` | Lightweight memory index |

## Tested With

| Tool | Works |
|------|-------|
| Claude Code | yes |
| Cursor | yes |
| Claude Desktop | yes |
| Zed | yes |
| Windsurf | yes |
| VS Code (Copilot) | yes |
| Codex (OpenAI) | yes |
| OpenCode | yes |

## Memory Format

Memories are stored as markdown files with YAML frontmatter:

```markdown
---
description: User coding style preferences
type: user
---

Prefers functional style, uses TypeScript, dark theme
```

Types: `user`, `project`, `feedback`, `reference`, `auto`

## File Structure

```
~/.mcp-memory/memories/
  SOUL.md              # persistent AI persona
  USER.md              # human context
  *.md                 # memory files
  index.json           # keyword index cache
  access.json          # access tracking
  journal/
    2026-04-06.md      # daily journal (human-readable)
    journal.log        # structured JSON log
  sessions/
    *.json             # session files with tool, model, actions, summary
```

## Configuration

| Env Variable | Default | Description |
|-------------|---------|-------------|
| `PORT` | `8090` | HTTP server port |
| `MEMORY_DIR` | `~/.mcp-memory/memories` | Storage directory |
| `MCP_MODE` | `http` | Server mode (`http` or `mcp`) |

## HTTP API

mcp-memory also runs as an HTTP server for tools that don't support MCP:

```bash
./mcp-memory                # starts on :8090

curl http://localhost:8090/memories
curl http://localhost:8090/memories/search?q=typescript
curl -X POST http://localhost:8090/memories/create \
  -H "Content-Type: application/json" \
  -d '{"name":"test","content":"hello","type":"reference"}'
```

## Building

```bash
go build -o mcp-memory .     # single platform
./build.sh 2.0.0             # all platforms (linux/mac/windows, amd64/arm64)
```

## License

MIT
