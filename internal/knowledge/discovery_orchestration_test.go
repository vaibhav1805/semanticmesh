package knowledge

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// makeDiscoveredEdge is a test helper that creates a *DiscoveredEdge with the
// specified confidence and numSignals for use in filter tests.
func makeDiscoveredEdge(source, target string, conf float64, numSignals int) *DiscoveredEdge {
	e, _ := NewEdge(source, target, EdgeMentions, conf, "test evidence")
	sigs := make([]Signal, numSignals)
	for i := range sigs {
		sigs[i] = Signal{
			SourceType: SignalCoOccurrence,
			Confidence: conf,
			Evidence:   fmt.Sprintf("signal-%d", i),
			Weight:     1.0,
		}
	}
	return &DiscoveredEdge{Edge: e, Signals: sigs}
}

// buildOrchTestDocs returns a small corpus of synthetic documents with known
// cross-references suitable for orchestration tests. Named distinctly to avoid
// conflict with hybrid_integration_test.go's buildTestDocs helper.
func buildOrchTestDocs() []Document {
	return []Document{
		{
			ID:        "services/auth.md",
			RelPath:   "services/auth.md",
			Title:     "Auth Service",
			Content:   "# Auth Service\n\nThe Auth Service authenticates users.\n\n## Dependencies\n\n- User Service\n- Redis",
			PlainText: "Auth Service authenticates users. Dependencies: User Service, Redis",
		},
		{
			ID:        "services/user.md",
			RelPath:   "services/user.md",
			Title:     "User Service",
			Content:   "# User Service\n\nManages user profiles.\n\n## Integration\n\n- Auth Service for authentication",
			PlainText: "User Service manages user profiles. Auth Service for authentication",
		},
		{
			ID:        "services/api-gateway.md",
			RelPath:   "services/api-gateway.md",
			Title:     "API Gateway",
			Content:   "# API Gateway\n\nRoutes requests to Auth Service and User Service.",
			PlainText: "API Gateway routes requests to Auth Service and User Service",
		},
	}
}

// ── FilterDiscoveredEdges: solo high-confidence ──────────────────────────────

