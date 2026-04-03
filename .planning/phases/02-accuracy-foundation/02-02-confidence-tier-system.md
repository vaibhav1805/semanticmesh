---
phase: 02-accuracy-foundation
plan: 02
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/knowledge/types.go
  - internal/knowledge/confidence.go
  - internal/knowledge/discovery_orchestration.go
autonomous: true
requirements:
  - REL-01

must_haves:
  truths:
    - "Every relationship has confidence score in range [0.4, 1.0]"
    - "Every confidence score maps to exactly one of 7 named tiers"
    - "7 tiers correspond to semantic confidence levels: explicit, strong-inference, moderate, weak, semantic, threshold"
    - "Tier-to-score and score-to-tier mappings are bidirectional"
    - "Confidence filtering supports both numeric (--min-confidence 0.7) and named tier (--min-tier strong) query interfaces"
  artifacts:
    - path: "internal/knowledge/confidence.go"
      provides: "ConfidenceTier type, tier constants, ScoreToTier(), TierToScore() functions"
      exports: ["ConfidenceTier", "TierExplicit", "TierStrongInference", "TierModerate", "TierWeak", "TierSemantic", "TierThreshold", "ScoreToTier", "TierToScore"]
      min_lines: 80
    - path: "internal/knowledge/types.go"
      provides: "Updated ConfidenceScore type annotation; tier constants injected"
      exports: ["ConfidenceScore"]
    - path: "internal/knowledge/discovery_orchestration.go"
      provides: "Tier-aware filtering in DiscoveryFilterConfig"
      exports: ["FilterByTier", "FilterByConfidenceScore"]
  key_links:
    - from: "Discovery algorithms (all types)"
      to: "ConfidenceScore with tier mapping"
      via: "Every DiscoverySignal.Confidence must be in [0.4, 1.0] and map to a tier"
      pattern: "ConfidenceScore.*[0-9.]+|ScoreToTier"
    - from: "DiscoveryFilterConfig"
      to: "Tier-based or numeric-based filtering"
      via: "FilterByTier(minTier) or FilterByConfidenceScore(minScore)"
      pattern: "Filter.*Tier|Filter.*Confidence"
    - from: "CLI query commands (future: Phase 4)"
      to: "Tier constants for flag parsing"
      via: "--min-tier strong → TierStrongInference"
      pattern: "TierStrong|--min-tier"

---

<objective>
Establish a 7-tier confidence system that categorizes relationship inference quality from "explicit" (1.0) to "threshold" (0.4), providing AI agents with both numeric scores and semantic tier names for filtering and trustworthiness assessment.

Purpose: REL-01 requirement; enables agents to distinguish high-confidence explicit links from weaker semantic inferences. Both numeric and semantic tier filtering must be supported for query flexibility.

Output: Confidence tier constants and mapping functions, tier-aware discovery filtering, bidirectional score ↔ tier conversions.
</objective>

<execution_context>
@/Users/flurryhead/.claude/get-shit-done/workflows/execute-plan.md
@/Users/flurryhead/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/ROADMAP.md
@.planning/phases/02-accuracy-foundation/02-CONTEXT.md
@.planning/phases/02-accuracy-foundation/02-RESEARCH.md
@internal/knowledge/types.go
@internal/knowledge/discovery_orchestration.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Define confidence tier system in new confidence.go file</name>
  <files>internal/knowledge/confidence.go</files>
  <action>
Create new file internal/knowledge/confidence.go with:

1. Type definitions:
   - type ConfidenceTier string (semantic tier name)
   - type ConfidenceScore float64 (numeric score [0.4, 1.0])

2. Tier constants (7 levels, from CONTEXT.md decision):
   - const TierExplicit ConfidenceTier = "explicit"           // 1.0
   - const TierStrongInference ConfidenceTier = "strong-inference" // 0.8
   - const TierModerate ConfidenceTier = "moderate"           // 0.6
   - const TierWeak ConfidenceTier = "weak"                   // 0.5
   - const TierSemantic ConfidenceTier = "semantic"           // 0.45
   - const TierThreshold ConfidenceTier = "threshold"         // 0.4

3. Implement ScoreToTier(score ConfidenceScore) ConfidenceTier:
   - score >= 0.95 → TierExplicit
   - score >= 0.75 → TierStrongInference
   - score >= 0.55 → TierModerate
   - score >= 0.45 → TierWeak (note: threshold at 0.45, not 0.5)
   - score >= 0.42 → TierSemantic
   - score >= 0.4 → TierThreshold
   - score < 0.4 → error or panic("confidence below minimum threshold")

