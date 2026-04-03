package knowledge

import (
	"fmt"
	"math"
	"testing"
)

func TestDiscoveryAggregateSignals_Empty(t *testing.T) {
	result := AggregateSignals(nil)
	if result != nil {
		t.Errorf("nil input: got %d results, want nil", len(result))
	}

	result = AggregateSignals([]DiscoverySignal{})
	if result != nil {
		t.Errorf("empty input: got %d results, want nil", len(result))
	}
}

func TestDiscoveryAggregateSignals_SingleSignal(t *testing.T) {
	signals := []DiscoverySignal{
		{
			Source:     "a.md",
			Target:     "b.md",
			Type:       EdgeRelated,
			Confidence: 0.6,
			Evidence:   "Semantic overlap: 0.55",
			Algorithm:  "semantic",
		},
	}
	result := AggregateSignals(signals)
	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}
	if result[0].Source != "a.md" || result[0].Target != "b.md" {
		t.Errorf("pair: got (%s,%s), want (a.md,b.md)", result[0].Source, result[0].Target)
	}
	if result[0].Confidence != 0.6 {
		t.Errorf("confidence: got %.2f, want 0.6", result[0].Confidence)
	}
	if len(result[0].Signals) != 1 {
		t.Errorf("signals: got %d, want 1", len(result[0].Signals))
	}
}

func TestDiscoveryAggregateSignals_MultipleSignalsSamePair(t *testing.T) {
	signals := []DiscoverySignal{
		{
			Source:     "a.md",
			Target:     "b.md",
			Type:       EdgeRelated,
			Confidence: 0.5,
			Evidence:   "Semantic overlap: 0.40",
			Algorithm:  "semantic",
		},
		{
			Source:     "a.md",
			Target:     "b.md",
			Type:       EdgeMentions,
			Confidence: 0.7,
			Evidence:   "Co-occurrence: 5 shared terms",
			Algorithm:  "cooccurrence",
		},
	}
	result := AggregateSignals(signals)
	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}
	// Should pick the higher confidence (0.7 from cooccurrence).
	if result[0].Confidence != 0.7 {
		t.Errorf("confidence: got %.2f, want 0.7", result[0].Confidence)
	}
	if result[0].Type != EdgeMentions {
		t.Errorf("type: got %s, want %s", result[0].Type, EdgeMentions)
	}
	if len(result[0].Signals) != 2 {
		t.Errorf("signals: got %d, want 2", len(result[0].Signals))
	}
}

func TestDiscoveryAggregateSignals_MultiplePairs(t *testing.T) {
	signals := []DiscoverySignal{
		{Source: "a.md", Target: "b.md", Type: EdgeRelated, Confidence: 0.6, Algorithm: "semantic"},
		{Source: "b.md", Target: "c.md", Type: EdgeRelated, Confidence: 0.8, Algorithm: "semantic"},
		{Source: "a.md", Target: "c.md", Type: EdgeRelated, Confidence: 0.4, Algorithm: "semantic"},
	}
	result := AggregateSignals(signals)
	if len(result) != 3 {
		t.Fatalf("got %d results, want 3", len(result))
	}
	// Should be sorted by confidence descending.
	if result[0].Confidence < result[1].Confidence || result[1].Confidence < result[2].Confidence {
		t.Errorf("not sorted by confidence: %.2f, %.2f, %.2f",
			result[0].Confidence, result[1].Confidence, result[2].Confidence)
	}
}

func TestDiscoveryAggregateSignals_DirectionalPairsAreDistinct(t *testing.T) {
	// (a->b) and (b->a) are different directed pairs.
	signals := []DiscoverySignal{
		{Source: "a.md", Target: "b.md", Type: EdgeDependsOn, Confidence: 0.7, Algorithm: "structural"},
		{Source: "b.md", Target: "a.md", Type: EdgeDependsOn, Confidence: 0.6, Algorithm: "structural"},
	}
	result := AggregateSignals(signals)
	if len(result) != 2 {
		t.Errorf("directional pairs should be distinct: got %d, want 2", len(result))
	}
}

