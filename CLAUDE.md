# LiteTracker MCP Server

This is a Go-based MCP server for LiteTracker project management.

## Available Tools

### Read Operations (v5 API, token auth)
- `get_me` — Returns your user profile (id, name, username, email, projects)
- `list_projects` — Lists all projects you have access to
- `list_stories` — Lists stories with optional filters: `state`, `filter`, `owners`, `owned_by`, `section_type`, `limit`
- `get_story` — Gets a single story with all its comments
- `get_story_comments` — Gets just the comments for a story
- `get_project_activity` — Gets recent activity (defaults to last 7 days)

### Write Operations (internal API, web session auth)
- `create_story` — Creates a new story (title required, optional: description, story_type, estimate, labels)
- `post_comment` — Posts a comment on a story
- `add_label` — Adds a label to a story (idempotent — safe to call if label exists)
- `add_owner` — Adds an owner to a story (preserves existing owners, idempotent)

## Project Structure

```
main.go                      # CLI entrypoint (serve, daemon, sync)
internal/
  api/
    client.go                # v5 API client (token auth, read + create story)
    webclient.go             # Internal API client (session auth, comments/labels/owners)
    types.go                 # Shared data types
  mcp/
    server.go                # MCP tool definitions and handlers
  config/
    config.go                # Environment-based configuration
  db/
    db.go                    # DuckDB storage (daemon/sync only)
  sync/
    sync.go                  # Story/comment sync logic (daemon/sync only)
  notify/
    notify.go                # macOS notifications (daemon only)
```

## Key Parameters

- `project_id` — LiteTracker project ID (number, required for most tools)
- `story_id` — Story ID (number)
- `user_id` — User ID for add_owner (number)
- `filter` — Tracker search syntax string (e.g., `"state:started label:bug"`)
- `state` — Story state: `started`, `unstarted`, `delivered`, `accepted`, `rejected`

## Build

```bash
go build -ldflags="-s -w" -o litetracker .
```
