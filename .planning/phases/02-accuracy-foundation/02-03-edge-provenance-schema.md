---
phase: 02-accuracy-foundation
plan: 03
type: execute
wave: 2
depends_on:
  - 02-02
files_modified:
  - internal/knowledge/types.go
  - internal/knowledge/db.go
  - internal/knowledge/db_test.go
autonomous: true
requirements:
  - REL-02

must_haves:
  truths:
    - "Every relationship edge has provenance metadata: source_file, extraction_method, detection_evidence, evidence_pointer"
    - "All provenance fields are non-null and populated during discovery"
    - "SQLite schema includes columns for source_file, extraction_method, detection_evidence, evidence_pointer, last_modified on graph_edges table"
    - "Edge struct persists to SQLite and round-trips with all provenance intact"
    - "Schema migration v3→v4 is atomic and backward-compatible"
  artifacts:
    - path: "internal/knowledge/types.go"
      provides: "Extended Edge struct with provenance fields"
      exports: ["Edge", "NewEdge", "ValidateEdge"]
      min_lines: 50
    - path: "internal/knowledge/db.go"
      provides: "Schema migration v3→v4, updated SaveGraph() and LoadGraph()"
      exports: ["Migrate", "SaveGraph", "LoadGraph"]
      min_lines: 100
    - path: "internal/knowledge/db_test.go"
      provides: "Round-trip persistence tests for provenance"
      exports: ["TestSaveLoadGraph_PreservesProvenance"]
  key_links:
    - from: "Edge struct"
      to: "SQLite graph_edges table"
      via: "SaveGraph() INSERT with all provenance columns"
      pattern: "source_file.*extraction_method.*detection_evidence"
    - from: "DiscoverySignal (from Plan 1)"
      to: "Edge struct provenance fields"
      via: "NewEdge() constructor populated from signal location and metadata"
      pattern: "NewEdge.*Location.*Evidence"
    - from: "Schema migration"
      to: "graph_edges columns"
      via: "v3→v4 adds source_file, extraction_method, detection_evidence, evidence_pointer, last_modified"
      pattern: "ALTER TABLE graph_edges ADD COLUMN"

---

<objective>
Extend the Edge struct and SQLite schema to capture provenance metadata for every relationship: where it was detected (source_file), how (extraction_method), what evidence supports it (detection_evidence), and when (last_modified). Implement schema migration from v3→v4 and ensure round-trip persistence.

Purpose: REL-02 requirement; enables agents to understand why a relationship exists and assess credibility. Provenance provides audit trail and source of truth.

Output: Extended Edge struct, v3→v4 schema migration, updated SaveGraph/LoadGraph functions, round-trip persistence tests.
</objective>

<execution_context>
@/Users/flurryhead/.claude/get-shit-done/workflows/execute-plan.md
@/Users/flurryhead/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/ROADMAP.md
@.planning/phases/02-accuracy-foundation/02-RESEARCH.md
@.planning/phases/02-accuracy-foundation/02-CONTEXT.md
@internal/knowledge/types.go
@internal/knowledge/db.go
@.planning/phases/01-component-model/01-01-SUMMARY.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Extend Edge struct with provenance fields</name>
  <files>internal/knowledge/types.go</files>
  <action>
Update Edge struct in types.go:

1. Add new fields to Edge struct (after existing fields Source, Target, Type, Confidence, Evidence):
   ```go
   type Edge struct {
       // existing fields
       ID         string
       Source     string
       Target     string
       Type       EdgeType
       Confidence float64
       Evidence   string

       // NEW provenance fields (REL-02)
       SourceFile        string    // path where relationship was detected (e.g., "docs/service.yaml")
       ExtractionMethod  string    // "explicit-link", "co-occurrence", "structural", "NER", "semantic", "LLM"
       DetectionEvidence string    // ~200 char contextual snippet (e.g., "calls primary-db for transaction storage")
       EvidencePointer   string    // file:line format optional (e.g., "service.yaml:42")
       LastModified      int64     // Unix timestamp of detection or file mtime
   }
   ```

2. Add NewEdge constructor that validates provenance:
   ```go
   func NewEdge(source, target, edgeType, sourceFile, extractionMethod, detectionEvidence, evidencePointer string, confidence float64, lastModified int64) *Edge {
       // Validate required provenance fields are non-empty
       // Validate extractionMethod is one of: explicit-link, co-occurrence, structural, NER, semantic, LLM
       // Validate confidence in [0.4, 1.0] (use existing confidence validation from Plan 2)
       // Validate sourceFile is relative path (no leading /)
       // Return Edge or error
   }
   ```

3. Add ValidateEdge(e *Edge) error function:
   - sourceFile non-empty and relative
   - extractionMethod one of valid methods
   - confidence in [0.4, 1.0]
   - detectionEvidence non-empty
   - lastModified > 0