func TestEdgeSignalsFromSemantic(t *testing.T) {
	edges := []*Edge{
		{Source: "a.md", Target: "b.md", Type: EdgeRelated, Confidence: 0.6, Evidence: "Semantic overlap: 0.50"},
	}
	signals := EdgeSignalsFromSemantic(edges)
	if len(signals) != 1 {
		t.Fatalf("got %d signals, want 1", len(signals))
	}
	if signals[0].Algorithm != "semantic" {
		t.Errorf("algorithm: got %q, want %q", signals[0].Algorithm, "semantic")
	}
	if signals[0].Source != "a.md" {
		t.Errorf("source: got %q, want %q", signals[0].Source, "a.md")
	}
}

func TestEdgeSignalsFromAlgorithm(t *testing.T) {
	edges := []*Edge{
		{Source: "x.md", Target: "y.md", Type: EdgeMentions, Confidence: 0.65, Evidence: "shared terms"},
	}
	signals := EdgeSignalsFromAlgorithm(edges, "cooccurrence")
	if len(signals) != 1 {
		t.Fatalf("got %d signals, want 1", len(signals))
	}
	if signals[0].Algorithm != "cooccurrence" {
		t.Errorf("algorithm: got %q, want %q", signals[0].Algorithm, "cooccurrence")
	}
}

func TestAggregatedToEdges(t *testing.T) {
	aggregated := []AggregatedEdge{
		{
			Source:     "a.md",
			Target:     "b.md",
			Type:       EdgeRelated,
			Confidence: 0.65,
			Evidence:   "Semantic overlap: 0.55",
			Signals: []DiscoverySignal{
				{Source: "a.md", Target: "b.md", Algorithm: "semantic"},
				{Source: "a.md", Target: "b.md", Algorithm: "cooccurrence"},
			},
		},
	}
	edges := AggregatedToEdges(aggregated)
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	if edges[0].Source != "a.md" || edges[0].Target != "b.md" {
		t.Errorf("pair: got (%s,%s)", edges[0].Source, edges[0].Target)
	}
	// Evidence should note [2 signals].
	if edges[0].Evidence == "" {
		t.Error("evidence should not be empty")
	}
}

func TestAggregatedToEdges_SingleSignal_NoSuffix(t *testing.T) {
	aggregated := []AggregatedEdge{
		{
			Source:     "a.md",
			Target:     "b.md",
			Type:       EdgeRelated,
			Confidence: 0.6,
			Evidence:   "Semantic overlap: 0.50",
			Signals:    []DiscoverySignal{{Algorithm: "semantic"}},
		},
	}
	edges := AggregatedToEdges(aggregated)
	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	// Single signal should not have "[1 signals]" suffix.
	if edges[0].Evidence != "Semantic overlap: 0.50" {
		t.Errorf("evidence: got %q, want %q", edges[0].Evidence, "Semantic overlap: 0.50")
	}
}

func TestAggregateSignalsByLocation_SameLocationDedup(t *testing.T) {
	loc := RelationshipLocation{File: "docs/service.yaml", Line: 42}
	signals := []DiscoverySignal{
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.6, Algorithm: "cooccurrence", Location: loc},
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.8, Algorithm: "structural", Location: loc},
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.5, Algorithm: "semantic", Location: loc},
	}
	result := AggregateSignalsByLocation(signals)
	if len(result) != 1 {
		t.Fatalf("same source/target/location: got %d edges, want 1", len(result))
	}
	if len(result[0].Signals) != 3 {
		t.Errorf("signals count: got %d, want 3", len(result[0].Signals))
	}
}

