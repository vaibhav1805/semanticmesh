// Package knowledge — SQLite persistence layer for indexes and knowledge graphs.
//
// Schema overview:
//
//	documents     — one row per markdown file (path, hash, mtime)
//	index_entries — inverted index postings (term → doc with frequency)
//	bm25_stats    — corpus-level BM25 parameters (N, avgDocLen, termDocs JSON)
//	graph_nodes   — knowledge graph vertices (id, type, file, title, metadata)
//	graph_edges   — knowledge graph directed edges (source, target, type, confidence, evidence)
//	metadata      — key/value store used for schema versioning
//
// All multi-step writes are wrapped in transactions to guarantee atomicity.
// Foreign keys with ON DELETE CASCADE ensure referential integrity when
// documents or nodes are removed.
//
// Usage:
//
//	db, err := OpenDB("/path/to/bmd.db")
//	if err != nil { ... }
//	defer db.Close()
//
//	// Persist an index
//	if err := db.SaveIndex(idx); err != nil { ... }
//
//	// Reload it later
//	idx2 := NewIndex()
//	if err := db.LoadIndex(idx2); err != nil { ... }
package knowledge

import (
	"crypto/md5" //nolint:gosec // used for file change detection, not security
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/graphmd/graphmd/internal/code"
	_ "modernc.org/sqlite" // register "sqlite" driver
)

// stderrWriter is the destination for warning messages (e.g., dropped edges).
// Defaults to os.Stderr; tests override it with a bytes.Buffer to capture output.
var stderrWriter io.Writer = os.Stderr

// SchemaVersion is incremented each time the database schema changes.
// Migrations run automatically in Migrate() when an older database is opened.
const SchemaVersion = 6

// Database wraps an open SQLite connection and provides domain-level
// read/write operations for indexes and knowledge graphs.
//
// Zero value is NOT valid; always create via OpenDB or NewDatabase.
type Database struct {
	path string
	conn *sql.DB
}

// ─── construction ────────────────────────────────────────────────────────────

// OpenDB opens (or creates) the SQLite database at path, initialises the
// schema, and runs any outstanding migrations.
//
// Equivalent to NewDatabase(path) followed by Initialize() and Migrate().
func OpenDB(path string) (*Database, error) {
	db, err := NewDatabase(path)
	if err != nil {
		return nil, err
	}
	if err := db.Initialize(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("knowledge.OpenDB: initialize: %w", err)
	}
	if err := db.Migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("knowledge.OpenDB: migrate: %w", err)
	}
	return db, nil
}

// NewDatabase opens the SQLite database at path without schema initialisation.
// Call Initialize() and Migrate() before use, or use OpenDB for convenience.
func NewDatabase(path string) (*Database, error) {
	// Ensure the parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("knowledge.NewDatabase: mkdir: %w", err)
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("knowledge.NewDatabase: open %q: %w", path, err)
	}

	// SQLite supports only one writer at a time; WAL mode improves concurrency.
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("knowledge.NewDatabase: enable WAL: %w", err)
	}
	// Enforce foreign key constraints (disabled by default in SQLite).
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("knowledge.NewDatabase: enable foreign_keys: %w", err)
	}

	return &Database{path: path, conn: conn}, nil
}

// Close closes the underlying database connection.
func (db *Database) Close() error {
	return db.conn.Close()
}

// ─── schema initialisation ────────────────────────────────────────────────────

// schemaSQL is the complete DDL for SchemaVersion 1.
// Each statement is idempotent (CREATE TABLE IF NOT EXISTS / CREATE INDEX IF
// NOT EXISTS) so Initialize may be called on an already-initialised database.
const schemaSQL = `
-- documents: one row per markdown file in the indexed corpus.
CREATE TABLE IF NOT EXISTS documents (
  id            TEXT    PRIMARY KEY,
  path          TEXT    NOT NULL UNIQUE,
  title         TEXT,
  content_hash  TEXT    NOT NULL,
  last_modified INTEGER NOT NULL,
  indexed_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_documents_path ON documents(path);

-- index_entries: inverted index — one row per (term, document/chunk) pair.
-- positions is a JSON array: [line, offset, ...] (currently unused but
-- reserved for future phrase-search support).
-- chunk_id is the full chunk DocID ("relPath#HeadingPath:L{startLine}").
-- heading_path, start_line, end_line carry chunk-level location metadata.
CREATE TABLE IF NOT EXISTS index_entries (
  id           INTEGER PRIMARY KEY,
  term         TEXT    NOT NULL,
  doc_id       TEXT    NOT NULL,
  positions    TEXT,
  frequency    INTEGER NOT NULL,
  chunk_id     TEXT,
  heading_path TEXT,
  start_line   INTEGER,
  end_line     INTEGER,
  FOREIGN KEY(doc_id) REFERENCES documents(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_index_terms ON index_entries(term);
CREATE INDEX IF NOT EXISTS idx_index_docs  ON index_entries(doc_id);

-- bm25_stats: corpus-level statistics required for BM25 scoring.
-- param is one of: "N", "avg_doc_len", "term_docs" (JSON object), "k1", "b".
CREATE TABLE IF NOT EXISTS bm25_stats (
  param TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- graph_nodes: vertices in the knowledge graph.
-- metadata is a JSON object (e.g. {"heading_level": 1, "line_range": [1,40]}).
-- component_type classifies the node using the 12-type taxonomy (default: unknown).
CREATE TABLE IF NOT EXISTS graph_nodes (
  id             TEXT PRIMARY KEY,
  type           TEXT NOT NULL,
  file           TEXT NOT NULL,
  title          TEXT,
  content        TEXT,
  metadata       TEXT,
  component_type TEXT NOT NULL DEFAULT 'unknown'
);
CREATE INDEX IF NOT EXISTS idx_nodes_type ON graph_nodes(type);
CREATE INDEX IF NOT EXISTS idx_nodes_component_type ON graph_nodes(component_type);
CREATE INDEX IF NOT EXISTS idx_nodes_title ON graph_nodes(title);

-- component_mentions: tracks where each component was detected, providing
-- provenance for "where was this found?" queries.
CREATE TABLE IF NOT EXISTS component_mentions (
  id                INTEGER PRIMARY KEY,
  component_id      TEXT    NOT NULL,
  file_path         TEXT    NOT NULL,
  heading_hierarchy TEXT,
  detected_by       TEXT    NOT NULL,
  confidence        REAL    NOT NULL CHECK (confidence >= 0.0 AND confidence <= 1.0),
  FOREIGN KEY(component_id) REFERENCES graph_nodes(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_mentions_component ON component_mentions(component_id);
CREATE INDEX IF NOT EXISTS idx_mentions_file ON component_mentions(file_path);

-- graph_edges: directed edges in the knowledge graph.
-- confidence must be in [0.0, 1.0] (enforced by CHECK constraint).
-- Provenance columns track where, how, and when each relationship was detected.
CREATE TABLE IF NOT EXISTS graph_edges (
  id                  TEXT    PRIMARY KEY,
  source_id           TEXT    NOT NULL,
  target_id           TEXT    NOT NULL,
  type                TEXT    NOT NULL,
  confidence          REAL    NOT NULL CHECK (confidence >= 0.0 AND confidence <= 1.0),
  evidence            TEXT,
  source_file         TEXT,
  extraction_method   TEXT,
  detection_evidence  TEXT,
  evidence_pointer    TEXT,
  last_modified       INTEGER,
  source_type         TEXT    NOT NULL DEFAULT 'markdown',
  FOREIGN KEY(source_id) REFERENCES graph_nodes(id) ON DELETE CASCADE,
  FOREIGN KEY(target_id) REFERENCES graph_nodes(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_edges_source ON graph_edges(source_id);
CREATE INDEX IF NOT EXISTS idx_edges_target ON graph_edges(target_id);
CREATE INDEX IF NOT EXISTS idx_edges_confidence ON graph_edges(confidence);

-- code_signals: raw provenance from code analysis (import/call detection).
CREATE TABLE IF NOT EXISTS code_signals (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  source_component  TEXT NOT NULL,
  target_component  TEXT NOT NULL,
  signal_type       TEXT NOT NULL,
  confidence        REAL NOT NULL,
  evidence          TEXT NOT NULL,
  file_path         TEXT NOT NULL,
  line_number       INTEGER NOT NULL,
  language          TEXT NOT NULL,
  created_at        TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_code_signals_source ON code_signals(source_component);
CREATE INDEX IF NOT EXISTS idx_code_signals_target ON code_signals(target_component);

-- metadata: arbitrary key/value pairs (schema version, timestamps, etc.).
CREATE TABLE IF NOT EXISTS metadata (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`

