# LiteTracker MCP Server

A Go-based [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server for [LiteTracker](https://app.litetracker.com) project management. Provides 10 tools for managing stories, comments, labels, and owners directly from Claude Code or Claude Desktop.

## Features

| Tool | Description |
|------|-------------|
| `get_me` | Get current authenticated user info |
| `list_projects` | List all projects |
| `list_stories` | List stories with filters (state, owner, labels) |
| `get_story` | Get a story with its comments |
| `get_story_comments` | Get comments for a story |
| `create_story` | Create a new story |
| `post_comment` | Post a comment on a story |
| `add_label` | Add a label to a story |
| `add_owner` | Add an owner to a story (preserves existing) |
| `get_project_activity` | Get recent project activity |

## Prerequisites

- [Go 1.25+](https://go.dev/dl/) installed
- [DuckDB](https://duckdb.org/docs/stable/guides/overview) installed (required for daemon/sync modes)
- A [LiteTracker](https://app.litetracker.com) account with an API token

### Install DuckDB

```bash
curl https://install.duckdb.org | sh
```

## Install in Claude Code

```bash
claude mcp add litetracker \
  -e LITETRACKER_TOKEN=your_api_token \
  -e LITETRACKER_EMAIL=your_email \
  -e LITETRACKER_PASSWORD=your_password \
  -e LITETRACKER_USER_ID=your_user_id \
  -- go run github.com/MelianLabs/litetracker-mcp@latest serve
```

> **Note:** `LITETRACKER_EMAIL`, `LITETRACKER_PASSWORD`, and `LITETRACKER_USER_ID` are required for write operations (comments, labels, owners). Read-only usage only needs `LITETRACKER_TOKEN`.

## Install in Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "litetracker": {
      "command": "go",
      "args": ["run", "github.com/MelianLabs/litetracker-mcp@latest", "serve"],
      "env": {
        "LITETRACKER_TOKEN": "your_api_token",
        "LITETRACKER_EMAIL": "your_email",
        "LITETRACKER_PASSWORD": "your_password",
        "LITETRACKER_USER_ID": "your_user_id"
      }
    }
  }
}
```

## Install from Source

```bash
git clone https://github.com/MelianLabs/litetracker-mcp.git
cd litetracker-mcp
cp .env.example .env  # Edit with your credentials
go build -ldflags="-s -w" -o litetracker .

# Register with Claude Code
claude mcp add litetracker -- ./litetracker serve
```

## Configuration

All configuration is via environment variables. You can also use a `.env` file (loaded from the current directory or `~/litetracker-go/.env`).

| Variable | Required | Description |
|----------|----------|-------------|
| `LITETRACKER_TOKEN` | Yes | API token from Profile > API Tokens |
| `LITETRACKER_EMAIL` | For writes | Login email for web session auth |
| `LITETRACKER_PASSWORD` | For writes | Login password for web session auth |
| `LITETRACKER_USER_ID` | For writes | Your user ID (for comment attribution) |
| `LITETRACKER_USERNAME` | No | Display name (for daemon mention detection) |
| `LITETRACKER_PROJECT_IDS` | For daemon | Comma-separated project IDs |
| `POLL_INTERVAL_MS` | No | Daemon poll interval (default: 300000ms) |
| `LITETRACKER_BASE_URL` | No | API base URL (default: `https://app.litetracker.com/services/v5`) |
| `LITETRACKER_WEB_URL` | No | Web base URL (default: `https://app.litetracker.com`) |
| `LITETRACKER_DATA_DIR` | No | Data directory for daemon/sync DuckDB storage |
| `LITETRACKER_ENV_FILE` | No | Custom path to .env file |

### Getting Your Credentials

1. **API Token**: Log in to LiteTracker > Profile > API Tokens > Create New Token
2. **User ID**: Run the `get_me` tool after setting up your token, or check your profile URL
3. **Email/Password**: Your LiteTracker login credentials (needed because the API doesn't support write operations for comments/labels/owners)

## Subcommands

| Command | Description |
|---------|-------------|
| `serve` | Start MCP server (stdio transport) |
| `daemon` | Background daemon: polls for activity, syncs to DuckDB, sends macOS notifications |
| `sync` | One-shot sync: fetches all stories/comments to DuckDB |

The `serve` command is all you need for Claude Code/Desktop integration. The `daemon` and `sync` commands are optional power-user features that maintain a local DuckDB cache.

The DuckDB database schema (tables, indexes, views) is created automatically on first run of `daemon` or `sync` — no manual setup required.

## Architecture

The server uses two authentication methods:

- **v5 API** (token auth): Read operations and story creation
- **Internal `/api/v1/`** (web session auth): Write operations (comments, labels, owners) — auto-login with session expiry retry

This dual approach is necessary because LiteTracker's public API doesn't support write operations for comments, labels, or owner assignments.

