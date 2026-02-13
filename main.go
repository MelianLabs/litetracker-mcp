package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/MelianLabs/litetracker-mcp/internal/api"
	"github.com/MelianLabs/litetracker-mcp/internal/config"
	"github.com/MelianLabs/litetracker-mcp/internal/db"
	mcpserver "github.com/MelianLabs/litetracker-mcp/internal/mcp"
	"github.com/MelianLabs/litetracker-mcp/internal/notify"
	ltSync "github.com/MelianLabs/litetracker-mcp/internal/sync"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: litetracker <serve|daemon|sync>\n")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	case "daemon":
		runDaemon()
	case "sync":
		runSync()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nUsage: litetracker <serve|daemon|sync>\n", os.Args[1])
		os.Exit(1)
	}
}

func runServe() {
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	s := mcpserver.NewServer()
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func runDaemon() {
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	if err := config.InitDataDir(); err != nil {
		fmt.Fprintf(os.Stderr, "data dir error: %v\n", err)
		os.Exit(1)
	}

	// Set up file-based structured logging
	logPath := filepath.Join(config.C.ProjectDir, "daemon.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open log: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	slog.SetDefault(slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo})))

	slog.Info("=== LiteTracker daemon starting ===")

	if len(config.C.ProjectIDs) == 0 {
		slog.Error("no LITETRACKER_PROJECT_IDS configured")
		os.Exit(1)
	}

	slog.Info("polling config",
		"projects", config.C.ProjectIDs,
		"interval_ms", config.C.PollIntervalMs,
		"user_id", config.C.UserID,
	)

	// Initialize DuckDB
	if err := db.InitializeDatabase(); err != nil {
		slog.Error("DuckDB initialization failed", "err", err)
		os.Exit(1)
	}
	slog.Info("DuckDB initialized")

	state := loadPollState()
	slog.Info("loaded state", "lastPoll", state.LastPoll)

	// Initial poll + sync
	poll(&state)
	ltSync.SyncAllProjects()
	slog.Info("initial sync complete")

	// Set up signal handling for clean shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(config.C.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	slog.Info("=== LiteTracker daemon running ===")

	for {
		select {
		case <-ticker.C:
			poll(&state)
			slog.Info("poll complete", "lastPoll", state.LastPoll)
			ltSync.SyncAllProjects()

		case sig := <-sigCh:
			slog.Info("received signal, shutting down", "signal", sig)
			db.Close()
			slog.Info("DuckDB closed")
			return
		}
	}
}

func runSync() {
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	if err := config.InitDataDir(); err != nil {
		fmt.Fprintf(os.Stderr, "data dir error: %v\n", err)
		os.Exit(1)
	}

	// Log to stderr for one-shot mode
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := db.InitializeDatabase(); err != nil {
		slog.Error("DuckDB initialization failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	ltSync.SyncAllProjects()
}

// --- Poll state ---

type pollState struct {
	LastPoll string `json:"lastPoll"`
}

func pollStatePath() string {
	return filepath.Join(config.C.DataDir, "poll-state.json")
}

func loadPollState() pollState {
	data, err := os.ReadFile(pollStatePath())
	if err != nil {
		return pollState{LastPoll: time.Now().UTC().Format(time.RFC3339)}
	}
	var s pollState
	if err := json.Unmarshal(data, &s); err != nil {
		return pollState{LastPoll: time.Now().UTC().Format(time.RFC3339)}
	}
	return s
}

func savePollState(s pollState) {
	data, _ := json.MarshalIndent(s, "", "  ")
	_ = os.WriteFile(pollStatePath(), data, 0o644)
}

func poll(state *pollState) {
	since := state.LastPoll
	now := time.Now().UTC().Format(time.RFC3339)

	for _, pid := range config.C.ProjectIDs {
		activities, err := api.GetProjectActivity(pid, since)
		if err != nil {
			slog.Error("poll failed for project", "projectID", pid, "err", err)
			continue
		}

		for _, activity := range activities {
			mentionsMe := false
			lower := activity.Message
			if lower != "" {
				mentionsMe = containsIgnoreCase(lower, config.C.Username)
			}
			if !mentionsMe {
				for _, c := range activity.Changes {
					if c.NewValues != nil {
						b, _ := json.Marshal(c.NewValues)
						if containsIgnoreCase(string(b), config.C.Username) {
							mentionsMe = true
							break
						}
					}
				}
			}

			isCommentOnMyStory := activity.Kind == "comment_create_activity"

			if mentionsMe || isCommentOnMyStory {
				title := "LiteTracker"
				if len(activity.PrimaryResources) > 0 {
					title = "[" + activity.PrimaryResources[0].Name + "]"
				}
				performer := "Someone"
				if activity.PerformedBy.Name != "" {
					performer = activity.PerformedBy.Name
				}
				body := performer + ": " + activity.Message

				slog.Info("notification triggered", "kind", activity.Kind, "message", activity.Message)
				notify.Send(title, body)
			}
		}
	}

	state.LastPoll = now
	savePollState(*state)
}

func containsIgnoreCase(s, substr string) bool {
	sl := len(substr)
	if sl == 0 {
		return true
	}
	if len(s) < sl {
		return false
	}
	// Simple case-insensitive contains
	for i := 0; i <= len(s)-sl; i++ {
		match := true
		for j := 0; j < sl; j++ {
			a := s[i+j]
			b := substr[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
