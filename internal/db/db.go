package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/MelianLabs/litetracker-mcp/internal/config"

	_ "github.com/duckdb/duckdb-go/v2"
)

const schemaVersion = 2

var conn *sql.DB

func dbPath() string    { return filepath.Join(config.C.DataDir, "litetracker.duckdb") }
func snapPath() string  { return filepath.Join(config.C.DataDir, "litetracker-snapshot.duckdb") }

func InitializeDatabase() error {
	var err error
	conn, err = sql.Open("duckdb", dbPath())
	if err != nil {
		return fmt.Errorf("open duckdb: %w", err)
	}

	if err := migrateSchema(); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}

	if err := createTables(); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	if err := createIndexes(); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}

	if err := createViews(); err != nil {
		return fmt.Errorf("create views: %w", err)
	}

	return nil
}

func Close() {
	if conn != nil {
		conn.Close()
		conn = nil
	}
}

func migrateSchema() error {
	currentVersion := getSchemaVersion()
	if currentVersion >= schemaVersion {
		return nil
	}

	slog.Info("migrating schema", "from", currentVersion, "to", schemaVersion)
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS comments",
		"DROP TABLE IF EXISTS stories",
		"DROP TABLE IF EXISTS schema_version",
		"CREATE TABLE schema_version (version INTEGER NOT NULL)",
	} {
		if _, err := conn.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	_, err := conn.Exec("INSERT INTO schema_version VALUES (?)", schemaVersion)
	return err
}

func getSchemaVersion() int {
	var v int
	err := conn.QueryRow("SELECT version FROM schema_version LIMIT 1").Scan(&v)
	if err != nil {
		return 0
	}
	return v
}

func createTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS stories (
			id INTEGER PRIMARY KEY,
			project_id INTEGER NOT NULL,
			title VARCHAR NOT NULL,
			description VARCHAR,
			story_type VARCHAR,
			current_state VARCHAR,
			estimate INTEGER,
			priority VARCHAR,
			url VARCHAR,
			requested_by_id INTEGER,
			owner_names VARCHAR,
			label_names VARCHAR,
			is_mine BOOLEAN DEFAULT false,
			mentions_me BOOLEAN DEFAULT false,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			synced_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY,
			story_id INTEGER NOT NULL,
			project_id INTEGER NOT NULL,
			text VARCHAR,
			person_id INTEGER,
			person_name VARCHAR,
			mentions_me BOOLEAN DEFAULT false,
			created_at TIMESTAMP,
			synced_at TIMESTAMP NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := conn.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func createIndexes() error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_stories_mine ON stories (is_mine)",
		"CREATE INDEX IF NOT EXISTS idx_stories_state ON stories (current_state)",
		"CREATE INDEX IF NOT EXISTS idx_stories_project ON stories (project_id)",
		"CREATE INDEX IF NOT EXISTS idx_stories_updated ON stories (updated_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_stories_mine_state ON stories (is_mine, current_state)",
		"CREATE INDEX IF NOT EXISTS idx_comments_story ON comments (story_id)",
		"CREATE INDEX IF NOT EXISTS idx_comments_mentions ON comments (mentions_me)",
		"CREATE INDEX IF NOT EXISTS idx_comments_created ON comments (created_at DESC)",
	}
	for _, s := range indexes {
		if _, err := conn.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func createViews() error {
	views := []string{
		`CREATE OR REPLACE VIEW my_stories AS
		SELECT id, title, story_type, current_state, estimate, priority,
		       owner_names, label_names, url, mentions_me, created_at, updated_at
		FROM stories WHERE is_mine = true
		ORDER BY updated_at DESC`,

		`CREATE OR REPLACE VIEW my_active_stories AS
		SELECT id, title, story_type, current_state, estimate, priority,
		       owner_names, label_names, url, mentions_me, created_at, updated_at
		FROM stories WHERE is_mine = true AND current_state IN ('started', 'unstarted')
		ORDER BY updated_at DESC`,

		`CREATE OR REPLACE VIEW stories_mentioning_me AS
		SELECT s.id, s.title, s.current_state, s.owner_names, s.is_mine,
		       s.updated_at, COUNT(c.id) AS mention_count
		FROM stories s
		JOIN comments c ON c.story_id = s.id AND c.mentions_me = true
		GROUP BY s.id, s.title, s.current_state, s.owner_names, s.is_mine, s.updated_at
		ORDER BY s.updated_at DESC`,

		`CREATE OR REPLACE VIEW recent_comments AS
		SELECT c.id, c.story_id, s.title AS story_title, c.person_name,
		       c.text, c.mentions_me, c.created_at
		FROM comments c
		JOIN stories s ON s.id = c.story_id
		ORDER BY c.created_at DESC`,

		`CREATE OR REPLACE VIEW story_stats AS
		SELECT
		  COUNT(*) AS total_stories,
		  COUNT(*) FILTER (WHERE is_mine) AS my_stories,
		  COUNT(*) FILTER (WHERE mentions_me) AS stories_with_mentions,
		  COUNT(*) FILTER (WHERE current_state = 'started') AS started,
		  COUNT(*) FILTER (WHERE current_state = 'unstarted') AS unstarted,
		  COUNT(*) FILTER (WHERE current_state = 'delivered') AS delivered,
		  COUNT(*) FILTER (WHERE current_state = 'accepted') AS accepted,
		  COUNT(*) FILTER (WHERE current_state = 'rejected') AS rejected
		FROM stories`,
	}
	for _, s := range views {
		if _, err := conn.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

// ParseApiDate converts LiteTracker date format "11 Feb 2026, 04:30AM" to ISO string.
func ParseApiDate(dateStr string) *string {
	if dateStr == "" {
		return nil
	}
	months := map[string]string{
		"Jan": "01", "Feb": "02", "Mar": "03", "Apr": "04", "May": "05", "Jun": "06",
		"Jul": "07", "Aug": "08", "Sep": "09", "Oct": "10", "Nov": "11", "Dec": "12",
	}
	re := regexp.MustCompile(`^(\d{1,2})\s+(\w{3})\s+(\d{4}),\s+(\d{1,2}):(\d{2})(AM|PM)$`)
	m := re.FindStringSubmatch(dateStr)
	if m == nil {
		return nil
	}
	day := m[1]
	mon := m[2]
	year := m[3]
	hourStr := m[4]
	min := m[5]
	ampm := strings.ToUpper(m[6])

	monthNum, ok := months[mon]
	if !ok {
		return nil
	}
	hour, _ := strconv.Atoi(hourStr)
	if ampm == "PM" && hour != 12 {
		hour += 12
	}
	if ampm == "AM" && hour == 12 {
		hour = 0
	}

	result := fmt.Sprintf("%s-%s-%02s T%02d:%s:00.000Z", year, monthNum, day, hour, min)
	// Remove accidental space
	result = strings.ReplaceAll(result, " T", "T")
	return &result
}

type StoryRow struct {
	ID            int
	ProjectID     int
	Title         string
	Description   *string
	StoryType     *string
	CurrentState  *string
	Estimate      *int
	Priority      *string
	URL           *string
	RequestedByID *int
	OwnerNames    *string
	LabelNames    *string
	IsMine        bool
	MentionsMe    bool
	CreatedAt     string
	UpdatedAt     string
}

func UpsertStory(s StoryRow) error {
	now := time.Now().UTC().Format(time.RFC3339)
	createdAt := ParseApiDate(s.CreatedAt)
	updatedAt := ParseApiDate(s.UpdatedAt)

	_, err := conn.Exec(
		`INSERT INTO stories (id, project_id, title, description, story_type, current_state,
			estimate, priority, url, requested_by_id, owner_names, label_names,
			is_mine, mentions_me, created_at, updated_at, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRY_CAST(? AS TIMESTAMP), TRY_CAST(? AS TIMESTAMP), TRY_CAST(? AS TIMESTAMP))
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			story_type = excluded.story_type,
			current_state = excluded.current_state,
			estimate = excluded.estimate,
			priority = excluded.priority,
			url = excluded.url,
			requested_by_id = excluded.requested_by_id,
			owner_names = excluded.owner_names,
			label_names = excluded.label_names,
			is_mine = excluded.is_mine,
			mentions_me = CASE WHEN excluded.mentions_me THEN true ELSE stories.mentions_me END,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			synced_at = excluded.synced_at`,
		s.ID, s.ProjectID, s.Title, s.Description, s.StoryType, s.CurrentState,
		s.Estimate, s.Priority, s.URL, s.RequestedByID, s.OwnerNames, s.LabelNames,
		s.IsMine, s.MentionsMe, ptrOrNil(createdAt), ptrOrNil(updatedAt), now,
	)
	return err
}

type CommentRow struct {
	ID         int
	StoryID    int
	ProjectID  int
	Text       *string
	PersonID   *int
	PersonName *string
	MentionsMe bool
	CreatedAt  string
}

func UpsertComment(c CommentRow) error {
	now := time.Now().UTC().Format(time.RFC3339)
	createdAt := ParseApiDate(c.CreatedAt)

	_, err := conn.Exec(
		`INSERT INTO comments (id, story_id, project_id, text, person_id, person_name, mentions_me, created_at, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, TRY_CAST(? AS TIMESTAMP), TRY_CAST(? AS TIMESTAMP))
		ON CONFLICT(id) DO UPDATE SET
			text = excluded.text,
			person_id = excluded.person_id,
			person_name = excluded.person_name,
			mentions_me = CASE WHEN excluded.mentions_me THEN true ELSE comments.mentions_me END,
			synced_at = excluded.synced_at`,
		c.ID, c.StoryID, c.ProjectID, c.Text, c.PersonID, c.PersonName,
		c.MentionsMe, ptrOrNil(createdAt), now,
	)
	return err
}

func MarkStoryMentionsMe(storyID int) error {
	_, err := conn.Exec("UPDATE stories SET mentions_me = true WHERE id = ?", storyID)
	return err
}

func CreateSnapshot() error {
	tmpPath := snapPath() + ".tmp"

	// Remove stale tmp file
	os.Remove(tmpPath)

	if _, err := conn.Exec("CHECKPOINT"); err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}

	// Copy main DB to tmp, then atomic rename
	data, err := os.ReadFile(dbPath())
	if err != nil {
		return fmt.Errorf("read db: %w", err)
	}
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, snapPath()); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func ptrOrNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