// Initialize creates all tables and indexes if they do not yet exist.
// This method is idempotent — calling it on an existing schema is safe.
func (db *Database) Initialize() error {
	if err := db.execMulti(schemaSQL); err != nil {
		return fmt.Errorf("knowledge.Database.Initialize: %w", err)
	}

	// Write the schema version if it is not already present.
	_, err := db.conn.Exec(
		`INSERT OR IGNORE INTO metadata (key, value) VALUES ('schema_version', ?)`,
		fmt.Sprintf("%d", SchemaVersion),
	)
	if err != nil {
		return fmt.Errorf("knowledge.Database.Initialize: write schema_version: %w", err)
	}
	return nil
}

// execMulti splits sql on ";" and executes each non-empty statement.
// This is required because database/sql does not support multi-statement Exec.
// Index creation errors on columns that don't yet exist (pre-migration) are
// tolerated — the migration will add the column and index.
func (db *Database) execMulti(sql string) error {
	for _, stmt := range strings.Split(sql, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.conn.Exec(stmt); err != nil {
			// Tolerate index/table creation errors referencing columns or tables
			// that will be added by a migration (e.g. component_type on an older DB).
			errStr := err.Error()
			isIndexOrCreate := strings.Contains(strings.ToUpper(stmt), "CREATE INDEX") ||
				strings.Contains(strings.ToUpper(stmt), "CREATE TABLE")
			if isIndexOrCreate && (strings.Contains(errStr, "no such column") ||
				strings.Contains(errStr, "no such table")) {
				continue
			}
			return fmt.Errorf("exec %q: %w", stmt[:min(len(stmt), 60)], err)
		}
	}
	return nil
}

// min returns the smaller of a and b (int).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── schema version & migrations ─────────────────────────────────────────────

// GetVersion returns the schema version stored in the metadata table.
// Returns 0 if the version has not been recorded yet.
func (db *Database) GetVersion() int {
	var v string
	err := db.conn.QueryRow(
		`SELECT value FROM metadata WHERE key='schema_version'`,
	).Scan(&v)
	if err != nil {
		return 0
	}
	var n int
	fmt.Sscanf(v, "%d", &n)
	return n
}

// Migrate inspects the stored schema version and runs any applicable
// migration functions in order.  Migrations are idempotent by design.
func (db *Database) Migrate() error {
	current := db.GetVersion()

	if current < 2 {
		if err := db.migrateV1ToV2(); err != nil {
			return fmt.Errorf("knowledge.Database.Migrate: v1→v2: %w", err)
		}
	}
	if current < 3 {
		if err := db.migrateV2ToV3(); err != nil {
			return fmt.Errorf("knowledge.Database.Migrate: v2→v3: %w", err)
		}
	}
	if current < 4 {
		if err := db.migrateV3ToV4(); err != nil {
			return fmt.Errorf("knowledge.Database.Migrate: v3→v4: %w", err)
		}
	}
	if current < 5 {
		if err := db.migrateV4ToV5(); err != nil {
			return fmt.Errorf("knowledge.Database.Migrate: v4→v5: %w", err)
		}
	}
	if current < 6 {
		if err := db.migrateV5ToV6(); err != nil {
			return fmt.Errorf("knowledge.Database.Migrate: v5→v6: %w", err)
		}
	}

	// Ensure the stored version reflects the latest schema.
	if current < SchemaVersion {
		if _, err := db.conn.Exec(
			`INSERT OR REPLACE INTO metadata (key, value) VALUES ('schema_version', ?)`,
			fmt.Sprintf("%d", SchemaVersion),
		); err != nil {
			return fmt.Errorf("knowledge.Database.Migrate: update version: %w", err)
		}
	}
	return nil
}

// migrateV1ToV2 adds chunk metadata columns to index_entries.
// These columns are nullable and default to NULL for pre-existing rows.
func (db *Database) migrateV1ToV2() error {
	alterStatements := []string{
		`ALTER TABLE index_entries ADD COLUMN chunk_id     TEXT`,
		`ALTER TABLE index_entries ADD COLUMN heading_path TEXT`,
		`ALTER TABLE index_entries ADD COLUMN start_line   INTEGER`,
		`ALTER TABLE index_entries ADD COLUMN end_line     INTEGER`,
	}
	for _, stmt := range alterStatements {
		if _, err := db.conn.Exec(stmt); err != nil {
			// SQLite returns an error if the column already exists (from a CREATE TABLE
			// that included the column on a fresh database).  Ignore "duplicate column"
			// errors so Initialize() + Migrate() is idempotent.
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("exec %q: %w", stmt, err)
			}
		}
	}
	return nil
}

