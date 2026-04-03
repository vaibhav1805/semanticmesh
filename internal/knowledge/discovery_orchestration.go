package knowledge

import (
	"sync"
)

// DiscoveryFilterConfig controls the quality thresholds for filtering
// implicit relationship discoveries before user review.
//
// The filter logic implements a tiered confidence+signal model:
//   - Solo high-confidence signal: accepted unconditionally.
//   - Two or more signals agree: lower confidence bar accepted.
//   - Three or more signals agree: even lower bar accepted.
//   - Below all thresholds with fewer signals: rejected as false positive.
type DiscoveryFilterConfig struct {
	// MinConfidence is the threshold for edges supported by a single signal.
	// Edges with Confidence >= MinConfidence pass regardless of signal count.
	MinConfidence float64

	// MinConfidenceDual is the threshold when 2+ signals agree on an edge.
	MinConfidenceDual float64

	// MinConfidenceTriple is the threshold when 3+ signals agree.
	MinConfidenceTriple float64

	// MinTier optionally filters by named confidence tier. When set, only
	// edges whose tier is at least this confident pass. Takes precedence
	// over MinScore when both are set (stricter of the two wins).
	MinTier *ConfidenceTier

	// MinScore optionally filters by numeric confidence threshold. When set,
	// only edges with Confidence >= MinScore pass.
	MinScore *float64
}

// DefaultDiscoveryFilterConfig returns the recommended production filter settings.
// These values were chosen to balance LLM discovery pass-through with false
// positive reduction across multi-signal and single-signal edges.
//
// Threshold rationale:
//   - MinConfidence (0.60): Solo non-LLM signal; lowered from 0.75 to allow
//     single-algorithm discoveries that would otherwise require corroboration.
//   - MinConfidenceDual (0.65): Two-signal agreement at moderate confidence;
//     slightly higher than solo since we can demand more from corroborated edges.
//   - MinConfidenceTriple (0.60): Three-signal agreement at the floor.
//
// Note: passesFilter() applies a special sub-threshold (0.50) for pure LLM
// signals (SignalLLM), since LLM reasoning about document purpose deserves
// more latitude than simple pattern-matching algorithms.
func DefaultDiscoveryFilterConfig() DiscoveryFilterConfig {
	return DiscoveryFilterConfig{
		MinConfidence:       0.60,
		MinConfidenceDual:   0.65,
		MinConfidenceTriple: 0.60,
	}
}

// FilterDiscoveredEdges applies signal quality filtering to a slice of
// discovered edges. Edges that fail the quality gate are discarded as
// likely false positives.
//
// Edges pass the filter when any of the following hold:
//   - Confidence >= cfg.MinConfidence (solo strong signal)
//   - Confidence >= cfg.MinConfidenceDual AND len(Signals) >= 2
//   - Confidence >= cfg.MinConfidenceTriple AND len(Signals) >= 3
func FilterDiscoveredEdges(edges []*DiscoveredEdge, cfg DiscoveryFilterConfig) []*DiscoveredEdge {
	out := make([]*DiscoveredEdge, 0, len(edges))
	for _, de := range edges {
		if !passesFilter(de, cfg) {
			continue
		}
		// Apply optional tier-based filter.
		if cfg.MinTier != nil && de.Edge != nil && IsValidConfidenceScore(de.Confidence) {
			tier := ScoreToTier(de.Confidence)
			if !TierAtLeast(tier, *cfg.MinTier) {
				continue
			}
		}
		// Apply optional numeric score filter.
		if cfg.MinScore != nil && de.Edge != nil {
			if de.Confidence < *cfg.MinScore {
				continue
			}
		}
		out = append(out, de)
	}
	return out
}

// passesFilter returns true when de meets at least one tier of the quality gate.
//
// Special case: edges with a single SignalLLM signal use a lower threshold
// (0.50) than the standard solo tier. LLM signals reason about document
// purpose rather than counting syntactic patterns, so they should not be
// required to reach the same confidence bar as co-occurrence or NER signals.
func passesFilter(de *DiscoveredEdge, cfg DiscoveryFilterConfig) bool {
	if de == nil || de.Edge == nil {
		return false
	}

	// Special handling for pure LLM signal: lower threshold since LLM reasons
	// about purpose, not just surface patterns.
	if len(de.Signals) == 1 && de.Signals[0].SourceType == SignalLLM {
		return de.Confidence >= 0.50
	}

	c := de.Confidence
	n := len(de.Signals)
	switch {
	case c >= cfg.MinConfidence:
		return true
	case c >= cfg.MinConfidenceDual && n >= 2:
		return true
	case c >= cfg.MinConfidenceTriple && n >= 3:
		return true
	default:
		return false
	}
}

// convertSemanticToDiscovered wraps SemanticRelationships edges as
// DiscoveredEdge values with a single signal tagged "semantic".
func convertSemanticToDiscovered(edges []*Edge) []*DiscoveredEdge {
	result := make([]*DiscoveredEdge, 0, len(edges))
	for _, e := range edges {
		de := &DiscoveredEdge{
			Edge: e,
			Signals: []Signal{{
				SourceType: SignalCoOccurrence, // closest available type; evidence field carries algorithm tag
				Confidence: e.Confidence,
				Evidence:   "semantic: " + e.Evidence,
				Weight:     1.0,
			}},
		}
		result = append(result, de)
	}
	return result
}

