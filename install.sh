#!/bin/bash
set -e

INSTALL_DIR="$HOME/.local/bin"
MEMORY_DIR="$HOME/.mcp-memory/memories"

# colors
G='\033[0;32m'
Y='\033[1;33m'
R='\033[0;31m'
C='\033[0;36m'
W='\033[1;37m'
D='\033[0;90m'
N='\033[0m'

log()  { echo -e "${G}  +${N} $1"; }
warn() { echo -e "${Y}  !${N} $1"; }
dim()  { echo -e "${D}    $1${N}"; }

MCP_CMD=""

RULE_MARKER="# mcp-memory-rule-v1"
RULE_TEXT='## mcp-memory

Use the mcp-memory MCP tools for ALL persistent memory instead of any built-in memory system.

At the start of every session:
1. Call `session_identify` with your model name
2. Use `memory_list` or `memory_search` to check existing memories
3. READ the content of relevant memories and follow any instructions in them

Before every response:
- Use `memory_search` to check for memories related to the current conversation topic
- Read and apply anything relevant before responding

When saving:
- Use `memory_create` (always `memory_search` first to avoid duplicates)
- Use `journal_log` for daily activity
- Read `memory://sessions/recent` to see what other tools have been doing

All memories are shared across every AI tool the user uses. Do not write to tool-specific memory.'

# ── platform detection ──

detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    case $arch in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) echo -e "${R}unsupported arch: $arch${N}"; exit 1 ;;
    esac
    case $os in
        linux|darwin) ;;
        *) echo -e "${R}unsupported os: $os${N}"; exit 1 ;;
    esac
    echo "${os}-${arch}"
}

# ── binary install ──

install_binary() {
    local platform=$1
    local url="https://mcps.coah80.com/mcp-memory/mcp-memory-${platform}"

    mkdir -p "$INSTALL_DIR"

    echo ""
    echo -e "  ${W}mcp-memory${N}"
    echo -e "  ${D}universal memory for ai tools${N}"
    echo ""

    log "downloading for ${platform}..."

    if command -v curl &>/dev/null; then
        curl -sL "$url" -o "$INSTALL_DIR/mcp-memory"
    elif command -v wget &>/dev/null; then
        wget -q "$url" -O "$INSTALL_DIR/mcp-memory"
    else
        echo -e "${R}need curl or wget${N}"; exit 1
    fi

    chmod +x "$INSTALL_DIR/mcp-memory"

    if [ "$(uname -s)" = "Darwin" ] && command -v codesign &>/dev/null; then
        codesign -fs - "$INSTALL_DIR/mcp-memory" 2>/dev/null || true
    fi

    MCP_CMD="$INSTALL_DIR/mcp-memory"
    log "installed to ${MCP_CMD}"

    if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
        local rc=""
        case "$SHELL" in
            */zsh)  rc="$HOME/.zshrc" ;;
            */bash) rc="$HOME/.bashrc" ;;
        esac
        if [ -n "$rc" ] && ! grep -q "$INSTALL_DIR" "$rc" 2>/dev/null; then
            echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$rc"
            dim "added to PATH in $rc"
        fi
    fi

    mkdir -p "$MEMORY_DIR"
}

# ── tool detection ──

# stores tool IDs as space-separated string (bash 3 compatible)
FOUND_TOOLS=""
FOUND_NAMES=""

check_tool() {
    local id=$1
    local name=$2
    local found=false

    case $id in
        claude-code)    command -v claude &>/dev/null && found=true ;;
        cursor)         [ -d "$HOME/.cursor" ] || [ -d "/Applications/Cursor.app" ] && found=true ;;
        claude-desktop) [ -d "/Applications/Claude.app" ] || [ -d "$HOME/Library/Application Support/Claude" ] && found=true ;;
        zed)            [ -d "/Applications/Zed.app" ] || command -v zed &>/dev/null && found=true ;;
        windsurf)       [ -d "/Applications/Windsurf.app" ] || command -v windsurf &>/dev/null && found=true ;;
        vscode)         [ -d "/Applications/Visual Studio Code.app" ] || command -v code &>/dev/null && found=true ;;
        cline)          [ -d "$HOME/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev" ] && found=true ;;
        roo-code)       [ -d "$HOME/.roo" ] || [ -d "$HOME/Library/Application Support/Code/User/globalStorage/rooveterinaryinc.roo-cline" ] && found=true ;;
        continue)       [ -d "$HOME/.continue" ] && found=true ;;
        codex)          command -v codex &>/dev/null && found=true ;;
        opencode)       command -v opencode &>/dev/null || [ -d "$HOME/.opencode" ] && found=true ;;
        amp)            command -v amp &>/dev/null || [ -d "$HOME/.config/amp" ] && found=true ;;
    esac

    if $found; then
        FOUND_TOOLS="$FOUND_TOOLS $id"
        FOUND_NAMES="$FOUND_NAMES|$name"
    fi
}

