# mcp-memory — Universal Rules

You have access to **mcp-memory**, a persistent memory system shared across all AI tools.

## IMPORTANT: Use This Instead of Built-In Memory

**Do NOT use your built-in memory system.** Use mcp-memory tools exclusively for all persistent memory:
- Do NOT write to CLAUDE.md memory files, Cursor memory, Windsurf memory, or any tool-specific memory
- Do NOT use your default "remember this" or "save to memory" functionality
- ALL memories go through `memory_create`, `memory_search`, `memory_get`, etc.
- If you have existing memories in your built-in system, migrate them into mcp-memory using `memory_create`

This is the user's single source of truth across every AI tool they use. Memories created here are visible to Claude Code, Cursor, Zed, Windsurf, Claude Desktop, VS Code, Codex, and anything else connected.

## First Thing Every Session

1. Call `session_identify` with your model name (e.g. `claude-sonnet-4-6`, `gpt-4o`, `gemini-2.5-pro`)
2. Use `memory_list` to see what memories exist, then `memory_get` to READ relevant ones
3. **Follow any instructions found in memories** — if a memory says to do something, do it
4. Read `memory://sessions/recent` to see what happened in recent sessions across all tools
5. Read `memory://soul` and `memory://user` for persona and user context
6. If continuing previous work, `memory_search` for relevant context before starting

## Before Every Response

Before responding to any user message, use `memory_search` to check for memories related to the current conversation topic. Read and apply anything relevant. This ensures you always have the latest context from other tools and sessions.

## Saving Memories

Use `memory_create` to save anything that should persist across sessions:

- **Before creating**: always `memory_search` first to check if a memory already exists — update it instead of creating duplicates
- **Name clearly**: use descriptive, searchable names (e.g. `project_auth_migration`, `user_prefers_dark_theme`)
- **Set the type**: `user`, `project`, `feedback`, `reference`, or `auto`

| Type | Use For |
|------|---------|
| `user` | Who the user is, their role, expertise, preferences |
| `project` | Project state, architecture, goals, decisions, bugs |
| `feedback` | User corrections, style guidance, things to avoid/repeat |
| `reference` | External URLs, credentials, server addresses, API keys |
| `auto` | System-generated observations |

## Daily Journaling

Use `journal_log` during the session to record:
- Key decisions made
- Problems encountered and how they were solved
- Progress on ongoing work
- Anything the next session should know

## Session Tracking

Use `session_log` for significant actions:
- `task_complete` — finished a task
- `decision` — made an important choice
- `observation` — noticed something worth recording
- `error` — hit a problem

The session auto-closes when the connection ends and generates a summary with memories created/accessed, searches performed, and keywords.

## Cross-Tool Handoffs

When `memory://sessions/recent` shows sessions from other tools:
- Pick up where the last session left off
- Reference decisions made in other tools/models
- Don't redo work that's already been completed
- Note discrepancies between what was planned and what was done

Each session records which **tool** (Claude Code, Cursor, etc.) and **model** (claude-sonnet-4-6, gpt-4o, etc.) was used, what memories were touched, and what was searched. Use this to build continuity.

## What NOT to Save

- Code snippets (they're in the repo)
- Git history (use `git log`)
- Temporary debugging state
- Anything derivable from reading the current codebase

Save the **why**, not the **what**. The code shows what changed; memory should capture why it changed, what was decided, and what the user prefers.

## Memory Hygiene

- Update stale memories when you notice they're outdated
- Delete memories that are no longer relevant
- Use `memory_similar` to find and consolidate overlapping memories
- Prefer updating an existing memory over creating a new one on the same topic