func TestAggregateSignalsByLocation_WeightedAverage(t *testing.T) {
	loc := RelationshipLocation{File: "docs/service.yaml", Line: 42}
	signals := []DiscoverySignal{
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.6, Algorithm: "cooccurrence", Location: loc},
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.8, Algorithm: "structural", Location: loc},
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.5, Algorithm: "semantic", Location: loc},
	}
	result := AggregateSignalsByLocation(signals)
	if len(result) != 1 {
		t.Fatalf("got %d edges, want 1", len(result))
	}
	// Expected: (0.6*0.3 + 0.8*0.6 + 0.5*0.7) / (0.3+0.6+0.7) = (0.18 + 0.48 + 0.35) / 1.6 = 1.01/1.6 ≈ 0.63125
	expected := 0.63125
	if diff := result[0].Confidence - expected; diff > 0.001 || diff < -0.001 {
		t.Errorf("weighted avg: got %.5f, want ~%.5f", result[0].Confidence, expected)
	}
}

func TestAggregateSignalsByLocation_DifferentLocations(t *testing.T) {
	loc1 := RelationshipLocation{File: "file1.md", Line: 10}
	loc2 := RelationshipLocation{File: "file2.md", Line: 20}
	signals := []DiscoverySignal{
		{Source: "A", Target: "B", Type: EdgeDependsOn, Confidence: 0.7, Algorithm: "structural", Location: loc1},
		{Source: "A", Target: "B", Type: EdgeDependsOn, Confidence: 0.6, Algorithm: "structural", Location: loc2},
	}
	result := AggregateSignalsByLocation(signals)
	if len(result) != 2 {
		t.Fatalf("different locations: got %d edges, want 2", len(result))
	}
}

func TestAggregateSignalsByLocation_Empty(t *testing.T) {
	result := AggregateSignalsByLocation(nil)
	if result != nil {
		t.Errorf("nil input: got %d results, want nil", len(result))
	}
	result = AggregateSignalsByLocation([]DiscoverySignal{})
	if result != nil {
		t.Errorf("empty input: got %d results, want nil", len(result))
	}
}

func TestAggregateSignalsByLocation_UnknownAlgorithmWeight(t *testing.T) {
	loc := RelationshipLocation{File: "test.md", Line: 1}
	signals := []DiscoverySignal{
		{Source: "A", Target: "B", Type: EdgeDependsOn, Confidence: 0.8, Algorithm: "unknown-algo", Location: loc},
	}
	result := AggregateSignalsByLocation(signals)
	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}
	// Single signal: weighted avg = 0.8 * 0.5 / 0.5 = 0.8
	if result[0].Confidence != 0.8 {
		t.Errorf("confidence: got %.2f, want 0.8", result[0].Confidence)
	}
}

func TestAggregateSignalsByLocation_VerificationCalc(t *testing.T) {
	// Verification from plan: 3 signals for (payment-api → primary-db) at docs/service.yaml:42
	// Confidences: 0.6 (weight 0.3), 0.8 (weight 0.6), 0.5 (weight 0.7)
	// Expected: (0.6*0.3 + 0.8*0.6 + 0.5*0.7) / (0.3+0.6+0.7) = 0.63125
	loc := RelationshipLocation{File: "docs/service.yaml", Line: 42}
	signals := []DiscoverySignal{
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.6, Algorithm: "cooccurrence", Location: loc},
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.8, Algorithm: "structural", Location: loc},
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.5, Algorithm: "semantic", Location: loc},
	}
	result := AggregateSignalsByLocation(signals)
	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}
	expected := (0.6*0.3 + 0.8*0.6 + 0.5*0.7) / (0.3 + 0.6 + 0.7)
	if diff := result[0].Confidence - expected; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("verification calc: got %.6f, want %.6f", result[0].Confidence, expected)
	}
}

func TestAggregatedToEdges_SkipsInvalid(t *testing.T) {
	aggregated := []AggregatedEdge{
		{
			Source:     "a.md",
			Target:     "a.md", // self-loop, should be skipped
			Type:       EdgeRelated,
			Confidence: 0.6,
			Evidence:   "bad",
			Signals:    []DiscoverySignal{{Algorithm: "test"}},
		},
	}
	edges := AggregatedToEdges(aggregated)
	if len(edges) != 0 {
		t.Errorf("self-loop should be skipped: got %d edges", len(edges))
	}
}