// migrateV2ToV3 adds component_type column to graph_nodes and creates the
// component_mentions table for provenance tracking.
func (db *Database) migrateV2ToV3() error {
	// Add component_type column (nullable ALTER, then default via trigger).
	alterStmt := `ALTER TABLE graph_nodes ADD COLUMN component_type TEXT NOT NULL DEFAULT 'unknown'`
	if _, err := db.conn.Exec(alterStmt); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("exec %q: %w", alterStmt, err)
		}
	}

	// Create component_mentions table.
	mentionsSQL := `
CREATE TABLE IF NOT EXISTS component_mentions (
  id                INTEGER PRIMARY KEY,
  component_id      TEXT    NOT NULL,
  file_path         TEXT    NOT NULL,
  heading_hierarchy TEXT,
  detected_by       TEXT    NOT NULL,
  confidence        REAL    NOT NULL CHECK (confidence >= 0.0 AND confidence <= 1.0),
  FOREIGN KEY(component_id) REFERENCES graph_nodes(id) ON DELETE CASCADE
)`
	if _, err := db.conn.Exec(mentionsSQL); err != nil {
		return fmt.Errorf("create component_mentions: %w", err)
	}

	// Create indexes.
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_nodes_component_type ON graph_nodes(component_type)`,
		`CREATE INDEX IF NOT EXISTS idx_mentions_component ON component_mentions(component_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mentions_file ON component_mentions(file_path)`,
	}
	for _, idx := range indexes {
		if _, err := db.conn.Exec(idx); err != nil {
			return fmt.Errorf("exec %q: %w", idx, err)
		}
	}

	return nil
}

// migrateV3ToV4 adds provenance columns to graph_edges for relationship source tracking.
// v3→v4: Add source_file, extraction_method, detection_evidence, evidence_pointer, last_modified.
func (db *Database) migrateV3ToV4() error {
	alterStatements := []string{
		`ALTER TABLE graph_edges ADD COLUMN source_file TEXT`,
		`ALTER TABLE graph_edges ADD COLUMN extraction_method TEXT`,
		`ALTER TABLE graph_edges ADD COLUMN detection_evidence TEXT`,
		`ALTER TABLE graph_edges ADD COLUMN evidence_pointer TEXT`,
		`ALTER TABLE graph_edges ADD COLUMN last_modified INTEGER`,
	}
	for _, stmt := range alterStatements {
		if _, err := db.conn.Exec(stmt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("exec %q: %w", stmt, err)
			}
		}
	}
	return nil
}

// migrateV4ToV5 adds indexes on graph_nodes.title and graph_edges.confidence
// for fast component name lookups and confidence filtering by agent queries.
func (db *Database) migrateV4ToV5() error {
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_nodes_title ON graph_nodes(title)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_confidence ON graph_edges(confidence)`,
	}
	for _, stmt := range indexes {
		if _, err := db.conn.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	return nil
}