4. Implement TierToScore(tier ConfidenceTier) ConfidenceScore:
   - TierExplicit → 1.0
   - TierStrongInference → 0.8
   - TierModerate → 0.6
   - TierWeak → 0.5
   - TierSemantic → 0.45
   - TierThreshold → 0.4
   - Unknown tier → error

5. Utility functions:
   - IsValidConfidenceScore(score float64) bool — returns true if score in [0.4, 1.0], false otherwise
   - AllConfidenceTiers() []ConfidenceTier — returns slice of all 7 tiers (for iteration in filtering)
   - TierDisplayName(tier ConfidenceTier) string — returns human-readable tier name with score (e.g., "Explicit (1.0)")

6. Add comments explaining semantic meaning of each tier (e.g., "Explicit: from service manifests/configs")

File structure: ~80-100 lines, idiomatic Go with clear error handling.
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestConfidenceTier -v
Expected tests (create these in confidence_test.go):
  - TestScoreToTier_BoundaryValues: 1.0→explicit, 0.8→strong, 0.4→threshold
  - TestScoreToTier_Edges: 0.95→explicit (just under), 0.4→threshold, 0.39→error
  - TestTierToScore_Reversible: tier→score→tier returns original tier
  - TestIsValidConfidenceScore: True for [0.4, 1.0], false outside
  - TestAllConfidenceTiers: Returns exactly 7 tiers

Also run: go build ./cmd/graphmd — no syntax errors
  </verify>
  <done>
- confidence.go created with 7 tier constants and canonical scores
- ScoreToTier() correctly maps numeric ranges to tiers with proper boundary handling
- TierToScore() provides inverse mapping
- Bidirectional conversion works correctly (tier → score → tier = original)
- Boundary cases tested: 0.4 (threshold), 1.0 (explicit), values just outside range rejected
- All 7 tiers accessible via AllConfidenceTiers()
  </done>
</task>

<task type="auto">
  <name>Task 2: Integrate confidence tiers into discovery algorithms and aggregation</name>
  <files>internal/knowledge/types.go, internal/knowledge/discovery_orchestration.go</files>
  <action>
Update existing discovery code to use confidence tier system:

1. In types.go:
   - Add type alias: type ConfidenceScore float64 at top (near existing type definitions)
   - Import confidence package: import "github.com/your-module/internal/knowledge/confidence"
   - Update comments on ConfidenceScore uses to reference [0.4, 1.0] range and tier mapping

2. In discovery_orchestration.go (where discovery algorithms are called):
   - After each discovery signal is created (co-occurrence, NER, semantic, LLM), validate confidence is in [0.4, 1.0]:
     ```go
     if !confidence.IsValidConfidenceScore(signal.Confidence) {
         // log warning or skip signal
         continue
     }
     ```
   - Add helper function FilterSignalsByTier(signals []DiscoverySignal, minTier ConfidenceTier) []DiscoverySignal:
     - Filter signals where ScoreToTier(signal.Confidence) >= minTier (with proper tier ordering)
     - Return filtered slice
   - Update DiscoveryFilterConfig to support both numeric and tier-based filtering:
     - Add field: MinTier *ConfidenceTier (optional)
     - Add field: MinScore *ConfidenceScore (optional, existing)
     - Update filter application logic to check both (if MinTier set, use tier-based; else use score-based)

3. In SaveGraph (or wherever edges are persisted):
   - Ensure all persisted Confidence values pass IsValidConfidenceScore() check
   - Add validation before INSERT: "Confidence must be in [0.4, 1.0]"

Reference existing DiscoveryFilterConfig structure; update without breaking existing field order.
  </action>
  <verify>
Run: go test ./internal/knowledge -run TestDiscoveryFiltering -v
Expected:
  - TestFilterSignalsByTier: Filtering by TierModerate returns only signals with tier >= moderate
  - TestConfidenceValidation: Signals with confidence < 0.4 are rejected
  - TestDualFilterModes: DiscoveryFilterConfig respects both MinTier and MinScore

Also run: go build ./cmd/graphmd — no compilation errors
  </verify>
  <done>
- ConfidenceScore type alias defined and documented
- All discovery algorithms validate confidence in [0.4, 1.0]
- FilterSignalsByTier() function implemented and tested
- DiscoveryFilterConfig supports both tier-based and score-based filtering
- Discovery pipeline rejects invalid confidence scores before persistence
  </done>