// TestPageindexDeduplication_E2E simulates a realistic discovery pipeline where
// multiple algorithms detect the same relationships at the same file locations.
// Verifies the full signal -> location -> aggregation -> deduplication flow.
func TestPageindexDeduplication_E2E(t *testing.T) {
	// Simulate discovery signals as if pageindex had populated location data.
	// Scenario: a microservice corpus with payment-service referencing multiple dependencies.
	signals := []DiscoverySignal{
		// payment-api -> primary-db detected by 3 algorithms at same location
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.6, Algorithm: "cooccurrence",
			Location: RelationshipLocation{File: "payment-service/architecture.md", Line: 12, Evidence: "storage persistence"}},
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.8, Algorithm: "structural",
			Location: RelationshipLocation{File: "payment-service/architecture.md", Line: 12, Evidence: "## Storage section"}},
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.5, Algorithm: "semantic",
			Location: RelationshipLocation{File: "payment-service/architecture.md", Line: 12, Evidence: "semantic similarity"}},

		// payment-api -> redis-cache detected by 2 algorithms at same location
		{Source: "payment-api", Target: "redis-cache", Type: EdgeDependsOn, Confidence: 0.7, Algorithm: "ner",
			Location: RelationshipLocation{File: "payment-service/README.md", Line: 8, Evidence: "uses redis"}},
		{Source: "payment-api", Target: "redis-cache", Type: EdgeDependsOn, Confidence: 0.6, Algorithm: "cooccurrence",
			Location: RelationshipLocation{File: "payment-service/README.md", Line: 8, Evidence: "redis cache layer"}},

		// payment-api -> primary-db at DIFFERENT location (should NOT be merged with above)
		{Source: "payment-api", Target: "primary-db", Type: EdgeDependsOn, Confidence: 0.9, Algorithm: "structural",
			Location: RelationshipLocation{File: "payment-service/api.md", Line: 25, Evidence: "DB connection in API docs"}},

		// auth-service -> user-db single signal (no dedup needed)
		{Source: "auth-service", Target: "user-db", Type: EdgeDependsOn, Confidence: 0.75, Algorithm: "structural",
			Location: RelationshipLocation{File: "auth-service/architecture.md", Line: 5, Evidence: "user data store"}},
	}

	result := AggregateSignalsByLocation(signals)

	// Should produce 4 edges:
	// 1. payment-api -> primary-db at architecture.md:12 (3 signals merged)
	// 2. payment-api -> redis-cache at README.md:8 (2 signals merged)
	// 3. payment-api -> primary-db at api.md:25 (separate location)
	// 4. auth-service -> user-db at architecture.md:5 (single signal)
	if len(result) != 4 {
		t.Fatalf("expected 4 deduplicated edges, got %d", len(result))
	}

	// Build lookup map for easier assertions.
	type edgeKey struct {
		source, target, locKey string
	}
	edgeMap := make(map[edgeKey]AggregatedEdge)
	for _, edge := range result {
		key := edgeKey{edge.Source, edge.Target, RelationshipLocationKey(edge.Location)}
		edgeMap[key] = edge
	}

	// Check: payment-api -> primary-db at architecture.md:12 has 3 signals
	archEdge, ok := edgeMap[edgeKey{"payment-api", "primary-db", "payment-service/architecture.md:12"}]
	if !ok {
		t.Fatal("missing edge: payment-api -> primary-db at architecture.md:12")
	}
	if len(archEdge.Signals) != 3 {
		t.Errorf("architecture.md:12 edge signals: got %d, want 3", len(archEdge.Signals))
	}
	// Weighted avg: (0.6*0.3 + 0.8*0.6 + 0.5*0.7) / (0.3+0.6+0.7) = 0.63125
	expectedConf := (0.6*0.3 + 0.8*0.6 + 0.5*0.7) / (0.3 + 0.6 + 0.7)
	if math.Abs(archEdge.Confidence-expectedConf) > 0.001 {
		t.Errorf("architecture.md:12 confidence: got %.5f, want ~%.5f", archEdge.Confidence, expectedConf)
	}

	// Check: payment-api -> redis-cache at README.md:8 has 2 signals
	cacheEdge, ok := edgeMap[edgeKey{"payment-api", "redis-cache", "payment-service/README.md:8"}]
	if !ok {
		t.Fatal("missing edge: payment-api -> redis-cache at README.md:8")
	}
	if len(cacheEdge.Signals) != 2 {
		t.Errorf("README.md:8 edge signals: got %d, want 2", len(cacheEdge.Signals))
	}
	// Weighted avg: (0.7*0.5 + 0.6*0.3) / (0.5+0.3) = (0.35 + 0.18) / 0.8 = 0.6625
	expectedCache := (0.7*0.5 + 0.6*0.3) / (0.5 + 0.3)
	if math.Abs(cacheEdge.Confidence-expectedCache) > 0.001 {
		t.Errorf("README.md:8 confidence: got %.5f, want ~%.5f", cacheEdge.Confidence, expectedCache)
	}

	// Check: payment-api -> primary-db at api.md:25 is separate (1 signal)
	apiEdge, ok := edgeMap[edgeKey{"payment-api", "primary-db", "payment-service/api.md:25"}]
	if !ok {
		t.Fatal("missing edge: payment-api -> primary-db at api.md:25")
	}
	if len(apiEdge.Signals) != 1 {
		t.Errorf("api.md:25 edge signals: got %d, want 1", len(apiEdge.Signals))
	}
	if math.Abs(apiEdge.Confidence-0.9) > 0.001 {
		t.Errorf("api.md:25 confidence: got %.5f, want ~0.9", apiEdge.Confidence)
	}

	// Check: all locations are valid and deterministic
	for _, edge := range result {
		if !edge.Location.IsValid() {
			t.Errorf("edge %s->%s has invalid location: %s", edge.Source, edge.Target, edge.Location)
		}
		key := RelationshipLocationKey(edge.Location)
		if key == "" {
			t.Errorf("edge %s->%s has empty location key", edge.Source, edge.Target)
		}
	}

	// Verify determinism: run twice, same results
	result2 := AggregateSignalsByLocation(signals)
	if len(result) != len(result2) {
		t.Fatalf("determinism: run1=%d edges, run2=%d edges", len(result), len(result2))
	}
	for i := range result {
		if result[i].Source != result2[i].Source || result[i].Target != result2[i].Target {
			t.Errorf("determinism: edge %d differs between runs", i)
		}
		if math.Abs(result[i].Confidence-result2[i].Confidence) > 0.0001 {
			t.Errorf("determinism: edge %d confidence differs: %.5f vs %.5f", i, result[i].Confidence, result2[i].Confidence)
		}
	}
}

