package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Token          string
	BaseURL        string
	WebURL         string
	Username       string
	Email          string
	Password       string
	ProjectIDs     []int
	UserID         int
	PollIntervalMs int
	DataDir        string
	ProjectDir     string
}

var C Config

func Init() error {
	// Try to load .env from multiple locations (first match wins, env vars take precedence)
	if envFile := os.Getenv("LITETRACKER_ENV_FILE"); envFile != "" {
		loadEnvFile(envFile)
	} else {
		// Try current directory, then ~/litetracker-go/.env
		loadEnvFile(".env")
		if home, err := os.UserHomeDir(); err == nil {
			loadEnvFile(filepath.Join(home, "litetracker-go", ".env"))
		}
	}

	C.Token = os.Getenv("LITETRACKER_TOKEN")
	if C.Token == "" || C.Token == "your_api_token_here" {
		return fmt.Errorf("LITETRACKER_TOKEN is required. Set it via environment variable or .env file")
	}

	C.BaseURL = envOrDefault("LITETRACKER_BASE_URL", "https://app.litetracker.com/services/v5")
	C.WebURL = envOrDefault("LITETRACKER_WEB_URL", "https://app.litetracker.com")
	C.Username = os.Getenv("LITETRACKER_USERNAME")
	C.Email = os.Getenv("LITETRACKER_EMAIL")
	C.Password = os.Getenv("LITETRACKER_PASSWORD")
	C.UserID = envInt("LITETRACKER_USER_ID")
	C.PollIntervalMs = envIntOrDefault("POLL_INTERVAL_MS", 300000)

	ids := os.Getenv("LITETRACKER_PROJECT_IDS")
	for _, s := range strings.Split(ids, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		C.ProjectIDs = append(C.ProjectIDs, id)
	}

	return nil
}

// InitDataDir sets up the data directory for daemon/sync modes.
// Not needed for MCP serve mode.
func InitDataDir() error {
	if dir := os.Getenv("LITETRACKER_DATA_DIR"); dir != "" {
		C.DataDir = dir
		C.ProjectDir = dir
	} else if home, err := os.UserHomeDir(); err == nil {
		C.ProjectDir = filepath.Join(home, "litetracker-go")
		C.DataDir = filepath.Join(C.ProjectDir, "data")
	} else {
		return fmt.Errorf("cannot determine data directory: set LITETRACKER_DATA_DIR")
	}

	if err := os.MkdirAll(C.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	return nil
}

func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

func envIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
