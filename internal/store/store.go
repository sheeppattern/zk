package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sheeppattern/nete/internal/model"
	_ "modernc.org/sqlite"
)

// Store manages SQLite-based persistence for memos, notes, links, and config.
type Store struct {
	db *sql.DB
}

// SearchOptions controls filtering and sorting for memo search.
type SearchOptions struct {
	NoteID        int64
	Layer         string
	Status        string
	Author        string
	Tags          []string
	CreatedAfter  time.Time
	CreatedBefore time.Time
	Sort          string // "relevance", "created", "updated"
	Limit         int
}

// LinkWithDepth represents a link discovered during BFS traversal with its depth.
type LinkWithDepth struct {
	model.Link
	Depth int `json:"depth"`
}

// NewStore opens or creates a SQLite database at the given path.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite requires single connection to ensure pragmas apply consistently.
	db.SetMaxOpenConns(1)
	// Verify connectivity.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Init creates all tables, indexes, triggers, and sets pragmas.
func (s *Store) Init() error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := s.db.Exec(p); err != nil {
			return fmt.Errorf("exec pragma %q: %w", p, err)
		}
	}

	schema := `
CREATE TABLE IF NOT EXISTS notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS memos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    tags TEXT NOT NULL DEFAULT '[]',
    layer TEXT NOT NULL DEFAULT 'concrete',
    note_id INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    author TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS memos_fts USING fts5(
    title, content, tags, summary,
    content='memos', content_rowid='id'
);

CREATE TRIGGER IF NOT EXISTS memos_ai AFTER INSERT ON memos BEGIN
    INSERT INTO memos_fts(rowid, title, content, tags, summary)
    VALUES (new.id, new.title, new.content, new.tags, new.summary);
END;
CREATE TRIGGER IF NOT EXISTS memos_ad AFTER DELETE ON memos BEGIN
    INSERT INTO memos_fts(memos_fts, rowid, title, content, tags, summary)
    VALUES ('delete', old.id, old.title, old.content, old.tags, old.summary);
END;
CREATE TRIGGER IF NOT EXISTS memos_au AFTER UPDATE ON memos BEGIN
    INSERT INTO memos_fts(memos_fts, rowid, title, content, tags, summary)
    VALUES ('delete', old.id, old.title, old.content, old.tags, old.summary);
    INSERT INTO memos_fts(rowid, title, content, tags, summary)
    VALUES (new.id, new.title, new.content, new.tags, new.summary);
END;

CREATE TABLE IF NOT EXISTS links (
    source_id INTEGER NOT NULL,
    target_id INTEGER NOT NULL,
    relation_type TEXT NOT NULL DEFAULT 'related',
    weight REAL NOT NULL DEFAULT 0.5,
    PRIMARY KEY (source_id, target_id, relation_type)
);

CREATE TABLE IF NOT EXISTS trash (
    id INTEGER PRIMARY KEY,
    original_data TEXT NOT NULL,
    deleted_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memos_note ON memos(note_id);
CREATE INDEX IF NOT EXISTS idx_memos_layer ON memos(layer);
CREATE INDEX IF NOT EXISTS idx_links_source ON links(source_id);
CREATE INDEX IF NOT EXISTS idx_links_target ON links(target_id);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Notes (formerly Projects)
// ---------------------------------------------------------------------------

// CreateNote inserts a new note and returns it with the assigned ID.
func (s *Store) CreateNote(name, description string) (*model.Note, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(
		"INSERT INTO notes (name, description, created_at, updated_at) VALUES (?, ?, ?, ?)",
		name, description, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert note: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get note id: %w", err)
	}
	t, _ := time.Parse(time.RFC3339, now)
	return &model.Note{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   t,
		UpdatedAt:   t,
	}, nil
}

// GetNote retrieves a note by ID.
func (s *Store) GetNote(id int64) (*model.Note, error) {
	var n model.Note
	var createdAt, updatedAt string
	err := s.db.QueryRow(
		"SELECT id, name, description, created_at, updated_at FROM notes WHERE id = ?", id,
	).Scan(&n.ID, &n.Name, &n.Description, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("note %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get note %d: %w", id, err)
	}
	n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &n, nil
}

// ListNotes returns all notes.
func (s *Store) ListNotes() ([]*model.Note, error) {
	rows, err := s.db.Query("SELECT id, name, description, created_at, updated_at FROM notes ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var createdAt, updatedAt string
		if err := rows.Scan(&n.ID, &n.Name, &n.Description, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		n.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		notes = append(notes, &n)
	}
	return notes, rows.Err()
}

// DeleteNote removes a note by ID. Returns an error if the note has memos.
func (s *Store) DeleteNote(id int64) error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM memos WHERE note_id = ?", id).Scan(&count); err != nil {
		return fmt.Errorf("check memos for note %d: %w", id, err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete note %d: has %d memos", id, count)
	}
	result, err := s.db.Exec("DELETE FROM notes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete note %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("note %d not found", id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Memos (formerly Notes)
// ---------------------------------------------------------------------------

// CreateMemo inserts a memo and sets memo.ID from the auto-generated row ID.
func (s *Store) CreateMemo(memo *model.Memo) error {
	now := time.Now().UTC()
	if memo.Metadata.CreatedAt.IsZero() {
		memo.Metadata.CreatedAt = now
	}
	if memo.Metadata.UpdatedAt.IsZero() {
		memo.Metadata.UpdatedAt = now
	}
	if memo.Tags == nil {
		memo.Tags = []string{}
	}
	if memo.Layer == "" {
		memo.Layer = model.LayerConcrete
	}
	if memo.Metadata.Status == "" {
		memo.Metadata.Status = model.StatusActive
	}

	tagsJSON, err := json.Marshal(memo.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	createdStr := memo.Metadata.CreatedAt.Format(time.RFC3339)
	updatedStr := memo.Metadata.UpdatedAt.Format(time.RFC3339)
	result, err := s.db.Exec(
		`INSERT INTO memos (title, content, tags, layer, note_id, status, author, source, summary, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		memo.Title, memo.Content, string(tagsJSON), memo.Layer, memo.NoteID,
		memo.Metadata.Status, memo.Metadata.Author, memo.Metadata.Source, memo.Metadata.Summary,
		createdStr, updatedStr,
	)
	if err != nil {
		return fmt.Errorf("insert memo: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get memo id: %w", err)
	}
	memo.ID = id
	return nil
}