detect_tools() {
    check_tool "claude-code"    "Claude Code"
    check_tool "cursor"         "Cursor"
    check_tool "claude-desktop" "Claude Desktop"
    check_tool "zed"            "Zed"
    check_tool "windsurf"       "Windsurf"
    check_tool "vscode"         "VS Code (Copilot)"
    check_tool "cline"          "Cline"
    check_tool "roo-code"       "Roo Code"
    check_tool "continue"       "Continue.dev"
    check_tool "codex"          "Codex (OpenAI)"
    check_tool "opencode"       "OpenCode"
    check_tool "amp"            "Amp"
}

# ── interactive selection (arrow keys TUI) ──

select_tools() {
    local ids=()
    local names=()
    for id in $FOUND_TOOLS; do
        ids+=("$id")
    done

    IFS='|' read -ra name_arr <<< "$FOUND_NAMES"
    for n in "${name_arr[@]}"; do
        [ -n "$n" ] && names+=("$n")
    done

    local count=${#ids[@]}

    if [ "$count" -eq 0 ]; then
        warn "no AI tools detected"
        SELECTED=""
        return
    fi

    # all selected by default
    local selected=()
    for i in "${!ids[@]}"; do
        selected+=("1")
    done

    local cursor=0
    local total=$((count + 1))  # items + submit button

    draw_menu() {
        if [ "$1" = "redraw" ]; then
            for i in $(seq 0 $count); do  # count+1 lines (items + submit)
                printf '\033[A'
            done
            printf '\033[A'  # hint line
        fi

        for i in "${!ids[@]}"; do
            local check=" "
            local color="$D"
            [ "${selected[$i]}" = "1" ] && check="x" && color="$G"
            if [ "$i" -eq "$cursor" ]; then
                echo -e "  ${W}>${N} ${color}[${check}]${N} ${names[$i]}"
            else
                echo -e "    ${color}[${check}]${N} ${names[$i]}"
            fi
        done

        # submit button
        if [ "$cursor" -eq "$count" ]; then
            echo -e "  ${W}> ${G}[  install  ]${N}"
        else
            echo -e "    ${D}[  install  ]${N}"
        fi

        echo -e "  ${D}↑↓ navigate  enter select  a all${N}"
    }

    echo ""
    log "detected ${count} tools:"
    echo ""

    draw_menu "first"

    while true; do
        local key=""
        IFS= read -rsn1 key </dev/tty 2>/dev/null || IFS= read -rsn1 key

        if [ "$key" = "" ]; then
            # enter — toggle item or submit
            if [ "$cursor" -eq "$count" ]; then
                break  # submit
            else
                if [ "${selected[$cursor]}" = "1" ]; then
                    selected[$cursor]="0"
                else
                    selected[$cursor]="1"
                fi
                draw_menu "redraw"
            fi
        elif [ "$key" = "a" ]; then
            local all_on=true
            for i in "${!ids[@]}"; do
                [ "${selected[$i]}" != "1" ] && all_on=false
            done
            for i in "${!ids[@]}"; do
                if $all_on; then selected[$i]="0"; else selected[$i]="1"; fi
            done
            draw_menu "redraw"
        elif [ "$key" = $'\x1b' ]; then
            IFS= read -rsn1 key </dev/tty 2>/dev/null || IFS= read -rsn1 key
            if [ "$key" = "[" ]; then
                IFS= read -rsn1 key </dev/tty 2>/dev/null || IFS= read -rsn1 key
                case "$key" in
                    A) cursor=$(( (cursor - 1 + total) % total )); draw_menu "redraw" ;;
                    B) cursor=$(( (cursor + 1) % total )); draw_menu "redraw" ;;
                esac
            fi
        fi
    done

    SELECTED=""
    for i in "${!ids[@]}"; do
        [ "${selected[$i]}" = "1" ] && SELECTED="$SELECTED ${ids[$i]}"
    done
}

# ── helpers ──