// BenchmarkAggregateSignalsByLocation benchmarks aggregation of 500+ signals.
func BenchmarkAggregateSignalsByLocation(b *testing.B) {
	// Generate 600 signals across 20 relationships with 3 locations each,
	// 10 algorithms per location.
	algorithms := []string{"cooccurrence", "ner", "structural", "semantic", "llm"}
	var signals []DiscoverySignal
	for i := 0; i < 20; i++ {
		source := fmt.Sprintf("service-%d", i)
		target := fmt.Sprintf("db-%d", i)
		for line := 0; line < 3; line++ {
			loc := RelationshipLocation{File: fmt.Sprintf("docs/service-%d.md", i), Line: line*10 + 1}
			for _, algo := range algorithms {
				signals = append(signals, DiscoverySignal{
					Source:     source,
					Target:     target,
					Type:       EdgeDependsOn,
					Confidence: 0.5 + float64(line)*0.1,
					Algorithm:  algo,
					Location:   loc,
				})
			}
			// Add duplicate signals from same algorithms for realistic volume
			for _, algo := range algorithms {
				signals = append(signals, DiscoverySignal{
					Source:     source,
					Target:     target,
					Type:       EdgeMentions,
					Confidence: 0.4 + float64(line)*0.1,
					Algorithm:  algo,
					Location:   loc,
				})
			}
		}
	}

	if len(signals) < 500 {
		b.Fatalf("expected 500+ signals, got %d", len(signals))
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		AggregateSignalsByLocation(signals)
	}
}
