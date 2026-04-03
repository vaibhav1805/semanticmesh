# Phase 2, Plan 2: Confidence Tier System (Wave 2)

**Frontmatter:**
```
phase: 2
plan: 02-confidence
wave: 2
depends_on: [02-01]
blocks: [02-05]
autonomous: true
files_modified:
  - internal/knowledge/confidence.go (NEW)
  - internal/knowledge/types.go
  - internal/knowledge/discovery_orchestration.go
  - internal/knowledge/*_test.go
```

---

## Plan Summary

**Requirement:** REL-01 (Implement relationship confidence tiers: 7-tier system mapping raw scores [0.4–1.0] to semantic names)

**Goal:** Define and integrate 7-level confidence tier system so agents can assess trustworthiness of detected relationships using both numeric thresholds and human-readable tier names.

**Scope:**
- Define 7 confidence tiers (explicit, strong-inference, moderate, weak, semantic, threshold)
- Implement mapping functions: score↔tier
- Update discovery algorithms to use tier-aware confidence
- Support CLI filtering by --min-tier and --min-confidence

**Depends on:** Plan 1 (aggregation foundation must exist first)

---

## Tasks

### Task 1: Define Confidence Tier System

**Action:** Create `internal/knowledge/confidence.go` with tier definitions and mapping functions.

**Specific steps:**
1. Define ConfidenceTier type and constants:
   ```go
   type ConfidenceTier string

   const (
       TierExplicit        ConfidenceTier = "explicit"          // 1.0
       TierStrongInference ConfidenceTier = "strong-inference"  // 0.8
       TierModerate        ConfidenceTier = "moderate"          // 0.6
       TierWeak            ConfidenceTier = "weak"              // 0.5
       TierSemantic        ConfidenceTier = "semantic"          // 0.45
       TierThreshold       ConfidenceTier = "threshold"         // 0.4
   )
   ```

2. Implement mapping functions:
   ```go
   // ScoreToTier maps [0.4, 1.0] → tier name
   func ScoreToTier(score float64) ConfidenceTier

   // TierToScore maps tier → canonical score (for aggregation)
   func TierToScore(tier ConfidenceTier) float64

   // IsSufficientConfidence checks if score >= minimum threshold
   func IsSufficientConfidence(score float64, minScore float64) bool
   ```

3. Add validation:
   - Reject scores < 0.4 or > 1.0 (clamp/warn)
   - Unknown tier names return error
   - Tier boundaries: explicit >= 0.95, strong >= 0.75, moderate >= 0.55, weak >= 0.45, semantic >= 0.42, threshold >= 0.4

4. Document tier meanings:
   - Explicit: from service manifests, configs, explicit links
   - Strong: structural patterns, code analysis, multiple algorithm agreement
   - Moderate: validated by 2+ algorithms
   - Weak: single algorithm, lower signal confidence
   - Semantic: NLP/LLM similarity, speculative
   - Threshold: minimum acceptable confidence

**Verification:**
- Code compiles
- ScoreToTier correctly maps all boundary cases
- TierToScore is reversible for canonical scores

---

### Task 2: Update Discovery Algorithms to Use Tier System

**Action:** Modify discovery algorithm outputs in `discovery_orchestration.go` to assign tier-aware confidence.

**Specific steps:**
1. For each discovery algorithm (co-occurrence, NER, structural, semantic, LLM):
   - Before returning signals, map raw algorithm score → confidence tier
   - Update DiscoverySignal.Confidence to reflect tier canonical score
   - Document which tier each algorithm typically produces

2. Example mappings:
   ```go
   // Co-occurrence typically produces weak-to-moderate
   // (word co-occurrence is signal but not definitive)
   cooccurrenceSignal.Confidence = TierToScore(TierWeak)  // 0.5

   // Explicit manifest links → explicit tier
   manifestLink.Confidence = TierToScore(TierExplicit)    // 1.0

   // LLM analysis → strong or moderate (depends on certainty)
   llmSignal.Confidence = TierToScore(TierStrongInference) // 0.8
   ```

3. Update signal validation to reject signals < TierThreshold

**Verification:**
- All signals use tier-aligned confidence values
- No signals have confidence < 0.4
- Algorithm weights from Plan 1 still apply correctly

---

### Task 3: Add Tier Filtering to Discovery Orchestration

**Action:** Extend DiscoveryFilterConfig in `discovery_orchestration.go` to support tier-based filtering.

**Specific steps:**
1. Add to DiscoveryFilterConfig:
   ```go
   type DiscoveryFilterConfig struct {
       // existing fields...
       MinConfidenceScore float64        // numeric threshold
       MinConfidenceTier  ConfidenceTier // tier threshold
   }
   ```