// migrateV5ToV6 adds source_type column to graph_edges and creates the
// code_signals table for raw code analysis provenance.
func (db *Database) migrateV5ToV6() error {
	// Add source_type column to graph_edges.
	alterStmt := `ALTER TABLE graph_edges ADD COLUMN source_type TEXT NOT NULL DEFAULT 'markdown'`
	if _, err := db.conn.Exec(alterStmt); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("exec %q: %w", alterStmt, err)
		}
	}

	// Create code_signals table for raw code analysis provenance.
	codeSignalsSQL := `
CREATE TABLE IF NOT EXISTS code_signals (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  source_component  TEXT NOT NULL,
  target_component  TEXT NOT NULL,
  signal_type       TEXT NOT NULL,
  confidence        REAL NOT NULL,
  evidence          TEXT NOT NULL,
  file_path         TEXT NOT NULL,
  line_number       INTEGER NOT NULL,
  language          TEXT NOT NULL,
  created_at        TEXT NOT NULL DEFAULT (datetime('now'))
)`
	if _, err := db.conn.Exec(codeSignalsSQL); err != nil {
		return fmt.Errorf("create code_signals: %w", err)
	}

	// Create indexes for efficient lookups.
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_code_signals_source ON code_signals(source_component)`,
		`CREATE INDEX IF NOT EXISTS idx_code_signals_target ON code_signals(target_component)`,
	}
	for _, idx := range indexes {
		if _, err := db.conn.Exec(idx); err != nil {
			return fmt.Errorf("exec %q: %w", idx, err)
		}
	}

	return nil
}

// GetSchemaVersion is an alias for GetVersion provided for the plan's API.
func (db *Database) GetSchemaVersion() int { return db.GetVersion() }

// ─── transaction helper ───────────────────────────────────────────────────────

// transaction executes fn inside a database transaction.  If fn returns an
// error the transaction is rolled back; otherwise it is committed.
func transaction(dbConn *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := dbConn.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ─── SQL helper functions ─────────────────────────────────────────────────────

// nullIfEmpty returns nil (SQL NULL) when s is empty, otherwise returns s.
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// nullIfZero returns nil (SQL NULL) when v is 0, otherwise returns v.
func nullIfZero(v int64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}

// ─── index persistence ────────────────────────────────────────────────────────

const batchSize = 1000

// SaveIndex serialises idx to the database, replacing any previously stored
// index data.  All changes are wrapped in a single transaction.
func (db *Database) SaveIndex(idx *Index) error {
	return transaction(db.conn, func(tx *sql.Tx) error {
		// Clear old data.
		if _, err := tx.Exec(`DELETE FROM index_entries`); err != nil {
			return fmt.Errorf("clear index_entries: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM documents`); err != nil {
			return fmt.Errorf("clear documents: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM bm25_stats`); err != nil {
			return fmt.Errorf("clear bm25_stats: %w", err)
		}

		now := time.Now().UnixNano()

		// Insert documents in batches.
		// After chunk-level indexing, multiple indexedDoc entries share the same
		// relPath.  We deduplicate: the documents table stores one row per FILE
		// (keyed by relPath), not one row per chunk.
		docs := idx.bm25.docs

		// Build a deduplicated file-level view for the documents table.
		type fileDoc struct {
			relPath string
			path    string
			title   string
		}
		seenFiles := make(map[string]bool, len(docs))
		var fileDocs []fileDoc
		for _, d := range docs {
			if seenFiles[d.relPath] {
				continue
			}
			seenFiles[d.relPath] = true
			fileDocs = append(fileDocs, fileDoc{relPath: d.relPath, path: d.path, title: d.title})
		}

		for start := 0; start < len(fileDocs); start += batchSize {
			end := start + batchSize
			if end > len(fileDocs) {
				end = len(fileDocs)
			}
			for _, fd := range fileDocs[start:end] {
				// docMeta is keyed by the document ID (= RelPath for file-level docs).
				// After chunk indexing, the relPath is still the file-relative path.
				meta, hasMeta := idx.docMeta[fd.relPath]
				hash := meta.Hash
				lastMod := meta.LastModified
				if !hasMeta {
					hash = ""
					lastMod = now
				}
				_, err := tx.Exec(
					`INSERT OR REPLACE INTO documents
					 (id, path, title, content_hash, last_modified, indexed_at)
					 VALUES (?, ?, ?, ?, ?, ?)`,
					fd.relPath, fd.path, fd.title, hash, lastMod, now,
				)
				if err != nil {
					return fmt.Errorf("insert document %q: %w", fd.relPath, err)
				}
			}
		}

		// Insert index entries in batches.
		// doc_id references the file-level documents.id (= relPath).
		// chunk_id stores the full chunk DocID for reconstruction on load.
		type entry struct {
			term        string
			docID       string // file-level relPath (FK → documents.id)
			chunkID     string // full chunk DocID
			freq        int
			headingPath string
			startLine   int
			endLine     int
		}
		// Build a flat list of entries.
		var entries []entry
		for term, postings := range idx.bm25.postings {
			for _, pe := range postings {
				if pe.DocIndex < len(docs) {
					d := docs[pe.DocIndex]
					entries = append(entries, entry{
						term:        term,
						docID:       d.relPath,  // file-level FK
						chunkID:     d.id,       // chunk-level unique ID
						freq:        pe.TF,
						headingPath: d.headingPath,
						startLine:   d.startLine,
						endLine:     d.endLine,
					})
				}
			}
		}
		for start := 0; start < len(entries); start += batchSize {
			end := start + batchSize
			if end > len(entries) {
				end = len(entries)
			}
			for _, e := range entries[start:end] {
				_, err := tx.Exec(
					`INSERT INTO index_entries
					 (term, doc_id, positions, frequency, chunk_id, heading_path, start_line, end_line)
					 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					e.term, e.docID, nil, e.freq, e.chunkID, e.headingPath, e.startLine, e.endLine,
				)
				if err != nil {
					return fmt.Errorf("insert index_entry (%q, %q): %w", e.term, e.docID, err)
				}
			}
		}

		// Persist BM25 stats.
		stats := idx.bm25.stats
		termDocsJSON, err := json.Marshal(stats.TermDocs)
		if err != nil {
			return fmt.Errorf("marshal term_docs: %w", err)
		}
		bm25Rows := [][2]string{
			{"N", fmt.Sprintf("%d", stats.N)},
			{"avg_doc_len", fmt.Sprintf("%g", stats.AvgDocLen)},
			{"term_docs", string(termDocsJSON)},
			{"k1", fmt.Sprintf("%g", idx.params.K1)},
			{"b", fmt.Sprintf("%g", idx.params.B)},
		}
		for _, row := range bm25Rows {
			if _, err := tx.Exec(
				`INSERT OR REPLACE INTO bm25_stats (param, value) VALUES (?, ?)`,
				row[0], row[1],
			); err != nil {
				return fmt.Errorf("insert bm25_stat %q: %w", row[0], err)
			}
		}

		// Record the build timestamp in metadata for staleness detection.
		_, err = tx.Exec(
			`INSERT OR REPLACE INTO metadata (key, value) VALUES ('built_at', ?)`,
			fmt.Sprintf("%d", now),
		)
		if err != nil {
			return fmt.Errorf("write built_at: %w", err)
		}

		return nil
	})
}

// GetIndexBuiltAt returns the Unix nanosecond timestamp when the index was last
// built, or zero if no timestamp is recorded (backwards-compatible with old
// databases that don't have the built_at metadata key).
func (db *Database) GetIndexBuiltAt() time.Time {
	var val string
	err := db.conn.QueryRow(
		`SELECT value FROM metadata WHERE key='built_at'`,
	).Scan(&val)
	if err != nil {
		return time.Time{} // zero time — treat as stale
	}
	var ns int64
	fmt.Sscanf(val, "%d", &ns)
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

// IsIndexStale returns true if any markdown file under root has been modified
// since the index was last built, or if files have been added or removed.
// It compares file modification times on disk against the stored built_at
// timestamp and the document list in the database.
//
// Returns true (stale) when:
//   - Any .md file has an mtime newer than the index build time
//   - A .md file exists on disk but is not in the database (new file)
//   - A document in the database no longer exists on disk (deleted file)
//   - The built_at timestamp is missing (old index, backwards-compatible)
func (db *Database) IsIndexStale(root string) (bool, error) {
	builtAt := db.GetIndexBuiltAt()
	if builtAt.IsZero() {
		return true, nil // no timestamp — consider stale
	}

	// Collect document IDs from the database.
	rows, err := db.conn.Query(`SELECT id FROM documents`)
	if err != nil {
		return false, fmt.Errorf("knowledge.Database.IsIndexStale: query documents: %w", err)
	}
	defer rows.Close()

	dbDocs := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return false, fmt.Errorf("knowledge.Database.IsIndexStale: scan: %w", err)
		}
		dbDocs[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("knowledge.Database.IsIndexStale: iterate: %w", err)
	}

	// Walk disk and compare.
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false, fmt.Errorf("knowledge.Database.IsIndexStale: abs: %w", err)
	}

	diskDocs := make(map[string]struct{})
	stale := false

	walkErr := filepath.Walk(absRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") && path != absRoot {
				return filepath.SkipDir
			}
			if _, skip := map[string]struct{}{
				"node_modules": {}, ".git": {}, ".svn": {},
			}[name]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}
		rel, relErr := filepath.Rel(absRoot, path)
		if relErr != nil {
			return nil
		}
		id := filepath.ToSlash(rel)
		diskDocs[id] = struct{}{}

		// New file?
		if _, inDB := dbDocs[id]; !inDB {
			stale = true
		}
		// Modified after index build?
		if info.ModTime().After(builtAt) {
			stale = true
		}
		return nil
	})
	if walkErr != nil {
		return false, fmt.Errorf("knowledge.Database.IsIndexStale: walk: %w", walkErr)
	}

	// Deleted files?
	if !stale {
		for id := range dbDocs {
			if _, onDisk := diskDocs[id]; !onDisk {
				stale = true
				break
			}
		}
	}

	return stale, nil
}

// LoadIndex reconstructs idx from the database, replacing its current state.
// The Index must have been created via NewIndex(); it is safe to call on an
// empty or partially populated Index.
func (db *Database) LoadIndex(idx *Index) error {
	// Load file-level documents for docMeta and path information.
	rows, err := db.conn.Query(
		`SELECT id, path, title, content_hash, last_modified FROM documents`,
	)
	if err != nil {
		return fmt.Errorf("knowledge.Database.LoadIndex: query documents: %w", err)
	}
	defer rows.Close()

	type fileDocRow struct {
		id           string // file-level relPath
		path         string
		title        string
		contentHash  string
		lastModified int64
	}
	var fileDocRows []fileDocRow
	for rows.Next() {
		var r fileDocRow
		if err := rows.Scan(&r.id, &r.path, &r.title, &r.contentHash, &r.lastModified); err != nil {
			return fmt.Errorf("knowledge.Database.LoadIndex: scan document: %w", err)
		}
		fileDocRows = append(fileDocRows, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("knowledge.Database.LoadIndex: iterate documents: %w", err)
	}

	// Reset the index.
	idx.bm25 = NewBM25Index(idx.params, idx.tokenizer)
	idx.docMeta = make(map[string]docMeta, len(fileDocRows))

	// Build a map from file-level ID → file metadata for chunk reconstruction.
	type fileMeta struct {
		path  string
		title string
	}
	fileMetaMap := make(map[string]fileMeta, len(fileDocRows))
	for _, dr := range fileDocRows {
		fileMetaMap[dr.id] = fileMeta{path: dr.path, title: dr.title}
		idx.docMeta[dr.id] = docMeta{
			Hash:         dr.contentHash,
			LastModified: dr.lastModified,
		}
	}

	// Load index entries (one row per (term, chunk)).
	// We reconstruct chunk-level indexedDoc entries from the stored chunk metadata.
	eRows, err := db.conn.Query(
		`SELECT term, doc_id, frequency, chunk_id, heading_path, start_line, end_line
		 FROM index_entries`,
	)
	if err != nil {
		return fmt.Errorf("knowledge.Database.LoadIndex: query index_entries: %w", err)
	}
	defer eRows.Close()

	// chunkIndex maps chunk_id → position in idx.bm25.docs.
	chunkIndexMap := make(map[string]int)
	postings := make(map[string][]PostingEntry)
	termDocs := make(map[string]int)

	for eRows.Next() {
		var term, docID string
		var freq int
		var chunkID, headingPath sql.NullString
		var startLine, endLine sql.NullInt64
		if err := eRows.Scan(&term, &docID, &freq, &chunkID, &headingPath, &startLine, &endLine); err != nil {
			return fmt.Errorf("knowledge.Database.LoadIndex: scan index_entry: %w", err)
		}

		// Determine the effective chunk ID.
		// Legacy rows (before v2) have NULL chunk_id; fall back to doc_id.
		effectiveChunkID := docID
		if chunkID.Valid && chunkID.String != "" {
			effectiveChunkID = chunkID.String
		}

		// Get or create the indexedDoc for this chunk.
		di, exists := chunkIndexMap[effectiveChunkID]
		if !exists {
			fm := fileMetaMap[docID]
			relPath := filepath.FromSlash(docID)
			di = len(idx.bm25.docs)
			idx.bm25.docs = append(idx.bm25.docs, indexedDoc{
				id:          effectiveChunkID,
				path:        fm.path,
				relPath:     relPath,
				title:       fm.title,
				content:     "", // plain text not stored; snippets from doc on disk
				headingPath: headingPath.String,
				startLine:   int(startLine.Int64),
				endLine:     int(endLine.Int64),
			})
			chunkIndexMap[effectiveChunkID] = di
		}

		postings[term] = append(postings[term], PostingEntry{DocIndex: di, TF: freq})
		termDocs[term]++
	}
	if err := eRows.Err(); err != nil {
		return fmt.Errorf("knowledge.Database.LoadIndex: iterate index_entries: %w", err)
	}

	idx.bm25.postings = postings
	idx.bm25.stats.TermDocs = termDocs

	// Load BM25 stats.
	sRows, err := db.conn.Query(`SELECT param, value FROM bm25_stats`)
	if err != nil {
		return fmt.Errorf("knowledge.Database.LoadIndex: query bm25_stats: %w", err)
	}
	defer sRows.Close()

	for sRows.Next() {
		var param, value string
		if err := sRows.Scan(&param, &value); err != nil {
			return fmt.Errorf("knowledge.Database.LoadIndex: scan bm25_stat: %w", err)
		}
		switch param {
		case "N":
			fmt.Sscanf(value, "%d", &idx.bm25.stats.N)
		case "avg_doc_len":
			fmt.Sscanf(value, "%g", &idx.bm25.stats.AvgDocLen)
		case "k1":
			fmt.Sscanf(value, "%g", &idx.params.K1)
			idx.bm25.params.K1 = idx.params.K1
		case "b":
			fmt.Sscanf(value, "%g", &idx.params.B)
			idx.bm25.params.B = idx.params.B
		case "term_docs":
			var td map[string]int
			if json.Unmarshal([]byte(value), &td) == nil {
				idx.bm25.stats.TermDocs = td
			}
		}
	}
	if err := sRows.Err(); err != nil {
		return fmt.Errorf("knowledge.Database.LoadIndex: iterate bm25_stats: %w", err)
	}

	return nil
}

// ─── graph persistence ────────────────────────────────────────────────────────

// SaveGraph serialises graph to the database, replacing any previously stored
// graph data.  All changes are wrapped in a single transaction.
func (db *Database) SaveGraph(graph *Graph) error {
	// Track edges dropped due to missing endpoint nodes.
	var droppedPairs []string

	err := transaction(db.conn, func(tx *sql.Tx) error {
		// Clear old data (cascade deletes edges automatically).
		if _, err := tx.Exec(`DELETE FROM graph_edges`); err != nil {
			return fmt.Errorf("clear graph_edges: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM graph_nodes`); err != nil {
			return fmt.Errorf("clear graph_nodes: %w", err)
		}

		// Insert nodes.
		nodes := make([]*Node, 0, len(graph.Nodes))
		for _, n := range graph.Nodes {
			nodes = append(nodes, n)
		}
		for start := 0; start < len(nodes); start += batchSize {
			end := start + batchSize
			if end > len(nodes) {
				end = len(nodes)
			}
			for _, n := range nodes[start:end] {
				compType := n.ComponentType
				if compType == "" {
					compType = ComponentTypeUnknown
				}
				_, err := tx.Exec(
					`INSERT OR REPLACE INTO graph_nodes (id, type, file, title, content, metadata, component_type)
					 VALUES (?, ?, ?, ?, ?, ?, ?)`,
					n.ID, n.Type, n.ID /* file == ID */, n.Title, nil, nil, string(compType),
				)
				if err != nil {
					return fmt.Errorf("insert graph_node %q: %w", n.ID, err)
				}
			}
		}

		// Insert edges.
		edges := make([]*Edge, 0, len(graph.Edges))
		for _, e := range graph.Edges {
			edges = append(edges, e)
		}
		for start := 0; start < len(edges); start += batchSize {
			end := start + batchSize
			if end > len(edges) {
				end = len(edges)
			}
			for _, e := range edges[start:end] {
				// Skip edges that reference non-existent nodes (dangling references
				// from the extractor or discovery algorithms).
				if _, ok := graph.Nodes[e.Source]; !ok {
					droppedPairs = append(droppedPairs,
						fmt.Sprintf("  %s -> %s (missing source %q)", e.Source, e.Target, e.Source))
					continue
				}
				if _, ok := graph.Nodes[e.Target]; !ok {
					droppedPairs = append(droppedPairs,
						fmt.Sprintf("  %s -> %s (missing target %q)", e.Source, e.Target, e.Target))
					continue
				}
				sourceType := e.SourceType
				if sourceType == "" {
					sourceType = "markdown"
				}
				_, err := tx.Exec(
					`INSERT OR REPLACE INTO graph_edges
					 (id, source_id, target_id, type, confidence, evidence,
					  source_file, extraction_method, detection_evidence, evidence_pointer, last_modified, source_type)
					 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					e.ID, e.Source, e.Target, string(e.Type), e.Confidence, e.Evidence,
					nullIfEmpty(e.SourceFile), nullIfEmpty(e.ExtractionMethod),
					nullIfEmpty(e.DetectionEvidence), nullIfEmpty(e.EvidencePointer),
					nullIfZero(e.LastModified), sourceType,
				)
				if err != nil {
					return fmt.Errorf("insert graph_edge %q: %w", e.ID, err)
				}
			}
		}

		return nil
	})

	// Print dropped-edge warnings to stderr after successful transaction.
	if err == nil && len(droppedPairs) > 0 {
		w := stderrWriter
		fmt.Fprintf(w, "  Warning: %d edge(s) dropped (missing endpoint nodes)\n", len(droppedPairs))
		for _, pair := range droppedPairs {
			fmt.Fprintf(w, "%s\n", pair)
		}
	}

	return err
}

// LoadGraph reconstructs graph from the database.
// graph is reset before loading; any existing data is discarded.
func (db *Database) LoadGraph(graph *Graph) error {
	// Reset the graph.
	*graph = *NewGraph()

	// Load nodes in deterministic order (sorted by ID).
	nRows, err := db.conn.Query(
		`SELECT id, type, file, title, component_type FROM graph_nodes ORDER BY id ASC`,
	)
	if err != nil {
		return fmt.Errorf("knowledge.Database.LoadGraph: query graph_nodes: %w", err)
	}
	defer nRows.Close()

	for nRows.Next() {
		var id, nodeType, file string
		var title, compType sql.NullString
		if err := nRows.Scan(&id, &nodeType, &file, &title, &compType); err != nil {
			return fmt.Errorf("knowledge.Database.LoadGraph: scan node: %w", err)
		}
		ct := ComponentType(compType.String)
		if ct == "" {
			ct = ComponentTypeUnknown
		}
		n := &Node{ID: id, Type: nodeType, Title: title.String, ComponentType: ct}
		graph.Nodes[id] = n
	}
	if err := nRows.Err(); err != nil {
		return fmt.Errorf("knowledge.Database.LoadGraph: iterate nodes: %w", err)
	}

	// Load edges in deterministic order (sorted by source, target, then ID).
	eRows, err := db.conn.Query(
		`SELECT id, source_id, target_id, type, confidence, evidence,
		        source_file, extraction_method, detection_evidence, evidence_pointer, last_modified, source_type
		 FROM graph_edges ORDER BY source_id ASC, target_id ASC, id ASC`,
	)
	if err != nil {
		return fmt.Errorf("knowledge.Database.LoadGraph: query graph_edges: %w", err)
	}
	defer eRows.Close()

	for eRows.Next() {
		var id, sourceID, targetID, edgeTypeStr string
		var confidence float64
		var evidence, sourceFile, extractionMethod, detectionEvidence, evidencePointer, sourceTypeCol sql.NullString
		var lastModified sql.NullInt64
		if err := eRows.Scan(&id, &sourceID, &targetID, &edgeTypeStr, &confidence, &evidence,
			&sourceFile, &extractionMethod, &detectionEvidence, &evidencePointer, &lastModified, &sourceTypeCol); err != nil {
			return fmt.Errorf("knowledge.Database.LoadGraph: scan edge: %w", err)
		}
		e := &Edge{
			ID:                id,
			Source:            sourceID,
			Target:            targetID,
			Type:              EdgeType(edgeTypeStr),
			Confidence:        confidence,
			Evidence:          evidence.String,
			SourceFile:        sourceFile.String,
			ExtractionMethod:  extractionMethod.String,
			DetectionEvidence: detectionEvidence.String,
			EvidencePointer:   evidencePointer.String,
			LastModified:      lastModified.Int64,
			SourceType:        sourceTypeCol.String,
		}
		if e.SourceType == "" {
			e.SourceType = "markdown"
		}
		graph.Edges[id] = e
		graph.BySource[sourceID] = append(graph.BySource[sourceID], e)
		graph.ByTarget[targetID] = append(graph.ByTarget[targetID], e)
	}
	if err := eRows.Err(); err != nil {
		return fmt.Errorf("knowledge.Database.LoadGraph: iterate edges: %w", err)
	}

	return nil
}

// ─── code signal persistence ──────────────────────────────────────────────────

// SaveCodeSignals bulk-inserts raw code analysis signals into the code_signals
// provenance table. Each signal is recorded with its source component context.
// The operation runs inside a transaction for atomicity.
func (db *Database) SaveCodeSignals(signals []code.CodeSignal, sourceComponent string) error {
	if len(signals) == 0 {
		return nil
	}

	return transaction(db.conn, func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(
			`INSERT INTO code_signals
			 (source_component, target_component, signal_type, confidence, evidence, file_path, line_number, language)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		)
		if err != nil {
			return fmt.Errorf("prepare code_signals insert: %w", err)
		}
		defer stmt.Close()

		for _, sig := range signals {
			_, err := stmt.Exec(
				sourceComponent, sig.TargetComponent, sig.DetectionKind,
				sig.Confidence, sig.Evidence, sig.SourceFile, sig.LineNumber, sig.Language,
			)
			if err != nil {
				return fmt.Errorf("insert code_signal for %q->%q: %w", sourceComponent, sig.TargetComponent, err)
			}
		}
		return nil
	})
}

// ─── component mention persistence ───────────────────────────────────────────

// ComponentMention represents a single detection provenance record.
type ComponentMention struct {
	ComponentID      string  `json:"component_id"`
	FilePath         string  `json:"file_path"`
	HeadingHierarchy string  `json:"heading_hierarchy,omitempty"`
	DetectedBy       string  `json:"detected_by"`
	Confidence       float64 `json:"confidence"`
}

// SaveComponentMentions inserts a batch of component mentions into the database.
// Existing mentions for the given component IDs are cleared first.
func (db *Database) SaveComponentMentions(mentions []ComponentMention) error {
	if len(mentions) == 0 {
		return nil
	}
	return transaction(db.conn, func(tx *sql.Tx) error {
		// Collect unique component IDs to clear.
		ids := make(map[string]bool)
		for _, m := range mentions {
			ids[m.ComponentID] = true
		}
		for id := range ids {
			if _, err := tx.Exec(`DELETE FROM component_mentions WHERE component_id = ?`, id); err != nil {
				return fmt.Errorf("clear mentions for %q: %w", id, err)
			}
		}

		for _, m := range mentions {
			_, err := tx.Exec(
				`INSERT INTO component_mentions (component_id, file_path, heading_hierarchy, detected_by, confidence)
				 VALUES (?, ?, ?, ?, ?)`,
				m.ComponentID, m.FilePath, m.HeadingHierarchy, m.DetectedBy, m.Confidence,
			)
			if err != nil {
				return fmt.Errorf("insert mention for %q: %w", m.ComponentID, err)
			}
		}
		return nil
	})
}

// LoadComponentMentions reads all component mentions from the database and
// returns them grouped by component_id, sorted by confidence descending.
func (db *Database) LoadComponentMentions() (map[string][]ComponentMention, error) {
	result := make(map[string][]ComponentMention)

	rows, err := db.conn.Query(
		`SELECT component_id, file_path, heading_hierarchy, detected_by, confidence
		 FROM component_mentions
		 ORDER BY component_id, confidence DESC`)
	if err != nil {
		return result, fmt.Errorf("query component_mentions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var m ComponentMention
		if err := rows.Scan(&m.ComponentID, &m.FilePath, &m.HeadingHierarchy, &m.DetectedBy, &m.Confidence); err != nil {
			return result, fmt.Errorf("scan component_mention: %w", err)
		}
		result[m.ComponentID] = append(result[m.ComponentID], m)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate component_mentions: %w", err)
	}

	return result, nil
}

// ListComponentsByType returns all graph nodes with the given component type.
func (db *Database) ListComponentsByType(ct ComponentType) ([]*Node, error) {
	rows, err := db.conn.Query(
		`SELECT id, type, file, title, component_type FROM graph_nodes WHERE component_type = ? ORDER BY id ASC`,
		string(ct),
	)
	if err != nil {
		return nil, fmt.Errorf("knowledge.Database.ListComponentsByType: %w", err)
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		var id, nodeType, file string
		var title, compType sql.NullString
		if err := rows.Scan(&id, &nodeType, &file, &title, &compType); err != nil {
			return nil, fmt.Errorf("knowledge.Database.ListComponentsByType: scan: %w", err)
		}
		ct := ComponentType(compType.String)
		if ct == "" {
			ct = ComponentTypeUnknown
		}
		nodes = append(nodes, &Node{ID: id, Type: nodeType, Title: title.String, ComponentType: ct})
	}
	return nodes, rows.Err()
}

// ─── incremental update detection ────────────────────────────────────────────

// GetChanges scans root and compares the found files against the documents
// table.  It returns three lists:
//   - added:    files present on disk but not in the database.
//   - modified: files present in both but with a different content hash.
//   - deleted:  files in the database but not found on disk.
func (db *Database) GetChanges(root string) (added, modified, deleted []string, err error) {
	// Hash all markdown files under root.
	diskFiles := make(map[string]string) // relPath → hash
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable paths
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		diskFiles[filepath.ToSlash(rel)] = calculateContentHash(data)
		return nil
	})
	if walkErr != nil {
		return nil, nil, nil, fmt.Errorf("knowledge.Database.GetChanges: walk: %w", walkErr)
	}

	// Load stored documents.
	rows, queryErr := db.conn.Query(`SELECT id, content_hash FROM documents`)
	if queryErr != nil {
		return nil, nil, nil, fmt.Errorf("knowledge.Database.GetChanges: query: %w", queryErr)
	}
	defer rows.Close()

	dbFiles := make(map[string]string) // id → hash
	for rows.Next() {
		var id, hash string
		if scanErr := rows.Scan(&id, &hash); scanErr != nil {
			return nil, nil, nil, fmt.Errorf("knowledge.Database.GetChanges: scan: %w", scanErr)
		}
		dbFiles[id] = hash
	}
	if err = rows.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("knowledge.Database.GetChanges: iterate: %w", err)
	}

	for relPath, diskHash := range diskFiles {
		if dbHash, found := dbFiles[relPath]; !found {
			added = append(added, relPath)
		} else if diskHash != dbHash {
			modified = append(modified, relPath)
		}
	}
	for id := range dbFiles {
		if _, found := diskFiles[id]; !found {
			deleted = append(deleted, id)
		}
	}

	return added, modified, deleted, nil
}