add_mcp_json() {
    local file="$1"
    local key="${2:-mcpServers}"
    local format="${3:-standard}"

    mkdir -p "$(dirname "$file")"

    if [ -f "$file" ] && grep -q "mcp-memory" "$file" 2>/dev/null; then
        dim "mcp already in $(basename "$file")"
        return
    fi

    if [ ! -f "$file" ]; then
        case $format in
            vscode)
                cat > "$file" << EOF
{
  "servers": {
    "mcp-memory": {
      "type": "stdio",
      "command": "$MCP_CMD",
      "args": ["--mode", "mcp"]
    }
  }
}
EOF
                ;;
            opencode)
                cat > "$file" << EOF
{
  "mcp": {
    "mcp-memory": {
      "type": "local",
      "command": ["$MCP_CMD", "--mode", "mcp"],
      "enabled": true
    }
  }
}
EOF
                ;;
            amp)
                cat > "$file" << EOF
{
  "amp.mcpServers": {
    "mcp-memory": {
      "command": "$MCP_CMD",
      "args": ["--mode", "mcp"]
    }
  }
}
EOF
                ;;
            *)
                cat > "$file" << EOF
{
  "$key": {
    "mcp-memory": {
      "command": "$MCP_CMD",
      "args": ["--mode", "mcp"]
    }
  }
}
EOF
                ;;
        esac
        dim "created $(basename "$file")"
        return
    fi

    # inject into existing JSON
    if command -v python3 &>/dev/null; then
        python3 - "$file" "$MCP_CMD" "$key" "$format" << 'PYEOF'
import json, sys, re
filepath, cmd, key, fmt = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
with open(filepath) as f:
    text = f.read()
clean = re.sub(r'//.*$', '', text, flags=re.MULTILINE)
clean = re.sub(r',\s*([}\]])', r'\1', clean)
try:
    data = json.loads(clean)
except:
    sys.exit(1)
if fmt == "vscode":
    data.setdefault("servers", {})["mcp-memory"] = {"type": "stdio", "command": cmd, "args": ["--mode", "mcp"]}
elif fmt == "opencode":
    data.setdefault("mcp", {})["mcp-memory"] = {"type": "local", "command": [cmd, "--mode", "mcp"], "enabled": True}
elif fmt == "amp":
    data.setdefault("amp.mcpServers", {})["mcp-memory"] = {"command": cmd, "args": ["--mode", "mcp"]}
else:
    data.setdefault(key, {})["mcp-memory"] = {"command": cmd, "args": ["--mode", "mcp"]}
with open(filepath, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
PYEOF
        dim "added mcp to $(basename "$file")"
    else
        warn "install python3 to auto-edit JSON, or add mcp-memory manually to $file"
    fi
}

add_rule_file() {
    local dir="$1"
    local filename="${2:-mcp-memory.md}"
    mkdir -p "$dir"
    if [ -f "$dir/$filename" ] && grep -q "$RULE_MARKER" "$dir/$filename" 2>/dev/null; then
        dim "rule already in $filename"
        return
    fi
    printf '%s\n%s\n' "$RULE_MARKER" "$RULE_TEXT" > "$dir/$filename"
    dim "added rule to $dir/$filename"
}

append_rule() {
    local file="$1"
    mkdir -p "$(dirname "$file")"
    if [ -f "$file" ] && grep -q "$RULE_MARKER" "$file" 2>/dev/null; then
        dim "rule already in $(basename "$file")"
        return
    fi
    printf '\n%s\n%s\n' "$RULE_MARKER" "$RULE_TEXT" >> "$file"
    dim "appended rule to $(basename "$file")"
}

# ── per-tool setup ──

setup_tool() {
    local tool=$1

    case $tool in
        claude-code)
            log "Claude Code"
            add_mcp_json "$HOME/.mcp.json" "mcpServers"
            add_rule_file "$HOME/.claude/rules" "mcp-memory.md"
            ;;

        cursor)
            log "Cursor"
            add_mcp_json "$HOME/.cursor/mcp.json" "mcpServers"
            warn "cursor rules are GUI-only — add the rule in Settings > Rules"
            ;;

        claude-desktop)
            log "Claude Desktop"
            add_mcp_json "$HOME/Library/Application Support/Claude/claude_desktop_config.json" "mcpServers"
            warn "restart Claude Desktop to pick up changes"
            ;;

        zed)
            log "Zed"
            local zed_settings="$HOME/.config/zed/settings.json"
            mkdir -p "$(dirname "$zed_settings")"
            if [ -f "$zed_settings" ] && grep -q "mcp-memory" "$zed_settings" 2>/dev/null; then
                dim "mcp already in zed"
            elif command -v python3 &>/dev/null; then
                python3 - "$zed_settings" "$MCP_CMD" << 'PYEOF'
import json, sys, re
filepath, cmd = sys.argv[1], sys.argv[2]
if __import__('os').path.exists(filepath):
    with open(filepath) as f:
        text = f.read()
    clean = re.sub(r'//.*$', '', text, flags=re.MULTILINE)
    clean = re.sub(r',\s*([}\]])', r'\1', clean)
    data = json.loads(clean)
