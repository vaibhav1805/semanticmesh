# Phase 2, Plan 3: Provenance Schema & Persistence (Wave 2)

**Frontmatter:**
```
phase: 2
plan: 03-provenance
wave: 2
depends_on: [02-01]
blocks: [02-05]
autonomous: true
files_modified:
  - internal/knowledge/edge.go
  - internal/knowledge/db.go
  - internal/knowledge/graph.go
  - internal/knowledge/*_test.go
```

---

## Plan Summary

**Requirement:** REL-02 (Add metadata provenance to SQLite schema: source file, extraction method, last-modified timestamp)

**Goal:** Extend Edge struct and SQLite schema to track complete provenance metadata so agents can audit relationship origins and assess trustworthiness during incident response.

**Scope:**
- Extend Edge struct with provenance fields (source_file, extraction_method, detection_evidence, evidence_pointer, last_modified)
- Create SQLite v3→v4 migration adding provenance columns to graph_edges
- Update SaveGraph/LoadGraph to persist and retrieve provenance
- Add validation ensuring non-null provenance fields

**Depends on:** Plan 1 (aggregation foundation must exist)

---

## Tasks

### Task 1: Extend Edge Struct with Provenance Fields

**Action:** Modify Edge struct in `internal/knowledge/edge.go` to include provenance metadata.

**Specific steps:**
1. Add provenance fields to Edge struct:
   ```go
   type Edge struct {
       // Existing fields
       ID         string
       Source     string
       Target     string
       Type       EdgeType
       Confidence float64
       Evidence   string

       // NEW provenance fields
       SourceFile        string    // relative path to source (e.g., "services/payment.md")
       ExtractionMethod  string    // "explicit-link", "co-occurrence", "structural", "NER", "semantic", "LLM"
       DetectionEvidence string    // ~200 char contextual snippet
       EvidencePointer   string    // file:line or byte offset (optional, e.g., "service.yaml:42")
       LastModified      int64     // Unix timestamp (file mtime or detection time)
       AggregationCount  int       // # signals merged (from Plan 1)
   }
   ```

2. Update NewEdge() constructor:
   - Accept all provenance parameters
   - Validate ExtractionMethod against allowed values (enum)
   - Validate SourceFile is non-empty
   - Allow EvidencePointer to be empty (optional)
   - Ensure LastModified > 0 (or use current time as fallback)

3. Add EdgeValidationError for invalid provenance:
   ```go
   func ValidateEdge(e *Edge) error {
       if e.SourceFile == "" {
           return fmt.Errorf("edge validation: source_file required")
       }
       if !isValidExtractionMethod(e.ExtractionMethod) {
           return fmt.Errorf("edge validation: invalid extraction_method %q", e.ExtractionMethod)
       }
       // ... more validation
   }
   ```

4. Document field semantics:
   - SourceFile: audit metadata (where relationship originated), not necessarily retrievable in containers
   - DetectionEvidence: complete, standalone snippet so agents understand relationships without file access
   - EvidencePointer: optional; not all algorithms produce precise line numbers

**Verification:**
- Code compiles
- NewEdge() validates all provenance fields
- Edge struct is serializable/deserializable

---

### Task 2: Create SQLite v3→v4 Migration

**Action:** Add schema migration in `db.go` to add provenance columns to graph_edges.

**Specific steps:**
1. Add migration function to db.go:
   ```go
   func migrateV3ToV4(db *sql.DB) error {
       statements := []string{
           "ALTER TABLE graph_edges ADD COLUMN source_file TEXT",
           "ALTER TABLE graph_edges ADD COLUMN extraction_method TEXT",
           "ALTER TABLE graph_edges ADD COLUMN detection_evidence TEXT",
           "ALTER TABLE graph_edges ADD COLUMN evidence_pointer TEXT",
           "ALTER TABLE graph_edges ADD COLUMN last_modified INTEGER",
           "ALTER TABLE graph_edges ADD COLUMN aggregation_count INTEGER DEFAULT 1",
       }
       return executeStatements(db, statements)
   }
   ```

2. Update Migrate() function:
   - Add condition: `if version < 4: call migrateV3ToV4()`
   - Update SchemaVersion constant = 4
   - Update metadata table version entry

3. Make migration idempotent:
   - Use `ALTER TABLE IF NOT EXISTS` or check column existence first
   - Test against existing test-data database without errors

4. Data safety:
   - New columns start NULL/empty
   - Old edges (from Phase 1) have provenance fields NULL
   - Phase 3+ export will populate provenance; import handles NULL gracefully

**Verification:**
- Migration runs without errors
- graph_edges table has new columns
- Existing edges still queryable
- Old test-data database can be migrated

---

### Task 3: Update SaveGraph to Persist Provenance

**Action:** Modify SaveGraph() in `db.go` to write provenance fields to graph_edges.

**Specific steps:**
1. Update INSERT statement in SaveGraph batch loop:
   ```go
   // Before:
   INSERT INTO graph_edges (id, source_id, target_id, type, confidence, evidence)
   VALUES (?, ?, ?, ?, ?, ?)

   // After:
   INSERT OR REPLACE INTO graph_edges
   (id, source_id, target_id, type, confidence, evidence,
    source_file, extraction_method, detection_evidence, evidence_pointer, last_modified, aggregation_count)
   VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
   ```

2. Bind all provenance parameters:
   - edge.SourceFile
   - edge.ExtractionMethod
   - edge.DetectionEvidence
   - edge.EvidencePointer
   - edge.LastModified
   - edge.AggregationCount