// UpdateDocuments performs a partial update of the documents table.
//
// docs contains documents to add or replace (by their ID).
// deletedIDs lists document IDs to remove, cascading to index_entries.
// All changes are wrapped in a single transaction.
func (db *Database) UpdateDocuments(docs []Document, deletedIDs []string) error {
	return transaction(db.conn, func(tx *sql.Tx) error {
		now := time.Now().UnixNano()

		// Delete removed documents (cascade removes index_entries).
		for _, id := range deletedIDs {
			if _, err := tx.Exec(`DELETE FROM documents WHERE id=?`, id); err != nil {
				return fmt.Errorf("delete document %q: %w", id, err)
			}
		}

		// Upsert changed/added documents.
		for _, doc := range docs {
			_, err := tx.Exec(
				`INSERT OR REPLACE INTO documents
				 (id, path, title, content_hash, last_modified, indexed_at)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				doc.ID, doc.Path, doc.Title, doc.ContentHash,
				doc.LastModified.UnixNano(), now,
			)
			if err != nil {
				return fmt.Errorf("upsert document %q: %w", doc.ID, err)
			}
		}

		return nil
	})
}

// RebuildIndex drops and recreates all index_entries from the current documents
// and bm25_stats tables.  Callers must pass the freshly-built Index to supply
// the new posting data.
func (db *Database) RebuildIndex(idx *Index) error {
	return db.SaveIndex(idx) // SaveIndex does a full replace already
}

// ─── database queries ─────────────────────────────────────────────────────────

// GetDocument retrieves a document row by ID.
// Returns nil and a non-nil error when the document is not found.
func (db *Database) GetDocument(id string) (*Document, error) {
	var d Document
	var lastMod int64
	err := db.conn.QueryRow(
		`SELECT id, path, title, content_hash, last_modified FROM documents WHERE id=?`,
		id,
	).Scan(&d.ID, &d.Path, &d.Title, &d.ContentHash, &lastMod)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("knowledge.Database.GetDocument: %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("knowledge.Database.GetDocument: %w", err)
	}
	d.RelPath = filepath.FromSlash(id)
	d.LastModified = time.Unix(0, lastMod)
	return &d, nil
}

// GetNode retrieves a graph node by ID.
// Returns nil and a non-nil error when the node is not found.
func (db *Database) GetNode(id string) (*Node, error) {
	var n Node
	var title sql.NullString
	err := db.conn.QueryRow(
		`SELECT id, type, title FROM graph_nodes WHERE id=?`, id,
	).Scan(&n.ID, &n.Type, &title)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("knowledge.Database.GetNode: %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("knowledge.Database.GetNode: %w", err)
	}
	n.Title = title.String
	return &n, nil
}

// GetEdges returns outgoing ("out") or incoming ("in") edges for nodeID.
// direction must be "out" or "in"; any other value returns an error.
func (db *Database) GetEdges(nodeID string, direction string) ([]*Edge, error) {
	var query string
	switch direction {
	case "out":
		query = `SELECT id, source_id, target_id, type, confidence, evidence
		         FROM graph_edges WHERE source_id=?`
	case "in":
		query = `SELECT id, source_id, target_id, type, confidence, evidence
		         FROM graph_edges WHERE target_id=?`
	default:
		return nil, fmt.Errorf("knowledge.Database.GetEdges: direction must be 'in' or 'out', got %q", direction)
	}

	rows, err := db.conn.Query(query, nodeID)
	if err != nil {
		return nil, fmt.Errorf("knowledge.Database.GetEdges: %w", err)
	}
	defer rows.Close()

	var edges []*Edge
	for rows.Next() {
		var e Edge
		var evidence sql.NullString
		if err := rows.Scan(&e.ID, &e.Source, &e.Target, (*string)(&e.Type), &e.Confidence, &evidence); err != nil {
			return nil, fmt.Errorf("knowledge.Database.GetEdges: scan: %w", err)
		}
		e.Evidence = evidence.String
		edges = append(edges, &e)
	}
	return edges, rows.Err()
}

// SearchTerms performs a simple SQL full-text search against the inverted
// index.  It returns up to topK SearchResult entries ordered by frequency sum.
func (db *Database) SearchTerms(terms []string, topK int) ([]SearchResult, error) {
	if len(terms) == 0 {
		return nil, nil
	}

	// Build a query that sums frequency across matched terms.
	placeholders := make([]string, len(terms))
	args := make([]interface{}, len(terms))
	for i, t := range terms {
		placeholders[i] = "?"
		args[i] = t
	}
	inClause := strings.Join(placeholders, ",")

	query := fmt.Sprintf(`
		SELECT d.id, d.path, d.title, SUM(ie.frequency) AS score
		FROM index_entries ie
		JOIN documents d ON ie.doc_id = d.id
		WHERE ie.term IN (%s)
		GROUP BY d.id
		ORDER BY score DESC
		LIMIT ?`, inClause)

	args = append(args, topK)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("knowledge.Database.SearchTerms: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var title sql.NullString
		if err := rows.Scan(&r.DocID, &r.Path, &title, &r.Score); err != nil {
			return nil, fmt.Errorf("knowledge.Database.SearchTerms: scan: %w", err)
		}
		r.Title = title.String
		r.RelPath = filepath.FromSlash(r.DocID)
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetServices returns all nodes whose type is "service".
func (db *Database) GetServices() ([]Node, error) {
	rows, err := db.conn.Query(
		`SELECT id, type, title FROM graph_nodes WHERE type='service'`,
	)
	if err != nil {
		return nil, fmt.Errorf("knowledge.Database.GetServices: %w", err)
	}
	defer rows.Close()

	var services []Node
	for rows.Next() {
		var n Node
		var title sql.NullString
		if err := rows.Scan(&n.ID, &n.Type, &title); err != nil {
			return nil, fmt.Errorf("knowledge.Database.GetServices: scan: %w", err)
		}
		n.Title = title.String
		services = append(services, n)
	}
	return services, rows.Err()
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// calculateContentHash returns the hex-encoded MD5 digest of data.
// Used here for content-change detection (not cryptographic security).
func calculateContentHash(data []byte) string {
	h := md5.Sum(data) //nolint:gosec
	return hex.EncodeToString(h[:])
}

// hashFile reads the file at path and returns its MD5 content hash.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("knowledge.hashFile: read %q: %w", path, err)
	}
	return calculateContentHash(data), nil
}