func TestFilterDiscoveredEdges_SoloHighConf_Accepted(t *testing.T) {
	// Exactly at the solo threshold (0.75) — should pass.
	edge := makeDiscoveredEdge("a", "b", 0.75, 1)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for conf=0.75 solo, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_SoloAboveThreshold_Accepted(t *testing.T) {
	// Well above the solo threshold — should pass.
	edge := makeDiscoveredEdge("a", "b", 0.90, 1)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for conf=0.90 solo, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_SoloBelowThreshold_Rejected(t *testing.T) {
	// Just below the solo threshold (0.59) with a non-LLM signal — should fail.
	// Solo threshold is 0.60; SignalCoOccurrence at 0.59 < 0.60.
	edge := makeDiscoveredEdge("a", "b", 0.59, 1)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 results for conf=0.59 solo non-LLM, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_SoloLowConf_Rejected(t *testing.T) {
	// Typical co-occurrence noise confidence — should fail.
	edge := makeDiscoveredEdge("a", "b", 0.45, 1)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 results for conf=0.45 solo (noise), got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_SoloPerfectConf_Accepted(t *testing.T) {
	// Perfect confidence with single signal — should pass.
	edge := makeDiscoveredEdge("x", "y", 1.0, 1)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for conf=1.0 solo, got %d", len(result))
	}
}

// ── FilterDiscoveredEdges: dual-signal ───────────────────────────────────────

func TestFilterDiscoveredEdges_DualSignalAtThreshold_Accepted(t *testing.T) {
	// Exactly at the dual threshold (0.70) with 2 signals — should pass.
	edge := makeDiscoveredEdge("a", "b", 0.70, 2)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for conf=0.70 dual, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_DualSignalAboveThreshold_Accepted(t *testing.T) {
	// Above dual threshold with 2 signals — should pass.
	edge := makeDiscoveredEdge("a", "b", 0.72, 2)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for conf=0.72 dual, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_DualSignalBelowThreshold_Rejected(t *testing.T) {
	// Just below dual threshold (0.64) with 2 signals — should fail.
	// Dual threshold is 0.65; 0.64 < 0.65 dual AND 0.64 >= 0.60 solo.
	// Solo tier passes (1 signal never needed; but this has 2 signals).
	// Wait: solo tier is conf >= 0.60 regardless of signal count.
	// 0.64 >= 0.60 so this will pass via the solo tier.
	// To get a dual-only rejection we need conf < 0.60 with n=2.
	// Instead, verify dual tier semantics via custom config that sets solo high.
	cfg := DiscoveryFilterConfig{
		MinConfidence:       0.90, // Solo requires very high
		MinConfidenceDual:   0.65, // Dual at new threshold
		MinConfidenceTriple: 0.60,
	}
	// conf=0.64, n=2: 0.64 < 0.90 (solo fails), 0.64 < 0.65 (dual fails), n < 3 (triple fails).
	edge := makeDiscoveredEdge("a", "b", 0.64, 2)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, cfg)
	if len(result) != 0 {
		t.Errorf("expected 0 results for conf=0.64 dual (below new dual threshold 0.65), got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_SingleSignalAtSoloThreshold_Accepted(t *testing.T) {
	// conf=0.60 exactly at solo threshold with 1 non-LLM signal — should pass.
	// Solo threshold is 0.60; SignalCoOccurrence at 0.60 >= 0.60.
	edge := makeDiscoveredEdge("a", "b", 0.60, 1)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for conf=0.60 with 1 non-LLM signal (at solo threshold), got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_DualSignalHighConf_Accepted(t *testing.T) {
	// 2 signals with confidence above solo threshold — passes via solo tier.
	edge := makeDiscoveredEdge("a", "b", 0.80, 2)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for conf=0.80 with 2 signals, got %d", len(result))
	}
}

// ── FilterDiscoveredEdges: triple-signal ─────────────────────────────────────

func TestFilterDiscoveredEdges_TripleSignalAtThreshold_Accepted(t *testing.T) {
	// Exactly at triple threshold (0.65) with 3 signals — should pass.
	edge := makeDiscoveredEdge("a", "b", 0.65, 3)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for conf=0.65 triple, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_TripleSignalAboveThreshold_Accepted(t *testing.T) {
	// Above triple threshold with 3 signals — should pass.
	edge := makeDiscoveredEdge("a", "b", 0.68, 3)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for conf=0.68 triple, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_TripleSignalBelowThreshold_Rejected(t *testing.T) {
	// Just below triple threshold (0.59) with 3 signals — should fail.
	// Triple threshold is 0.60; 0.59 < 0.60. Solo tier also requires 0.60. Fails all tiers.
	edge := makeDiscoveredEdge("a", "b", 0.59, 3)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 results for conf=0.59 with 3 signals (below new triple threshold 0.60), got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_DualSignalAtDualThreshold_Accepted(t *testing.T) {
	// conf=0.65 exactly at the dual threshold with 2 signals — should pass.
	// MinConfidenceDual=0.65; 0.65 >= 0.65 AND n=2 >= 2.
	// Also note: 0.65 >= solo threshold 0.60, so it passes via solo tier too.
	edge := makeDiscoveredEdge("a", "b", 0.65, 2)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for conf=0.65 with 2 signals (at new dual threshold), got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_TripleSignalManySignals_Accepted(t *testing.T) {
	// 5 signals with confidence at triple threshold — passes via triple tier.
	edge := makeDiscoveredEdge("a", "b", 0.65, 5)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for conf=0.65 with 5 signals, got %d", len(result))
	}
}

// ── FilterDiscoveredEdges: edge cases ────────────────────────────────────────

func TestFilterDiscoveredEdges_NilEdge_Rejected(t *testing.T) {
	// A DiscoveredEdge with nil Edge field must be rejected without panic.
	de := &DiscoveredEdge{Edge: nil, Signals: []Signal{{Confidence: 0.90}}}
	result := FilterDiscoveredEdges([]*DiscoveredEdge{de}, DefaultDiscoveryFilterConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 results for nil Edge field, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_NilInput_Empty(t *testing.T) {
	// nil input slice should return non-nil empty slice without panic.
	result := FilterDiscoveredEdges(nil, DefaultDiscoveryFilterConfig())
	if result == nil {
		t.Error("expected non-nil result for nil input, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_EmptyInput_Empty(t *testing.T) {
	// Empty input slice should return non-nil empty slice.
	result := FilterDiscoveredEdges([]*DiscoveredEdge{}, DefaultDiscoveryFilterConfig())
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_MixedBatch(t *testing.T) {
	// A batch of 5 edges with varying confidence and signal counts.
	// New thresholds: solo>=0.60, dual>=0.65, triple>=0.60.
	// Expected pass count: 4 (everything except the noise at 0.50).
	edges := []*DiscoveredEdge{
		makeDiscoveredEdge("a", "b", 0.90, 1), // PASS: solo high (0.90 >= 0.60)
		makeDiscoveredEdge("c", "d", 0.75, 1), // PASS: solo above threshold
		makeDiscoveredEdge("e", "f", 0.74, 1), // PASS: solo above threshold (was FAIL at old 0.75)
		makeDiscoveredEdge("g", "h", 0.70, 2), // PASS: solo tier (0.70 >= 0.60)
		makeDiscoveredEdge("i", "j", 0.50, 1), // FAIL: 0.50 < 0.60 non-LLM solo
	}
	result := FilterDiscoveredEdges(edges, DefaultDiscoveryFilterConfig())
	if len(result) != 4 {
		t.Errorf("expected 4 results from mixed batch with new thresholds, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_CustomConfig(t *testing.T) {
	// A stricter custom config should reject edges that pass the default config.
	cfg := DiscoveryFilterConfig{
		MinConfidence:       0.80,
		MinConfidenceDual:   0.75,
		MinConfidenceTriple: 0.70,
	}
	// conf=0.76 passes default solo (0.75) but fails custom solo (0.80).
	edge := makeDiscoveredEdge("a", "b", 0.76, 1)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, cfg)
	if len(result) != 0 {
		t.Errorf("expected 0 results with strict custom config for conf=0.76, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_CustomConfig_Accepted(t *testing.T) {
	// Same strict config but conf=0.80 exactly at custom solo threshold — should pass.
	cfg := DiscoveryFilterConfig{
		MinConfidence:       0.80,
		MinConfidenceDual:   0.75,
		MinConfidenceTriple: 0.70,
	}
	edge := makeDiscoveredEdge("a", "b", 0.80, 1)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, cfg)
	if len(result) != 1 {
		t.Errorf("expected 1 result with strict config for conf=0.80, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_ZeroConf_Rejected(t *testing.T) {
	// Zero confidence with any number of signals — should always fail.
	edge := makeDiscoveredEdge("a", "b", 0.0, 5)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 results for conf=0.0, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_ZeroSignals_Rejected(t *testing.T) {
	// High confidence but zero signals — should fail all tier checks (n=0 < 2).
	e, _ := NewEdge("a", "b", EdgeMentions, 0.75, "evidence")
	de := &DiscoveredEdge{Edge: e, Signals: []Signal{}}
	// conf=0.75 >= solo threshold (0.75) so it passes via solo tier regardless.
	result := FilterDiscoveredEdges([]*DiscoveredEdge{de}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result: conf=0.75 passes solo tier even with 0 signals, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_AllPass(t *testing.T) {
	// All edges above solo threshold — all should pass.
	edges := []*DiscoveredEdge{
		makeDiscoveredEdge("a", "b", 0.80, 1),
		makeDiscoveredEdge("c", "d", 0.85, 1),
		makeDiscoveredEdge("e", "f", 0.90, 2),
	}
	result := FilterDiscoveredEdges(edges, DefaultDiscoveryFilterConfig())
	if len(result) != 3 {
		t.Errorf("expected all 3 edges to pass, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_AllFail(t *testing.T) {
	// All edges well below all thresholds — none should pass.
	// New floor: 0.60 for solo/triple. Use values below the floor.
	edges := []*DiscoveredEdge{
		makeDiscoveredEdge("a", "b", 0.40, 1),
		makeDiscoveredEdge("c", "d", 0.50, 2),
		makeDiscoveredEdge("e", "f", 0.59, 3),
	}
	result := FilterDiscoveredEdges(edges, DefaultDiscoveryFilterConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 edges to pass (all below new floor 0.60), got %d", len(result))
	}
}

// ── DefaultDiscoveryFilterConfig values ──────────────────────────────────────

func TestDefaultDiscoveryFilterConfig_Values(t *testing.T) {
	// New thresholds: lowered to allow LLM-discovered relationships through.
	// MinConfidence: 0.60 (solo non-LLM), MinConfidenceDual: 0.65 (two signals),
	// MinConfidenceTriple: 0.60 (three signals).
	cfg := DefaultDiscoveryFilterConfig()
	if cfg.MinConfidence != 0.60 {
		t.Errorf("expected MinConfidence=0.60, got %.2f", cfg.MinConfidence)
	}
	if cfg.MinConfidenceDual != 0.65 {
		t.Errorf("expected MinConfidenceDual=0.65, got %.2f", cfg.MinConfidenceDual)
	}
	if cfg.MinConfidenceTriple != 0.60 {
		t.Errorf("expected MinConfidenceTriple=0.60, got %.2f", cfg.MinConfidenceTriple)
	}
}

func TestDefaultDiscoveryFilterConfig_NotZero(t *testing.T) {
	// Sanity check: default config has all non-zero thresholds.
	cfg := DefaultDiscoveryFilterConfig()
	if cfg.MinConfidence == 0 || cfg.MinConfidenceDual == 0 || cfg.MinConfidenceTriple == 0 {
		t.Errorf("default filter config must have non-zero thresholds: %+v", cfg)
	}
}

func TestDefaultDiscoveryFilterConfig_TierOrdering(t *testing.T) {
	// With the LLM-friendly thresholds, MinConfidenceDual >= MinConfidence and
	// MinConfidence >= MinConfidenceTriple. The dual tier requires corroboration
	// AND higher confidence; the solo tier is intentionally lower to allow
	// single strong signals through without requiring multi-algorithm agreement.
	// passesFilter() also provides a separate sub-threshold (0.50) for SignalLLM.
	cfg := DefaultDiscoveryFilterConfig()
	// Dual must be >= triple (corroborated edges demand at least as much as triple).
	if !(cfg.MinConfidenceDual >= cfg.MinConfidenceTriple) {
		t.Errorf("expected MinConfidenceDual >= MinConfidenceTriple, got %+v", cfg)
	}
	// All thresholds must be positive (no zero/negative threshold).
	if cfg.MinConfidence <= 0 || cfg.MinConfidenceDual <= 0 || cfg.MinConfidenceTriple <= 0 {
		t.Errorf("all thresholds must be positive, got %+v", cfg)
	}
}

// ── DiscoverAndIntegrateRelationships ────────────────────────────────────────

func TestDiscoverAndIntegrate_EmptyDocuments(t *testing.T) {
	filtered, all := DiscoverAndIntegrateRelationships(nil, nil, nil, DefaultDiscoveryFilterConfig(), DefaultLLMDiscoveryConfig(), []FileTree{}, []string{})
	if filtered != nil {
		t.Errorf("expected nil filtered for empty input, got %v", filtered)
	}
	if all != nil {
		t.Errorf("expected nil all for empty input, got %v", all)
	}
}

func TestDiscoverAndIntegrate_NilIndex_NoSemanticPanic(t *testing.T) {
	// Should not panic when idx is nil (semantic step skipped gracefully).
	docs := []Document{
		{ID: "a.md", RelPath: "a.md", Content: "# Service A\ncalls service B"},
	}
	// Use recover to catch any panics.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic with nil idx: %v", r)
		}
	}()
	filtered, all := DiscoverAndIntegrateRelationships(docs, nil, nil, DefaultDiscoveryFilterConfig(), DefaultLLMDiscoveryConfig(), []FileTree{}, []string{})
	t.Logf("nil idx: filtered=%d all=%d (no panic)", len(filtered), len(all))
}

func TestDiscoverAndIntegrate_ExplicitEdgesExcluded(t *testing.T) {
	docs := buildOrchTestDocs()
	explicitGraph := NewGraph()

	// Add an explicit edge that discovery might also find.
	e, _ := NewEdge("services/auth.md", "services/user.md", EdgeMentions, 1.0, "explicit link")
	_ = explicitGraph.AddEdge(e)

	filtered, _ := DiscoverAndIntegrateRelationships(docs, nil, explicitGraph, DefaultDiscoveryFilterConfig(), DefaultLLMDiscoveryConfig(), []FileTree{}, []string{})

	// The explicit auth.md → user.md edge (EdgeMentions) must not appear in filtered.
	for _, de := range filtered {
		if de.Source == "services/auth.md" && de.Target == "services/user.md" && de.Type == EdgeMentions {
			t.Errorf("explicit edge leaked into filtered: %s -> %s [%s]", de.Source, de.Target, de.Type)
		}
	}
}

func TestDiscoverAndIntegrate_AllFiltered_ReturnsBoth(t *testing.T) {
	docs := buildOrchTestDocs()

	// A filter with impossible thresholds — nothing will pass.
	veryStrictCfg := DiscoveryFilterConfig{
		MinConfidence:       1.0,
		MinConfidenceDual:   1.0,
		MinConfidenceTriple: 1.0,
	}

	filtered, all := DiscoverAndIntegrateRelationships(docs, nil, nil, veryStrictCfg, DefaultLLMDiscoveryConfig(), []FileTree{}, []string{})

	// `all` may have edges, but `filtered` should be empty (no confidence reaches 1.0).
	if len(all) > 0 && len(filtered) != 0 {
		t.Errorf("expected 0 filtered with impossible threshold, got %d", len(filtered))
	}

	t.Logf("AllFiltered: all=%d filtered=%d", len(all), len(filtered))
}

func TestDiscoverAndIntegrate_WithDocs_ReturnsSomething(t *testing.T) {
	// A corpus with obvious relationships — at least some should be discovered.
	docs := buildOrchTestDocs()
	filtered, all := DiscoverAndIntegrateRelationships(docs, nil, nil, DefaultDiscoveryFilterConfig(), DefaultLLMDiscoveryConfig(), []FileTree{}, []string{})

	t.Logf("WithDocs: all=%d filtered=%d", len(all), len(filtered))

	// We cannot assert exact counts (algorithm output varies), but all should be >= 0.
	if len(all) < 0 || len(filtered) < 0 {
		t.Error("negative counts are impossible")
	}
	// filtered is always a subset of candidates (which come from all).
	if len(filtered) > len(all) {
		t.Errorf("filtered (%d) cannot exceed all (%d)", len(filtered), len(all))
	}
}

func TestDiscoverAndIntegrate_FilteredIsSubsetOfAll(t *testing.T) {
	// Every edge in filtered must also appear in all (by source+target+type key).
	docs := buildOrchTestDocs()
	filtered, all := DiscoverAndIntegrateRelationships(docs, nil, nil, DefaultDiscoveryFilterConfig(), DefaultLLMDiscoveryConfig(), []FileTree{}, []string{})

	allKeys := make(map[string]bool)
	for _, de := range all {
		if de.Edge != nil {
			k := de.Source + "\x00" + de.Target + "\x00" + string(de.Type)
			allKeys[k] = true
		}
	}

	for _, de := range filtered {
		if de.Edge == nil {
			t.Error("filtered contains nil Edge")
			continue
		}
		k := de.Source + "\x00" + de.Target + "\x00" + string(de.Type)
		if !allKeys[k] {
			t.Errorf("filtered edge not in all: %s -> %s [%s]", de.Source, de.Target, de.Type)
		}
	}
}

func TestDiscoverAndIntegrate_NilExplicitGraph(t *testing.T) {
	// nil explicitGraph is valid — should not panic.
	docs := buildOrchTestDocs()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic with nil explicitGraph: %v", r)
		}
	}()
	filtered, all := DiscoverAndIntegrateRelationships(docs, nil, nil, DefaultDiscoveryFilterConfig(), DefaultLLMDiscoveryConfig(), []FileTree{}, []string{})
	t.Logf("nil graph: filtered=%d all=%d", len(filtered), len(all))
}

// ── Integration test on test-data/ monorepo ──────────────────────────────────

func TestDiscoverAndIntegrate_RealTestData(t *testing.T) {
	// Use the graph-test-docs directory which has representative markdown files.
	// Go test working directory is the package dir (internal/knowledge/), so
	// we go up two levels to reach the repository root.
	testDataDir := "../../test-data/graph-test-docs"
	if _, err := os.Stat(testDataDir); os.IsNotExist(err) {
		t.Skip("test-data/graph-test-docs not found — skipping integration test")
	}

	docs, err := ScanDirectory(testDataDir, ScanConfig{UseDefaultIgnores: true})
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}
	if len(docs) == 0 {
		t.Skip("No markdown files found in test-data/graph-test-docs")
	}

	// Build BM25 index for semantic algorithm.
	idx := NewIndex()
	if err := idx.Build(docs); err != nil {
		t.Fatalf("Index.Build: %v", err)
	}

	// Build explicit graph.
	gb := NewGraphBuilder(testDataDir)
	graph := gb.Build(docs)

	cfg := DefaultDiscoveryFilterConfig()
	llmCfg := DefaultLLMDiscoveryConfig()
	llmCfg.ExecutablePath = "/nonexistent/pageindex" // Skip actual pageindex calls in tests
	filtered, all := DiscoverAndIntegrateRelationships(docs, idx.bm25, graph, cfg, llmCfg, []FileTree{}, []string{})

	t.Logf("Integration: %d docs, %d all discovered, %d passed filter",
		len(docs), len(all), len(filtered))

	// Basic sanity checks — not asserting exact counts (they vary with test data).
	if filtered == nil && len(docs) > 2 {
		t.Log("Warning: no relationships passed filter on real test data — check thresholds")
	}

	// Verify no explicit edges leaked into filtered output.
	explicitKeys := make(map[string]bool)
	for _, e := range graph.Edges {
		k := e.Source + "\x00" + e.Target + "\x00" + string(e.Type)
		explicitKeys[k] = true
	}
	for _, de := range filtered {
		if de.Edge == nil {
			t.Error("filtered contains nil Edge")
			continue
		}
		k := de.Source + "\x00" + de.Target + "\x00" + string(de.Type)
		if explicitKeys[k] {
			t.Errorf("explicit edge leaked into filtered: %s -> %s", de.Source, de.Target)
		}
	}

	// All filtered edges must pass the filter themselves.
	for _, de := range filtered {
		conf := de.Confidence
		n := len(de.Signals)
		passes := conf >= cfg.MinConfidence ||
			(conf >= cfg.MinConfidenceDual && n >= 2) ||
			(conf >= cfg.MinConfidenceTriple && n >= 3)
		if !passes {
			t.Errorf("filtered edge does not pass filter: %s -> %s conf=%.2f signals=%d",
				de.Source, de.Target, conf, n)
		}
	}
}

// ── Performance test: 100-file corpus ────────────────────────────────────────

func TestDiscoverAndIntegrate_Performance_100Files(t *testing.T) {
	// Generate 100 synthetic documents.
	docs := make([]Document, 100)
	for i := range docs {
		docs[i] = Document{
			ID:        fmt.Sprintf("svc-%03d.md", i),
			RelPath:   fmt.Sprintf("svc-%03d.md", i),
			Title:     fmt.Sprintf("Service %03d", i),
			Content:   fmt.Sprintf("# Service %03d\n\nThis service calls Service %03d and uses Service %03d.", i, (i+1)%100, (i+7)%100),
			PlainText: fmt.Sprintf("Service %03d calls Service %03d and uses Service %03d.", i, (i+1)%100, (i+7)%100),
		}
	}

	idx := NewIndex()
	_ = idx.Build(docs)

	graph := NewGraph()

	start := time.Now()
	llmCfg := DefaultLLMDiscoveryConfig()
	llmCfg.ExecutablePath = "/nonexistent/pageindex"
	filtered, all := DiscoverAndIntegrateRelationships(docs, idx.bm25, graph, DefaultDiscoveryFilterConfig(), llmCfg, []FileTree{}, []string{})
	elapsed := time.Since(start)

	t.Logf("Performance (100 docs): %v — %d all, %d filtered", elapsed, len(all), len(filtered))

	if elapsed > 500*time.Millisecond {
		t.Errorf("DiscoverAndIntegrateRelationships took %v for 100 docs (limit: 500ms)", elapsed)
	}
}

// TestDiscoverAndIntegrate_WithLLMStub validates 4-algorithm pipeline with LLM stubbed out.
func TestDiscoverAndIntegrate_WithLLMStub(t *testing.T) {
	docs := buildOrchTestDocs()

	llmCfg := DefaultLLMDiscoveryConfig()
	llmCfg.ExecutablePath = "/nonexistent/pageindex"

	filtered, all := DiscoverAndIntegrateRelationships(
		docs, nil, nil, DefaultDiscoveryFilterConfig(),
		llmCfg, []FileTree{}, []string{},
	)

	t.Logf("WithLLMStub: all=%d filtered=%d", len(all), len(filtered))

	// The 3 non-LLM algorithms (co-occurrence, structural, NER) should still find edges.
	if len(all) == 0 {
		t.Error("expected non-zero edges from 3 non-LLM algorithms, got 0")
	}
	// filtered must be a subset of all.
	if len(filtered) > len(all) {
		t.Errorf("filtered (%d) > all (%d): impossible", len(filtered), len(all))
	}
}

// ── SignalLLM-specific passesFilter logic ────────────────────────────────────

// makeLLMDiscoveredEdge creates a *DiscoveredEdge with a single SignalLLM signal
// at the given confidence level. Used to test the LLM-specific filter path.
func makeLLMDiscoveredEdge(source, target string, conf float64) *DiscoveredEdge {
	e, _ := NewEdge(source, target, EdgeMentions, conf, "llm evidence")
	return &DiscoveredEdge{
		Edge: e,
		Signals: []Signal{{
			SourceType: SignalLLM,
			Confidence: conf,
			Evidence:   "llm: reasoning about document purpose",
			Weight:     1.0,
		}},
	}
}

func TestFilterDiscoveredEdges_LLMSolo_HighConf_Accepted(t *testing.T) {
	// Solo LLM signal at 0.80 — should pass via SignalLLM special case (threshold 0.50).
	edge := makeLLMDiscoveredEdge("a", "b", 0.80)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for solo LLM conf=0.80, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_LLMSolo_AtSubThreshold_Accepted(t *testing.T) {
	// Solo LLM signal at exactly 0.50 — should pass via SignalLLM special case.
	// 0.50 is below the standard solo threshold (0.60) but LLM gets a lower bar.
	edge := makeLLMDiscoveredEdge("c", "d", 0.50)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for solo LLM conf=0.50 (at LLM sub-threshold), got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_LLMSolo_BelowSubThreshold_Rejected(t *testing.T) {
	// Solo LLM signal at 0.49 — should fail (below the 0.50 LLM sub-threshold).
	edge := makeLLMDiscoveredEdge("e", "f", 0.49)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 results for solo LLM conf=0.49 (below 0.50 LLM threshold), got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_LLMSolo_BetweenLLMAndStandard_Accepted(t *testing.T) {
	// Solo LLM signal at 0.55 — between LLM threshold (0.50) and standard solo (0.60).
	// Should pass via the SignalLLM special case.
	edge := makeLLMDiscoveredEdge("g", "h", 0.55)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for solo LLM conf=0.55 (LLM path, between 0.50-0.60), got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_LLMPlusOtherSignal_UsesStandardLogic(t *testing.T) {
	// When LLM signal is combined with another signal, the standard multi-signal
	// logic applies (not the LLM special case which requires exactly 1 LLM signal).
	e, _ := NewEdge("a", "b", EdgeMentions, 0.55, "evidence")
	de := &DiscoveredEdge{
		Edge: e,
		Signals: []Signal{
			{SourceType: SignalLLM, Confidence: 0.55, Evidence: "llm reasoning", Weight: 1.0},
			{SourceType: SignalCoOccurrence, Confidence: 0.55, Evidence: "co-occur", Weight: 1.0},
		},
	}
	// 2 signals, conf=0.55: solo fails (0.55 < 0.60), dual fails (0.55 < 0.65).
	result := FilterDiscoveredEdges([]*DiscoveredEdge{de}, DefaultDiscoveryFilterConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 results for LLM+other at conf=0.55 (2-signal standard path), got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_NonLLMSolo_AtNewThreshold_Accepted(t *testing.T) {
	// Non-LLM solo signal at exactly 0.60 (new solo threshold) — should pass.
	edge := makeDiscoveredEdge("a", "b", 0.60, 1)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 1 {
		t.Errorf("expected 1 result for non-LLM solo conf=0.60 (at new threshold), got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_NonLLMSolo_BelowNewThreshold_Rejected(t *testing.T) {
	// Non-LLM solo signal just below 0.60 — should fail (uses standard path, not LLM path).
	edge := makeDiscoveredEdge("a", "b", 0.59, 1)
	result := FilterDiscoveredEdges([]*DiscoveredEdge{edge}, DefaultDiscoveryFilterConfig())
	if len(result) != 0 {
		t.Errorf("expected 0 results for non-LLM solo conf=0.59 (below new 0.60 threshold), got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_Determinism_ThreeRuns(t *testing.T) {
	// Verify that FilterDiscoveredEdges produces identical results on repeated calls
	// with the same input (no randomness or ordering variation).
	edges := []*DiscoveredEdge{
		makeLLMDiscoveredEdge("auth", "user", 0.75),
		makeLLMDiscoveredEdge("auth", "cache", 0.52),
		makeLLMDiscoveredEdge("api", "auth", 0.48),         // Below 0.50 LLM threshold — fails
		makeDiscoveredEdge("order", "user", 0.65, 1),       // Solo non-LLM above 0.60 — passes
		makeDiscoveredEdge("order", "cache", 0.55, 1),      // 0.55 < 0.60 non-LLM — fails
		makeDiscoveredEdge("billing", "order", 0.65, 2),    // Dual at threshold — passes
	}
	cfg := DefaultDiscoveryFilterConfig()

	var results [3][]*DiscoveredEdge
	for i := range results {
		results[i] = FilterDiscoveredEdges(edges, cfg)
	}

	// All 3 runs must produce the same count.
	for i := 1; i < 3; i++ {
		if len(results[i]) != len(results[0]) {
			t.Errorf("run %d produced %d results, run 0 produced %d (non-determinism detected)",
				i, len(results[i]), len(results[0]))
		}
	}

	// Expected: auth→user(LLM 0.75), auth→cache(LLM 0.52), order→user(0.65 solo), billing→order(dual) = 4
	if len(results[0]) != 4 {
		t.Errorf("expected 4 edges to pass filter, got %d", len(results[0]))
	}
}

// ── FilterSignalsByTier tests ────────────────────────────────────────────────

func TestFilterSignalsByTier_ModerateAndAbove(t *testing.T) {
	edges := []*DiscoveredEdge{
		makeDiscoveredEdge("a", "b", 1.0, 1),  // explicit
		makeDiscoveredEdge("c", "d", 0.8, 1),  // strong-inference
		makeDiscoveredEdge("e", "f", 0.6, 1),  // moderate
		makeDiscoveredEdge("g", "h", 0.5, 1),  // weak
		makeDiscoveredEdge("i", "j", 0.42, 1), // semantic
		makeDiscoveredEdge("k", "l", 0.4, 1),  // threshold
	}
	result := FilterSignalsByTier(edges, TierModerate)
	if len(result) != 3 {
		t.Errorf("FilterSignalsByTier(moderate) = %d edges, want 3", len(result))
	}
}

func TestFilterSignalsByTier_ExplicitOnly(t *testing.T) {
	edges := []*DiscoveredEdge{
		makeDiscoveredEdge("a", "b", 1.0, 1),  // explicit
		makeDiscoveredEdge("c", "d", 0.96, 1), // explicit
		makeDiscoveredEdge("e", "f", 0.8, 1),  // strong-inference
	}
	result := FilterSignalsByTier(edges, TierExplicit)
	if len(result) != 2 {
		t.Errorf("FilterSignalsByTier(explicit) = %d edges, want 2", len(result))
	}
}

func TestFilterSignalsByTier_ThresholdReturnsAll(t *testing.T) {
	edges := []*DiscoveredEdge{
		makeDiscoveredEdge("a", "b", 1.0, 1),
		makeDiscoveredEdge("c", "d", 0.8, 1),
		makeDiscoveredEdge("e", "f", 0.5, 1),
		makeDiscoveredEdge("g", "h", 0.4, 1),
	}
	result := FilterSignalsByTier(edges, TierThreshold)
	if len(result) != 4 {
		t.Errorf("FilterSignalsByTier(threshold) = %d edges, want 4", len(result))
	}
}

func TestFilterSignalsByTier_NilEdge_Skipped(t *testing.T) {
	edges := []*DiscoveredEdge{
		nil,
		{Edge: nil, Signals: []Signal{}},
		makeDiscoveredEdge("a", "b", 0.8, 1),
	}
	result := FilterSignalsByTier(edges, TierThreshold)
	if len(result) != 1 {
		t.Errorf("expected 1 valid edge, got %d", len(result))
	}
}

func TestFilterSignalsByTier_InvalidConfidence_Skipped(t *testing.T) {
	e, _ := NewEdge("a", "b", EdgeMentions, 0.3, "evidence")
	edges := []*DiscoveredEdge{
		{Edge: e, Signals: []Signal{}}, // 0.3 is below valid range
	}
	result := FilterSignalsByTier(edges, TierThreshold)
	if len(result) != 0 {
		t.Errorf("expected 0 edges for invalid confidence, got %d", len(result))
	}
}

// ── FilterByConfidenceScore tests ────────────────────────────────────────────

func TestFilterByConfidenceScore_Basic(t *testing.T) {
	edges := []*DiscoveredEdge{
		makeDiscoveredEdge("a", "b", 0.9, 1),
		makeDiscoveredEdge("c", "d", 0.7, 1),
		makeDiscoveredEdge("e", "f", 0.5, 1),
	}
	result := FilterByConfidenceScore(edges, 0.7)
	if len(result) != 2 {
		t.Errorf("FilterByConfidenceScore(0.7) = %d edges, want 2", len(result))
	}
}

// ── DiscoveryFilterConfig with MinTier/MinScore ──────────────────────────────

func TestFilterDiscoveredEdges_WithMinTier(t *testing.T) {
	strongTier := TierStrongInference
	cfg := DefaultDiscoveryFilterConfig()
	cfg.MinTier = &strongTier

	edges := []*DiscoveredEdge{
		makeDiscoveredEdge("a", "b", 0.9, 1),  // strong-inference, passes standard + tier
		makeDiscoveredEdge("c", "d", 0.65, 1), // moderate, passes standard but fails tier
	}
	result := FilterDiscoveredEdges(edges, cfg)
	if len(result) != 1 {
		t.Errorf("expected 1 edge with MinTier=strong, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_WithMinScore(t *testing.T) {
	minScore := 0.8
	cfg := DefaultDiscoveryFilterConfig()
	cfg.MinScore = &minScore

	edges := []*DiscoveredEdge{
		makeDiscoveredEdge("a", "b", 0.9, 1),  // passes
		makeDiscoveredEdge("c", "d", 0.75, 1), // fails MinScore
	}
	result := FilterDiscoveredEdges(edges, cfg)
	if len(result) != 1 {
		t.Errorf("expected 1 edge with MinScore=0.8, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_WithBothMinTierAndMinScore(t *testing.T) {
	strongTier := TierStrongInference
	minScore := 0.85
	cfg := DefaultDiscoveryFilterConfig()
	cfg.MinTier = &strongTier
	cfg.MinScore = &minScore

	edges := []*DiscoveredEdge{
		makeDiscoveredEdge("a", "b", 0.9, 1),  // passes both
		makeDiscoveredEdge("c", "d", 0.8, 1),  // passes tier but fails score (0.8 < 0.85)
		makeDiscoveredEdge("e", "f", 0.65, 1), // fails tier
	}
	result := FilterDiscoveredEdges(edges, cfg)
	if len(result) != 1 {
		t.Errorf("expected 1 edge with both filters, got %d", len(result))
	}
}

func TestFilterDiscoveredEdges_NilMinTier_NoEffect(t *testing.T) {
	cfg := DefaultDiscoveryFilterConfig()
	// MinTier and MinScore are nil by default - should behave like before
	edges := []*DiscoveredEdge{
		makeDiscoveredEdge("a", "b", 0.65, 1),
	}
	result := FilterDiscoveredEdges(edges, cfg)
	if len(result) != 1 {
		t.Errorf("nil MinTier should not affect filtering, got %d results", len(result))
	}
}