// convertNERToDiscovered wraps NERRelationships edges as DiscoveredEdge
// values with a single signal tagged "nersvo".
func convertNERToDiscovered(edges []*Edge) []*DiscoveredEdge {
	result := make([]*DiscoveredEdge, 0, len(edges))
	for _, e := range edges {
		de := &DiscoveredEdge{
			Edge: e,
			Signals: []Signal{{
				SourceType: SignalMention,
				Confidence: e.Confidence,
				Evidence:   "nersvo: " + e.Evidence,
				Weight:     1.0,
			}},
		}
		result = append(result, de)
	}
	return result
}

// FilterSignalsByTier returns only the discovered edges whose confidence
// maps to a tier at least as high as minTier.
func FilterSignalsByTier(edges []*DiscoveredEdge, minTier ConfidenceTier) []*DiscoveredEdge {
	out := make([]*DiscoveredEdge, 0, len(edges))
	for _, de := range edges {
		if de == nil || de.Edge == nil {
			continue
		}
		if !IsValidConfidenceScore(de.Confidence) {
			continue
		}
		tier := ScoreToTier(de.Confidence)
		if TierAtLeast(tier, minTier) {
			out = append(out, de)
		}
	}
	return out
}

// FilterByConfidenceScore returns only the discovered edges whose confidence
// is at least minScore.
func FilterByConfidenceScore(edges []*DiscoveredEdge, minScore float64) []*DiscoveredEdge {
	out := make([]*DiscoveredEdge, 0, len(edges))
	for _, de := range edges {
		if de == nil || de.Edge == nil {
			continue
		}
		if de.Confidence >= minScore {
			out = append(out, de)
		}
	}
	return out
}

// DiscoverAndIntegrateRelationships runs all 4 discovery algorithms in
// parallel and returns filtered (quality-gated) and unfiltered results.
//
// Parameters:
//
//	documents     — the full corpus to analyse
//	idx           — (deprecated, unused; kept for backward compat)
//	explicitGraph — the baseline explicit graph; edges already present are excluded
//	               from the returned filtered slice (they don't need review)
//	cfg           — filter thresholds; use DefaultDiscoveryFilterConfig() for defaults
//	llmCfg        — LLM discovery configuration (model, cache, concurrency)
//	trees         — hierarchical document trees (built by TreeNodeFromMarkdown or loaded)
//	knownComponents — canonical component IDs for LLM reasoning
//
// Returns:
//
//	filtered — edges that passed quality gate, ready for user review
//	all      — all discovered edges before filtering (for stats / debugging)
func DiscoverAndIntegrateRelationships(
	documents []Document,
	idx *BM25Index,
	explicitGraph *Graph,
	cfg DiscoveryFilterConfig,
	llmCfg LLMDiscoveryConfig,
	trees []FileTree,
	knownComponents []string,
) (filtered []*DiscoveredEdge, all []*DiscoveredEdge) {
	if len(documents) == 0 {
		return nil, nil
	}

	componentNames := BuildComponentNameMap(documents)

	// Run 4 algorithms concurrently. Each goroutine writes to its own
	// variable so no mutex is needed.
	var (
		coOccEdges []*DiscoveredEdge
		structEdges []*DiscoveredEdge
		llmEdges   []*DiscoveredEdge
		nerEdges   []*DiscoveredEdge
	)

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		coOccEdges = CoOccurrenceRelationships(documents, componentNames, DefaultCoOccurrenceConfig())
	}()
	go func() {
		defer wg.Done()
		structEdges = StructuralRelationships(documents, componentNames)
	}()
	go func() {
		defer wg.Done()
		// Use DiscoverWithClaude (direct API, no subprocess) when trees and
		// components are provided. Falls back to nil when not configured.
		llmEdges = DiscoverWithClaude(documents, trees, knownComponents, llmCfg, nil)
	}()
	go func() {
		defer wg.Done()
		nerEdges = convertNERToDiscovered(NERRelationships(documents))
	}()

	wg.Wait()

	// Merge all signals across algorithms (aggregates by source+target+type key).
	all = MergeDiscoveredEdges(coOccEdges, structEdges, llmEdges, nerEdges)

	// Build explicit edge set to exclude already-known relationships from review.
	explicitKeys := make(map[string]bool)
	if explicitGraph != nil {
		for _, e := range explicitGraph.Edges {
			k := e.Source + "\x00" + e.Target + "\x00" + string(e.Type)
			explicitKeys[k] = true
		}
	}

	// Apply quality filter, skipping edges already present in the explicit graph.
	candidates := make([]*DiscoveredEdge, 0, len(all))
	for _, de := range all {
		if de.Edge == nil {
			continue
		}
		k := de.Source + "\x00" + de.Target + "\x00" + string(de.Type)
		if explicitKeys[k] {
			continue // already explicit — skip, no review needed
		}
		candidates = append(candidates, de)
	}

	filtered = FilterDiscoveredEdges(candidates, cfg)
	return filtered, all
}