</task>

<task type="auto">
  <name>Task 3: Unit tests for confidence tier system and integration with discovery</name>
  <files>internal/knowledge/confidence_test.go, internal/knowledge/discovery_orchestration_test.go</files>
  <action>
Create comprehensive test coverage:

1. In confidence_test.go:
   - TestScoreToTier_AllBoundaries: Verify all 7 tier transitions
   - TestScoreToTier_InvalidScores: Confirm < 0.4 and > 1.0 rejected
   - TestTierToScore_AllTiers: All 7 tiers map to expected scores
   - TestBidirectionalConversion: For each score in [0.4, 1.0], tier→score→tier = original
   - TestIsValidConfidenceScore: True for [0.4, 1.0], false outside
   - TestAllConfidenceTiers: Returns exactly 7 unique tiers
   - TestTierOrdering: Can compare tiers (e.g., TierModerate > TierWeak)

2. In discovery_orchestration_test.go:
   - TestFilterSignalsByTier_ModerateAndAbove: Only signals with tier >= moderate
   - TestFilterSignalsByTier_ExplicitOnly: Tier = explicit returns only 1.0 confidence signals
   - TestFilterSignalsByTier_AllTiers: Filtering by threshold returns all signals
   - TestDualFilterModes: DiscoveryFilterConfig with MinTier vs MinScore produces same/compatible results
   - TestConfidenceValidation_Rejection: Signals with confidence < 0.4 rejected before aggregation

Run tests with: go test ./internal/knowledge -run "TestConfidence|TestFiltering" -v -cover
Expected: All tests pass, >85% coverage on confidence.go and tier-related code in discovery_orchestration.go

No breaking changes to existing discovery algorithms; tier system is purely additive and validation-focused.
  </action>
  <verify>
Run: go test ./internal/knowledge -run "TestConfidence|TestFiltering" -v
Expected: All tests pass
Run coverage: go test ./internal/knowledge -cover | grep "confidence.go"
Expected: Coverage >= 85%
  </verify>
  <done>
- All 7 tiers have unit test coverage for boundary transitions
- Bidirectional conversion tested (score ↔ tier round-trip)
- Invalid scores rejected consistently
- Discovery signal filtering by tier works correctly
- Integration with DiscoveryFilterConfig working
- >85% coverage on confidence-related code
  </done>
</task>

</tasks>

<verification>
**Comprehensive checks before marking complete:**

1. **Tier Completeness:** All 7 tiers defined and have canonical scores. No tier is missing or duplicated.

2. **Score Range Enforcement:** Any confidence value < 0.4 is rejected. Any value > 1.0 is rejected. Boundaries 0.4 and 1.0 are valid.

3. **Bidirectional Conversion:** For every integer step in [0.4, 1.0] (e.g., 0.4, 0.5, 0.6, ..., 1.0), ScoreToTier(score) then TierToScore(tier) returns original tier (or adjacent tier if score falls between canonical values).

4. **No Breaking Changes:** Existing DiscoverySignal, Edge, Graph structures still work. Tier system is purely additive filtering/validation layer.

5. **Test Coverage:** confidence.go has unit tests for all 7 tiers and boundary cases. discovery_orchestration.go has tests for filtering by tier.

6. **CLI-Ready:** TierConstants exported so Phase 4 can reference them in flag parsing (e.g., --min-tier strong).
</verification>

<success_criteria>
- REL-01 satisfied: 7-tier confidence system defined and integrated
- All 7 tiers have canonical scores and semantic names
- ScoreToTier() and TierToScore() provide bidirectional conversion
- All confidence scores in persisted relationships are in [0.4, 1.0]
- Discovery pipeline validates confidence scores during signal creation
- DiscoveryFilterConfig supports both numeric (--min-confidence) and tier-based (--min-tier) filtering
- Unit tests pass with >85% coverage on confidence tier code
- No breaking changes to Phase 1 artifacts or existing discovery logic
</success_criteria>

<output>
After completion, create `.planning/phases/02-accuracy-foundation/02-02-SUMMARY.md` documenting:
- 7-tier confidence system with canonical scores
- ScoreToTier() and TierToScore() function behavior
- Integration with discovery algorithms and filtering
- Test coverage and verification results
- Confidence validation in discovery pipeline
</output>