4. Add String() method for logging (include all provenance fields in output)

5. Update existing code that creates Edge instances to include new fields (search for NewEdge or direct struct init)

Backward compatibility: Existing code should still work; new fields have zero values. Add comments explaining when fields are populated.
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestEdge -v
Expected:
  - TestNewEdge_Valid: Creates edge with all provenance
  - TestNewEdge_InvalidMethod: Rejects invalid extraction_method
  - TestNewEdge_BadSourceFile: Rejects absolute paths or empty SourceFile
  - TestValidateEdge: All validations pass

Also run: go build ./cmd/graphmd — no syntax errors
  </verify>
  <done>
- Edge struct includes SourceFile, ExtractionMethod, DetectionEvidence, EvidencePointer, LastModified
- NewEdge constructor validates all provenance fields
- ValidateEdge function enforces non-null and correct format
- String() method includes provenance in output for debugging
- Existing code compiles (may have zero-value provenance until Plan 1/2 integration)
  </done>
</task>

<task type="auto">
  <name>Task 2: Implement schema migration v3→v4 with provenance columns</name>
  <files>internal/knowledge/db.go</files>
  <action>
Update db.go with schema migration:

1. Update SchemaVersion constant:
   - const SchemaVersion = 4 (currently 3)

2. Add migration function in Migrate() (find existing migration pattern):
   ```go
   // In Migrate() function, add case for v3→v4:
   case 3:
       // Add provenance columns to graph_edges table
       statements := []string{
           `ALTER TABLE graph_edges ADD COLUMN source_file TEXT`,
           `ALTER TABLE graph_edges ADD COLUMN extraction_method TEXT`,
           `ALTER TABLE graph_edges ADD COLUMN detection_evidence TEXT`,
           `ALTER TABLE graph_edges ADD COLUMN evidence_pointer TEXT`,
           `ALTER TABLE graph_edges ADD COLUMN last_modified INTEGER`,
       }
       for _, stmt := range statements {
           if _, err := db.Exec(stmt); err != nil {
               return fmt.Errorf("migration v3→v4 failed: %w", err)
           }
       }
       // Update schema version in metadata table
       if _, err := db.Exec(`UPDATE metadata SET value = '4' WHERE key = 'schema_version'`); err != nil {
           return fmt.Errorf("update schema version failed: %w", err)
       }
       fallthrough
   case 4:
       // Current version; no-op
       return nil
   ```

3. Ensure all ALTER statements are idempotent (or wrapped in IF NOT EXISTS where applicable)

4. Add migration to a transaction() wrapper for atomicity (existing pattern in codebase)

5. Add comment: "v3→v4: Add provenance tracking to edges (source_file, extraction_method, detection_evidence, evidence_pointer, last_modified)"

Reference existing migrations (v1→v2, v2→v3) for pattern. No data loss; columns are added nullable initially (agents can work with null values for legacy edges).
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestMigration -v
Expected:
  - TestMigration_V3toV4: Starting with v3 database, migration succeeds
  - TestMigration_V4Idempotent: Running migration twice does not error
  - TestMigration_SchemaVersion: After v3→v4, SELECT MAX(schema_version) returns 4

Manual test: Create test database at v3, run Migrate(), verify schema has new columns:
  go test ./internal/knowledge -run TestMigrateExistingDatabase -v
  </verify>
  <done>
- SchemaVersion updated to 4
- Migration v3→v4 adds 5 columns to graph_edges
- Atomicity ensured via transaction() wrapper
- Idempotent and backward-compatible
- Schema version tracked in metadata table
  </done>
</task>

<task type="auto">
  <name>Task 3: Update SaveGraph and LoadGraph for provenance persistence</name>
  <files>internal/knowledge/db.go</files>
  <action>
Update SaveGraph() and LoadGraph() functions:

1. In SaveGraph (currently around line 804-875):
   - Find the INSERT INTO graph_edges statement
   - Extend to include new columns:
     ```sql
     INSERT OR REPLACE INTO graph_edges
     (id, source_id, target_id, type, confidence, evidence,
      source_file, extraction_method, detection_evidence, evidence_pointer, last_modified)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
     ```
   - Bind all new provenance values from edge struct
   - Add validation before insert: ValidateEdge(edge) must pass (from Plan 1)
   - Log warning if edge has null provenance (log but don't fail; backward compat)

2. In LoadGraph (find where graph_edges rows are loaded):
   - Update SELECT to include new columns:
     ```sql
     SELECT id, source_id, target_id, type, confidence, evidence,
            source_file, extraction_method, detection_evidence, evidence_pointer, last_modified
     FROM graph_edges
     ```
   - Scan all new fields into Edge struct
   - Handle NULL values gracefully (use sql.NullString or equivalent for optional fields)

3. Add helper function EdgeFromRow() to unpack database row into Edge struct (if not already exists)

4. Update comments in SaveGraph/LoadGraph explaining provenance fields

Testing: Round-trip test (Plan 3 Task 3) will verify.
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestSaveLoad -v
Expected:
  - TestSaveGraph_Provenance: SaveGraph includes all provenance fields in INSERT
  - TestLoadGraph_Provenance: LoadGraph correctly reads provenance from database

Sampling test will verify in full integration (Task 3).
  </verify>
  <done>
- SaveGraph updated to persist all 5 provenance columns
- LoadGraph updated to read all provenance columns
- NULL values handled gracefully for backward compatibility
- Validation checks edge before persistence
  </done>
</task>

<task type="auto">
  <name>Task 4: Round-trip persistence test with provenance</name>
  <files>internal/knowledge/db_test.go</files>
  <action>
Create comprehensive round-trip test:

Test TestSaveLoadGraph_PreservesProvenance:
1. Create test Edge with all provenance fields populated:
   ```go
   edge := &Edge{
       ID: "edge-1",
       Source: "payment-api",
       Target: "primary-db",
       Type: "dependency",
       Confidence: 0.8,
       Evidence: "calls for transaction storage",
       // NEW provenance
       SourceFile: "docs/service.yaml",
       ExtractionMethod: "explicit-link",
       DetectionEvidence: "calls primary-db for transaction storage",
       EvidencePointer: "service.yaml:42",
       LastModified: time.Now().Unix(),
   }
   ```

2. Create test database with v3→v4 migration
3. Call SaveGraph() with edge
4. Call LoadGraph() to retrieve
5. Assert all provenance fields match original:
   - SourceFile == "docs/service.yaml"
   - ExtractionMethod == "explicit-link"
   - DetectionEvidence == "calls primary-db for transaction storage"
   - EvidencePointer == "service.yaml:42"
   - LastModified same timestamp

6. Test with multiple extraction methods: "co-occurrence", "NER", "semantic", "LLM"

7. Test backward compatibility: Load v3 database edge (no provenance columns), verify NULL handling

Run test with: go test ./internal/knowledge -run TestSaveLoadGraph_PreservesProvenance -v
Expected: All assertions pass, no NULL pointer errors
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestSaveLoadGraph_PreservesProvenance -v
Expected: Test passes for all extraction methods

Also run: go test ./internal/knowledge -run TestLoadGraph_BackwardCompat -v
Expected: Loading v3-era edges with NULL provenance succeeds (no crashes)
  </verify>
  <done>
- Round-trip persistence works: Edge with provenance → SQLite → Edge
- All 5 provenance fields preserved exactly
- Multiple extraction methods tested
- Backward compatibility: NULL provenance values handled gracefully
- No data loss or corruption during migration and persistence
  </done>
</task>

</tasks>

<verification>
**Comprehensive checks before marking complete:**

1. **Schema Correctness:** SQLite schema includes all 5 new columns on graph_edges table. Run: `sqlite3 test.db ".schema graph_edges"` shows source_file, extraction_method, detection_evidence, evidence_pointer, last_modified columns.

2. **Migration Atomicity:** Migrate v3→v4 in a transaction. If any ALTER fails, entire migration rolls back (test with read-only database or permission error scenario).

3. **Data Integrity:** No existing data is lost during migration. Run on test-data corpus database (if exists), verify all existing edges are preserved.

4. **Round-Trip Accuracy:** Edge → SaveGraph → LoadGraph → Edge produces identical Edge struct (all provenance fields match byte-for-byte for string fields, timestamp matches).

5. **Backward Compatibility:** Loading edges from v3 database (no provenance columns) does not crash. NULL values handled gracefully.

6. **Validation:** NewEdge() rejects invalid extraction_method values. ValidateEdge() ensures all required fields are present before persistence.
</verification>

<success_criteria>
- REL-02 satisfied: Provenance metadata tracked and persisted
- Edge struct includes SourceFile, ExtractionMethod, DetectionEvidence, EvidencePointer, LastModified
- SQLite graph_edges table has columns for all 5 provenance fields
- Schema migration v3→v4 is atomic and backward-compatible
- SaveGraph() and LoadGraph() round-trip all provenance correctly
- ValidateEdge() ensures data quality before persistence
- Unit tests pass with round-trip verification
- No breaking changes to Phase 1 or existing graph structures
</success_criteria>

<output>
After completion, create `.planning/phases/02-accuracy-foundation/02-03-SUMMARY.md` documenting:
- Edge struct extension with 5 provenance fields
- Schema migration v3→v4 with atomic changes
- SaveGraph/LoadGraph updates for provenance persistence
- Round-trip test results verifying data integrity
- Backward compatibility with v3 databases
</output>