// GetMemo retrieves a memo by ID.
func (s *Store) GetMemo(id int64) (*model.Memo, error) {
	var m model.Memo
	var tagsStr, createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, title, content, tags, layer, note_id, status, author, source, summary, created_at, updated_at
		 FROM memos WHERE id = ?`, id,
	).Scan(&m.ID, &m.Title, &m.Content, &tagsStr, &m.Layer, &m.NoteID,
		&m.Metadata.Status, &m.Metadata.Author, &m.Metadata.Source, &m.Metadata.Summary,
		&createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("memo %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get memo %d: %w", id, err)
	}
	if err := json.Unmarshal([]byte(tagsStr), &m.Tags); err != nil {
		return nil, fmt.Errorf("parse tags for memo %d: %w", id, err)
	}
	m.Metadata.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	m.Metadata.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &m, nil
}

// UpdateMemo updates an existing memo, setting updated_at to now.
func (s *Store) UpdateMemo(memo *model.Memo) error {
	now := time.Now().UTC()
	memo.Metadata.UpdatedAt = now
	if memo.Tags == nil {
		memo.Tags = []string{}
	}

	tagsJSON, err := json.Marshal(memo.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	result, err := s.db.Exec(
		`UPDATE memos SET title=?, content=?, tags=?, layer=?, note_id=?, status=?, author=?, source=?, summary=?, updated_at=?
		 WHERE id=?`,
		memo.Title, memo.Content, string(tagsJSON), memo.Layer, memo.NoteID,
		memo.Metadata.Status, memo.Metadata.Author, memo.Metadata.Source, memo.Metadata.Summary,
		now.Format(time.RFC3339), memo.ID,
	)
	if err != nil {
		return fmt.Errorf("update memo %d: %w", memo.ID, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("memo %d not found", memo.ID)
	}
	return nil
}

// DeleteMemo moves a memo to the trash table as JSON, then deletes it.
func (s *Store) DeleteMemo(id int64) error {
	memo, err := s.GetMemo(id)
	if err != nil {
		return err
	}

	data, err := json.Marshal(memo)
	if err != nil {
		return fmt.Errorf("marshal memo for trash: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("INSERT INTO trash (id, original_data, deleted_at) VALUES (?, ?, ?)", id, string(data), now); err != nil {
		return fmt.Errorf("insert trash: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM links WHERE source_id = ? OR target_id = ?", id, id); err != nil {
		return fmt.Errorf("delete links for memo: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM memos WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete memo: %w", err)
	}
	return tx.Commit()
}

// ListMemos returns memos for a given note ID. noteID=0 means global memos.
func (s *Store) ListMemos(noteID int64) ([]*model.Memo, error) {
	rows, err := s.db.Query(
		`SELECT id, title, content, tags, layer, note_id, status, author, source, summary, created_at, updated_at
		 FROM memos WHERE note_id = ? ORDER BY id`, noteID,
	)
	if err != nil {
		return nil, fmt.Errorf("list memos: %w", err)
	}
	defer rows.Close()
	return scanMemos(rows)
}

// ListAllMemos returns all memos across all notes.
func (s *Store) ListAllMemos() ([]*model.Memo, error) {
	rows, err := s.db.Query(
		`SELECT id, title, content, tags, layer, note_id, status, author, source, summary, created_at, updated_at
		 FROM memos ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all memos: %w", err)
	}
	defer rows.Close()
	return scanMemos(rows)
}