3. Handle NULL fields gracefully:
   - Convert empty strings to NULL in SQL
   - Preserve empty EvidencePointer as NULL

4. Maintain batch processing (1000 edges per batch)

**Verification:**
- SaveGraph compiles and runs
- Provenance fields appear in database after save
- Round-trip test (save → load) preserves all fields

---

### Task 4: Update LoadGraph to Retrieve Provenance

**Action:** Modify LoadGraph() in `db.go` to read provenance fields.

**Specific steps:**
1. Update SELECT statement:
   ```go
   SELECT id, source_id, target_id, type, confidence, evidence,
          source_file, extraction_method, detection_evidence, evidence_pointer, last_modified, aggregation_count
   FROM graph_edges
   ```

2. Populate Edge struct from query results:
   - edge.SourceFile = rows.SourceFile
   - edge.ExtractionMethod = rows.ExtractionMethod
   - ... (all provenance fields)

3. Handle NULL/missing values (backward compatibility):
   - If SourceFile is NULL, use Edge.ID as fallback (unique identifier)
   - If ExtractionMethod is NULL, use "unknown" as fallback
   - If LastModified is NULL, use current time as fallback

4. Validate loaded edges:
   - After loading, run ValidateEdge() on each edge
   - Log warnings for invalid provenance (but don't fail load)

**Verification:**
- LoadGraph compiles and runs
- Loaded edges have provenance fields populated
- Round-trip test passes (save → load → save produces identical schema)
- Backward compatibility: old edges (NULL provenance) load without error

---

### Task 5: Add Provenance Validation Tests

**Action:** Create `edge_test.go` with comprehensive provenance validation tests.

**Specific steps:**
1. Test functions:
   ```go
   func TestEdgeValidation_RequiresSourceFile()
   func TestEdgeValidation_RequiresExtractionMethod()
   func TestEdgeValidation_AllowsNullEvidencePointer()
   func TestEdgeValidation_RejectsInvalidMethod()
   func TestNewEdge_ValidProvenance()
   func TestNewEdge_InvalidProvenance()
   func TestEdgeProvenance_LastModifiedFallback()
   ```

2. Example test:
   ```go
   edge := NewEdge("service-a", "service-b", "depends-on", 0.8)
   edge.SourceFile = ""
   err := ValidateEdge(edge)
   assert.NotNil(t, err)
   assert.Contains(t, err.Error(), "source_file required")
   ```

3. Round-trip test:
   ```go
   // Create edge with provenance, save to DB, load, verify
   original := Edge{
       SourceFile: "services/payment.md",
       ExtractionMethod: "co-occurrence",
       DetectionEvidence: "calls primary-db for transaction storage",
       LastModified: time.Now().Unix(),
   }
   // save, load, assert all fields equal
   ```

**Verification:**
- All validation tests pass
- Coverage >= 85% for edge.go
- Round-trip tests show no data loss

---

### Task 6: Integration Test with Test-Data Persistence

**Action:** Create integration test validating end-to-end provenance persistence.

**Specific steps:**
1. Create test in `graph_test.go`:
   ```go
   func TestIntegration_ProvenanceRoundTrip_WithTestData()
   ```

2. Test flow:
   - Load test-data corpus
   - Run discovery pipeline (Plan 1) with pageindex + confidence tiers (Plan 2)
   - Build graph with edges containing provenance
   - Save to SQLite with migration v3→v4
   - Load graph back
   - Verify all edges have non-null provenance fields
   - Query: `SELECT COUNT(*) FROM graph_edges WHERE source_file IS NOT NULL`
   - Assert: count > 0 (all edges have source_file after export)

3. Backward compatibility test:
   - Manually insert edge with NULL provenance (simulating old export)
   - Load graph
   - Verify edge loads without error
   - Verify fallback values used correctly

**Verification:**
- Integration test passes
- Provenance fields persist correctly
- Backward compatibility confirmed
- Test report shows provenance field population rates

---

## Verification Criteria

**What "done" looks like:**

1. ✓ **Edge struct extended:** All provenance fields added. NewEdge() validates.

2. ✓ **SQLite migration works:** v3→v4 runs without error. New columns exist and queryable.

3. ✓ **SaveGraph persists provenance:** Edges written to database include source_file, extraction_method, etc.

4. ✓ **LoadGraph retrieves provenance:** Loaded edges have provenance fields populated. Backward compatibility ensured.

5. ✓ **Unit tests pass:** edge_test.go validates all provenance fields and error conditions. Coverage >= 85%.

6. ✓ **Integration test validates:** With test-data, all edges have non-null provenance after round-trip. Backward compatibility confirmed.

7. ✓ **No regressions:** Phase 1 tests still pass. Plans 1–2 (aggregation + tiers) still work.

8. ✓ **Documentation:** Code comments explain provenance field semantics (especially SourceFile as audit metadata vs. fetchy paths).

---

## Must-Haves (Phase Goal Backward)

From phase goal: "provenance infrastructure that prevents false edges from misleading agents"

This plan delivers:
- ✓ Complete provenance metadata (source file, method, evidence) in schema
- ✓ Persistent storage (SQLite v3→v4 migration)
- ✓ Retrieval mechanism (LoadGraph populates provenance for queries)
- ✓ Validation (non-null constraint, extraction method enum)

---

## Dependency Notes

**Depends on:** Plan 1 (aggregation foundation; Location metadata from pageindex flows here)

**Enables:** Plan 5 (query interface returns provenance in JSON)

---

*Plan created: 2026-03-19*
*Autonomous execution: Yes*
