---
phase: 02-accuracy-foundation
plan: 03
subsystem: database
tags: [sqlite, provenance, schema-migration, edge-tracking]

# Dependency graph
requires:
  - phase: 02-accuracy-foundation
    provides: "Confidence tier system and schema v3"
provides:
  - "Edge struct with 5 provenance fields (SourceFile, ExtractionMethod, DetectionEvidence, EvidencePointer, LastModified)"
  - "Schema migration v3->v4 with provenance columns on graph_edges"
  - "Round-trip persistence of provenance through SaveGraph/LoadGraph"
  - "ValidateEdge() for provenance data quality"
affects: [02-05-query-interface, 03-extract-export]

# Tech tracking
tech-stack:
  added: []
  patterns: ["nullable provenance columns with nullIfEmpty/nullIfZero helpers", "lenient validation (zero-value provenance passes)"]

key-files:
  created: ["internal/knowledge/edge_provenance_test.go"]
  modified: ["internal/knowledge/edge.go", "internal/knowledge/db.go", "internal/knowledge/component_types_test.go"]

key-decisions:
  - "Provenance validation is lenient: edges with all-zero provenance pass ValidateEdge()"
  - "Provenance columns stored as NULL in SQLite when empty (not empty string)"
  - "6 valid extraction methods: explicit-link, co-occurrence, structural, NER, semantic, LLM"

patterns-established:
  - "nullIfEmpty/nullIfZero helpers for clean NULL insertion in SQLite"
  - "Provenance fields optional on Edge struct for backward compatibility"

requirements-completed: [REL-02]

# Metrics
duration: 15min
completed: 2026-03-19
---

# Phase 2 Plan 3: Edge Provenance Summary

**Edge provenance tracking with 5 fields on Edge struct, SQLite schema v3->v4 migration, and round-trip persistence with backward-compatible NULL handling**

## Performance

- **Duration:** 15 min
- **Started:** 2026-03-19T14:44:30Z
- **Completed:** 2026-03-19T15:00:10Z
- **Tasks:** 4
- **Files modified:** 4

## Accomplishments
- Extended Edge struct with SourceFile, ExtractionMethod, DetectionEvidence, EvidencePointer, LastModified provenance fields
- Implemented atomic schema migration v3->v4 adding 5 columns to graph_edges table
- Updated SaveGraph/LoadGraph for full provenance round-trip with NULL handling
- Comprehensive test suite: 12 tests covering round-trip, backward compatibility, validation, and migration

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend Edge struct with provenance fields** - `817c9f8` (feat) - combined with 02-04 commit
2. **Task 2: Schema migration v3->v4** - `c3c918c` (feat)
3. **Task 3: Update SaveGraph/LoadGraph** - `72aae30` (feat)
4. **Task 4: Round-trip persistence tests** - `d99ffca` (test)

## Files Created/Modified
- `internal/knowledge/edge.go` - Added 5 provenance fields, ValidateEdge(), IsValidExtractionMethod(), validExtractionMethods
- `internal/knowledge/db.go` - SchemaVersion 4, migrateV3ToV4(), updated SaveGraph/LoadGraph with provenance columns, nullIfEmpty/nullIfZero helpers
- `internal/knowledge/edge_provenance_test.go` - 12 tests: round-trip, multiple methods, backward compat, validation, migration
- `internal/knowledge/component_types_test.go` - Updated schema version assertions from 3 to 4

## Decisions Made
- Provenance validation is lenient: ValidateEdge() only enforces rules when at least one provenance field is set. This enables gradual migration.
- Provenance columns stored as SQL NULL (not empty strings) via nullIfEmpty/nullIfZero helpers, enabling proper IS NULL queries.
- Task 1 was committed as part of 02-04's edge.go changes since both executors modified the same file concurrently.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Edge struct in edge.go not types.go**
- **Found during:** Task 1
- **Issue:** Plan specified types.go but Edge struct lives in edge.go
- **Fix:** Applied changes to edge.go instead
- **Files modified:** internal/knowledge/edge.go
- **Verification:** Build passes, tests pass

**2. [Rule 3 - Blocking] Concurrent modification with Plan 02-04**
- **Found during:** Task 1
- **Issue:** Plan 02-04 executor was simultaneously editing edge.go and committed first
- **Fix:** My edits were applied on top and included in 02-04's commit (817c9f8)
- **Files modified:** internal/knowledge/edge.go
- **Verification:** All provenance fields present in HEAD, all tests pass

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both deviations were mechanical (file location, concurrent editing). No scope change.

## Issues Encountered
None beyond the deviations noted above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Provenance columns available for query interface (Plan 05) to expose in API output
- Discovery algorithms can now populate Edge provenance fields when creating edges
- Export pipeline (Phase 3) will include provenance in exported graph data

---
*Phase: 02-accuracy-foundation*
*Completed: 2026-03-19*