2. Update filter validation:
   - If both MinConfidenceScore and MinConfidenceTier set, use stricter of the two
   - Default: MinConfidenceTier = TierThreshold (0.4)

3. Apply filter before aggregation (filter weak signals early)

**Verification:**
- Filtering by tier name works (--min-tier strong filters correctly)
- Numeric and tier filters compose correctly
- No signals pass through below threshold

---

### Task 4: Unit Tests for Tier System

**Action:** Create `confidence_test.go` with comprehensive tier mapping tests.

**Specific steps:**
1. Test functions:
   ```go
   func TestScoreToTier_AllBoundaries()           // Each boundary value
   func TestScoreToTier_InterpolatedValues()      // Mid-range values
   func TestTierToScore_Canonical()               // Reverse mapping
   func TestIsSufficientConfidence_Thresholds()   // Min threshold checks
   func TestScoreValidation_RejectsOutOfRange()   // Clamp/warn for <0.4
   ```

2. Example test case:
   ```go
   // Score 0.77 should map to strong-inference (≥0.75)
   tier := ScoreToTier(0.77)
   assert.Equal(t, TierStrongInference, tier)
   ```

3. Validation test:
   ```go
   // Score 0.35 < 0.4 should fail validation
   _, err := NewConfidenceScore(0.35)
   assert.NotNil(t, err)
   ```

**Verification:**
- All tests pass
- Coverage >= 85% for confidence.go
- Boundary mapping is deterministic

---

### Task 5: Integration Test with Phase 1 Data

**Action:** Verify tier system works with component type detection from Phase 1.

**Specific steps:**
1. Create test in `graph_test.go`:
   ```go
   func TestIntegration_ConfidenceTiersWithComponentTypes()
   ```

2. Test flow:
   - Load test-data with component type detection enabled
   - Run discovery pipeline with tier-aware confidence
   - Export to in-memory graph
   - Query: filter by --min-tier strong
   - Verify: returned edges all have confidence >= 0.8

3. Measure:
   - Tier distribution across all edges (count by tier)
   - Impact on relationship count when filtering by different tiers

**Verification:**
- Tier filtering reduces result set predictably
- Confidence distribution matches expected algorithm outputs
- Component types and confidence tiers coexist without issues

---

### Task 6: Documentation & CLI Help Text

**Action:** Add documentation of tier system to codebase.

**Specific steps:**
1. Add to types.go comments:
   ```go
   // ConfidenceTier represents semantic interpretation of numeric confidence
   // Valid range: explicit (1.0) → threshold (0.4)
   // Below 0.4: not exported, not queryable
   ```

2. Update CLI help text (in main.go or cmd/):
   ```
   --min-tier NAME     Filter relationships by confidence tier
                       Valid: explicit, strong-inference, moderate,
                       weak, semantic, threshold
                       Default: threshold (includes all)

   --min-confidence N  Filter relationships by numeric score [0.4, 1.0]
   ```

3. Add to README or docs/CONFIDENCE.md (user-facing):
   ```markdown
   ## Confidence Tiers

   Every relationship includes a confidence score reflecting detection reliability...
   ```

**Verification:**
- Help text appears in `graphmd --help`
- Documentation is accurate and up-to-date
- Examples cover both --min-tier and --min-confidence usage

---

## Verification Criteria

**What "done" looks like:**

1. ✓ **Tier system defined:** 7 tiers with canonical score mapping. ScoreToTier/TierToScore reversible.

2. ✓ **Discovery algorithms use tiers:** All signal outputs have tier-aligned confidence [0.4, 1.0]. No signals rejected for low confidence until after filtering.

3. ✓ **Tier filtering works:** CLI accepts --min-tier and --min-confidence. Filtering produces expected result counts.

4. ✓ **Unit tests pass:** confidence_test.go covers all boundary values and edge cases. Coverage >= 85%.

5. ✓ **Integration test validates:** With test-data, tier filtering reduces results predictably. Tier distribution visible across exported graph.

6. ✓ **No regressions:** Phase 1 tests (component types) still pass. Aggregation (Plan 1) still works correctly.

7. ✓ **Documentation complete:** Code comments explain tier meanings. CLI help text visible. User-facing docs describe tier system.

---

## Must-Haves (Phase Goal Backward)

From phase goal: "confidence and provenance infrastructure that prevents false edges"

This plan delivers:
- ✓ 7-tier confidence interpretation (agents trust scores by understanding what they mean)
- ✓ Tier-aware filtering (low-confidence edges can be excluded from queries)
- ✓ Semantic clarity (names like "strong-inference" more useful than raw 0.8)

---

## Dependency Notes

**Depends on:** Plan 1 (aggregation must exist first; weighted average produces final confidence)

**Enables:** Plan 5 (query interface filters by tier)

---

*Plan created: 2026-03-19*
*Autonomous execution: Yes*