// MoveMemo changes the note_id for a memo.
func (s *Store) MoveMemo(memoID, targetNoteID int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec("UPDATE memos SET note_id = ?, updated_at = ? WHERE id = ?", targetNoteID, now, memoID)
	if err != nil {
		return fmt.Errorf("move memo %d: %w", memoID, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("memo %d not found", memoID)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

// SearchMemos performs FTS5 search with optional filtering.
// If query is a pure integer, it also matches memo ID directly.
func (s *Store) SearchMemos(query string, opts SearchOptions) ([]*model.Memo, error) {
	var args []interface{}
	var conditions []string

	// Check if query is a numeric ID.
	queryID, isNumericQuery := int64(0), false
	if query != "" {
		if id, err := strconv.ParseInt(strings.TrimSpace(query), 10, 64); err == nil && id > 0 {
			queryID = id
			isNumericQuery = true
		}
	}

	// Base: join memos with FTS results.
	baseQuery := `SELECT m.id, m.title, m.content, m.tags, m.layer, m.note_id,
		m.status, m.author, m.source, m.summary, m.created_at, m.updated_at
		FROM memos m`

	hasFTS := query != ""
	if hasFTS {
		baseQuery += ` JOIN memos_fts f ON m.id = f.rowid`
		conditions = append(conditions, "memos_fts MATCH ?")
		// Quote the query to prevent FTS5 syntax errors from special characters.
		quotedQuery := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
		args = append(args, quotedQuery)
	}

	if opts.NoteID != 0 {
		conditions = append(conditions, "m.note_id = ?")
		args = append(args, opts.NoteID)
	}
	if opts.Layer != "" {
		conditions = append(conditions, "m.layer = ?")
		args = append(args, opts.Layer)
	}
	if opts.Status != "" {
		conditions = append(conditions, "m.status = ?")
		args = append(args, opts.Status)
	}
	if opts.Author != "" {
		conditions = append(conditions, "m.author = ?")
		args = append(args, opts.Author)
	}
	if !opts.CreatedAfter.IsZero() {
		conditions = append(conditions, "m.created_at >= ?")
		args = append(args, opts.CreatedAfter.Format(time.RFC3339))
	}
	if !opts.CreatedBefore.IsZero() {
		conditions = append(conditions, "m.created_at <= ?")
		args = append(args, opts.CreatedBefore.Format(time.RFC3339))
	}
	// Tag filtering: exact match via json_each().
	for _, tag := range opts.Tags {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM json_each(m.tags) WHERE value = ?)")
		args = append(args, tag)
	}

	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Sort order.
	switch opts.Sort {
	case "created":
		baseQuery += " ORDER BY m.created_at DESC"
	case "updated":
		baseQuery += " ORDER BY m.updated_at DESC"
	default:
		if hasFTS {
			baseQuery += " ORDER BY bm25(memos_fts)"
		} else {
			baseQuery += " ORDER BY m.id DESC"
		}
	}

	if opts.Limit > 0 {
		baseQuery += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	rows, err := s.db.Query(baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search memos: %w", err)
	}
	defer rows.Close()
	results, err := scanMemos(rows)
	if err != nil {
		return nil, err
	}

	// If query is a numeric ID, prepend the matching memo if not already in results.
	if isNumericQuery {
		memo, mErr := s.GetMemo(queryID)
		if mErr == nil {
			found := false
			for _, r := range results {
				if r.ID == queryID {
					found = true
					break
				}
			}
			if !found {
				results = append([]*model.Memo{memo}, results...)
			}
		}
	}

	return results, nil
}

// ---------------------------------------------------------------------------
// Links
// ---------------------------------------------------------------------------

// AddLink inserts a single link (no bidirectional duplication).
func (s *Store) AddLink(sourceID, targetID int64, relationType string, weight float64) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO links (source_id, target_id, relation_type, weight) VALUES (?, ?, ?, ?)",
		sourceID, targetID, relationType, weight,
	)
	if err != nil {
		return fmt.Errorf("add link: %w", err)
	}
	return nil
}

// RemoveLink deletes a link.
func (s *Store) RemoveLink(sourceID, targetID int64, relationType string) error {
	result, err := s.db.Exec(
		"DELETE FROM links WHERE source_id = ? AND target_id = ? AND relation_type = ?",
		sourceID, targetID, relationType,
	)
	if err != nil {
		return fmt.Errorf("remove link: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("link not found")
	}
	return nil
}

// ListLinks returns outgoing and incoming links for a memo.
func (s *Store) ListLinks(memoID int64) (outgoing []model.Link, incoming []model.Link, err error) {
	// Outgoing links.
	rows, err := s.db.Query(
		"SELECT source_id, target_id, relation_type, weight FROM links WHERE source_id = ?", memoID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list outgoing links: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var l model.Link
		if err := rows.Scan(&l.SourceID, &l.TargetID, &l.RelationType, &l.Weight); err != nil {
			return nil, nil, fmt.Errorf("scan outgoing link: %w", err)
		}
		outgoing = append(outgoing, l)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Incoming links.
	rows2, err := s.db.Query(
		"SELECT source_id, target_id, relation_type, weight FROM links WHERE target_id = ?", memoID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list incoming links: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var l model.Link
		if err := rows2.Scan(&l.SourceID, &l.TargetID, &l.RelationType, &l.Weight); err != nil {
			return nil, nil, fmt.Errorf("scan incoming link: %w", err)
		}
		incoming = append(incoming, l)
	}
	return outgoing, incoming, rows2.Err()
}

// maxBFSDepth is the hard cap on BFS traversal depth.
const maxBFSDepth = 5

// maxBFSResults is the hard cap on BFS result count.
const maxBFSResults = 1000

// ListLinksBFS performs a breadth-first traversal from a memo up to the given depth.
func (s *Store) ListLinksBFS(memoID int64, depth int) ([]LinkWithDepth, error) {
	if depth <= 0 {
		return nil, nil
	}
	if depth > maxBFSDepth {
		depth = maxBFSDepth
	}

	visited := map[int64]bool{memoID: true}
	queue := []struct {
		id    int64
		depth int
	}{{memoID, 0}}

	var result []LinkWithDepth

	for len(queue) > 0 && len(result) < maxBFSResults {
		current := queue[0]
		queue = queue[1:]

		if current.depth >= depth {
			continue
		}

		rows, err := s.db.Query(
			"SELECT source_id, target_id, relation_type, weight FROM links WHERE source_id = ? OR target_id = ?",
			current.id, current.id,
		)
		if err != nil {
			return nil, fmt.Errorf("bfs query: %w", err)
		}

		var links []model.Link
		for rows.Next() {
			var l model.Link
			if err := rows.Scan(&l.SourceID, &l.TargetID, &l.RelationType, &l.Weight); err != nil {
				rows.Close()
				return nil, fmt.Errorf("bfs scan: %w", err)
			}
			links = append(links, l)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}

		for _, l := range links {
			neighbor := l.TargetID
			if neighbor == current.id {
				neighbor = l.SourceID
			}
			if !visited[neighbor] {
				result = append(result, LinkWithDepth{Link: l, Depth: current.depth + 1})
				visited[neighbor] = true
				queue = append(queue, struct {
					id    int64
					depth int
				}{neighbor, current.depth + 1})
			}
		}
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// LoadConfig reads configuration from the config table.
func (s *Store) LoadConfig() (*model.Config, error) {
	cfg := model.DefaultConfig()
	rows, err := s.db.Query("SELECT key, value FROM config")
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		switch key {
		case "store_path":
			cfg.StorePath = value
		case "default_note":
			cfg.DefaultNote = value
		case "default_format":
			cfg.DefaultFormat = value
		case "default_author":
			cfg.DefaultAuthor = value
		case "custom_relation_types":
			if value != "" {
				var types []string
				if err := json.Unmarshal([]byte(value), &types); err == nil {
					cfg.CustomRelationTypes = types
				}
			}
		}
	}
	return cfg, rows.Err()
}

// SaveConfig upserts configuration into the config table.
func (s *Store) SaveConfig(cfg *model.Config) error {
	pairs := map[string]string{
		"store_path":     cfg.StorePath,
		"default_note":   cfg.DefaultNote,
		"default_format": cfg.DefaultFormat,
		"default_author": cfg.DefaultAuthor,
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for k, v := range pairs {
		if _, err := tx.Exec(
			"INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
			k, v,
		); err != nil {
			return fmt.Errorf("save config %q: %w", k, err)
		}
	}

	if cfg.CustomRelationTypes != nil {
		typesJSON, err := json.Marshal(cfg.CustomRelationTypes)
		if err != nil {
			return fmt.Errorf("marshal custom relation types: %w", err)
		}
		if _, err := tx.Exec(
			"INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
			"custom_relation_types", string(typesJSON),
		); err != nil {
			return fmt.Errorf("save config custom_relation_types: %w", err)
		}
	}

	return tx.Commit()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// scanMemos scans rows into a slice of Memo pointers.
func scanMemos(rows *sql.Rows) ([]*model.Memo, error) {
	var memos []*model.Memo
	for rows.Next() {
		var m model.Memo
		var tagsStr, createdAt, updatedAt string
		if err := rows.Scan(
			&m.ID, &m.Title, &m.Content, &tagsStr, &m.Layer, &m.NoteID,
			&m.Metadata.Status, &m.Metadata.Author, &m.Metadata.Source, &m.Metadata.Summary,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan memo: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsStr), &m.Tags); err != nil {
			return nil, fmt.Errorf("parse tags: %w", err)
		}
		m.Metadata.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		m.Metadata.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		memos = append(memos, &m)
	}
	return memos, rows.Err()
}