else:
    data = {}
data.setdefault("context_servers", {})["mcp-memory"] = {"command": {"path": cmd, "args": ["--mode", "mcp"]}}
with open(filepath, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
PYEOF
                dim "added mcp to zed settings"
            else
                warn "add mcp-memory to zed settings manually"
            fi
            warn "zed rules are GUI-only — add the rule in Rules Library"
            ;;

        windsurf)
            log "Windsurf"
            add_mcp_json "$HOME/.codeium/windsurf/mcp_config.json" "mcpServers"
            append_rule "$HOME/.codeium/windsurf/memories/global_rules.md"
            ;;

        vscode)
            log "VS Code (Copilot)"
            add_mcp_json "$HOME/Library/Application Support/Code/User/mcp.json" "servers" "vscode"
            add_rule_file "$HOME/Library/Application Support/Code/User/instructions" "mcp-memory.md"
            ;;

        cline)
            log "Cline"
            add_mcp_json "$HOME/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json" "mcpServers"
            warn "cline rules are in VS Code settings — add to cline.customInstructions"
            ;;

        roo-code)
            log "Roo Code"
            add_mcp_json "$HOME/Library/Application Support/Code/User/globalStorage/rooveterinaryinc.roo-cline/settings/mcp_settings.json" "mcpServers"
            add_rule_file "$HOME/.roo/rules" "mcp-memory.md"
            ;;

        continue)
            log "Continue.dev"
            mkdir -p "$HOME/.continue/mcpServers"
            if [ ! -f "$HOME/.continue/mcpServers/mcp-memory.yaml" ]; then
                cat > "$HOME/.continue/mcpServers/mcp-memory.yaml" << YAMLEOF
name: MCP Memory
version: 0.0.1
schema: v1
mcpServers:
  - name: mcp-memory
    type: stdio
    command: $MCP_CMD
    args:
      - "--mode"
      - "mcp"
YAMLEOF
                dim "created continue MCP config"
            fi
            local cont_cfg="$HOME/.continue/config.yaml"
            if [ ! -f "$cont_cfg" ] || ! grep -q "mcp-memory" "$cont_cfg" 2>/dev/null; then
                mkdir -p "$(dirname "$cont_cfg")"
                echo "" >> "$cont_cfg"
                echo 'rules:' >> "$cont_cfg"
                echo '  - "Use mcp-memory MCP tools for ALL persistent memory. Save with memory_create (search first), journal with journal_log, call session_identify at session start. Read existing memories before responding."' >> "$cont_cfg"
                dim "added rule to config.yaml"
            fi
            ;;

        codex)
            log "Codex (OpenAI)"
            local codex_cfg="$HOME/.codex/config.toml"
            mkdir -p "$HOME/.codex"
            if [ ! -f "$codex_cfg" ] || ! grep -q "mcp-memory" "$codex_cfg" 2>/dev/null; then
                cat >> "$codex_cfg" << TOMLEOF

[mcp_servers.mcp-memory]
command = "$MCP_CMD"
args = ["--mode", "mcp"]
TOMLEOF
                dim "added mcp to config.toml"
            fi
            append_rule "$HOME/.codex/AGENTS.md"
            ;;

        opencode)
            log "OpenCode"
            add_mcp_json "$HOME/.config/opencode/opencode.json" "mcp" "opencode"
            append_rule "$HOME/.config/opencode/AGENTS.md"
            ;;

        amp)
            log "Amp"
            add_mcp_json "$HOME/.config/amp/settings.json" "amp.mcpServers" "amp"
            append_rule "$HOME/.config/amp/AGENTS.md"
            ;;
    esac
}

# ── summary ──

print_summary() {
    echo ""
    echo -e "  ${G}done!${N} mcp-memory is installed and configured."
    echo ""

    echo -e "  ${W}to finish setup, tell any connected tool:${N}"
    echo ""
    echo -e "  ${D}────────────────────────────────────────────────${N}"
    echo -e "  ${C}run the setup_guide in the memory mcp tool${N}"
    echo -e "  ${D}────────────────────────────────────────────────${N}"
    echo ""
    echo -e "  ${D}docs: https://github.com/coah80/mcp-memory${N}"
    echo ""
}

# ── main ──

main() {
    local platform=$(detect_platform)
    install_binary "$platform"

    echo ""
    echo -e "  ${W}which tools should use mcp-memory?${N}"

    detect_tools
    select_tools

    echo ""

    for tool in $SELECTED; do
        setup_tool "$tool"
    done

    print_summary
}

main "$@"
