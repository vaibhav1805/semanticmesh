package knowledge

import (
	"sort"

	"github.com/vaibhav1805/semanticmesh/internal/code"
)

// convertCodeSignalsToDiscovered converts CodeSignal values from code analysis
// into DiscoveredEdge values suitable for the graph merge pipeline.
//
// Signals are grouped by (sourceComponent, targetComponent, edgeType) key.
// For each unique key, a single DiscoveredEdge is produced with the highest
// confidence and all contributing signals preserved. Self-loops and empty
// targets are silently skipped.
//
// All code detection kinds map to EdgeDependsOn; fine-grained type info is
// preserved in the code_signals provenance table.
func convertCodeSignalsToDiscovered(signals []code.CodeSignal, sourceComponent string) []*DiscoveredEdge {
	type groupKey struct {
		source, target string
		edgeType       EdgeType
	}

	type group struct {
		key            groupKey
		bestConfidence float64
		bestEvidence   string
		bestSourceFile string
		signals        []Signal
	}

	groups := make(map[string]*group) // string key for deterministic iteration
	var keyOrder []string

	for _, sig := range signals {
		// Skip self-loops and empty targets.
		if sig.TargetComponent == "" || sig.TargetComponent == sourceComponent {
			continue
		}

		gk := groupKey{
			source:   sourceComponent,
			target:   sig.TargetComponent,
			edgeType: EdgeDependsOn,
		}
		// Use null-byte separator for unique key.
		strKey := gk.source + "\x00" + gk.target + "\x00" + string(gk.edgeType)

		s := Signal{
			SourceType: SignalCode,
			Confidence: sig.Confidence,
			Evidence:   sig.Evidence,
			Weight:     AlgorithmWeight["code"],
		}

		if g, ok := groups[strKey]; ok {
			g.signals = append(g.signals, s)
			if sig.Confidence > g.bestConfidence {
				g.bestConfidence = sig.Confidence
				g.bestEvidence = sig.Evidence
				g.bestSourceFile = sig.SourceFile
			}
		} else {
			groups[strKey] = &group{
				key:            gk,
				bestConfidence: sig.Confidence,
				bestEvidence:   sig.Evidence,
				bestSourceFile: sig.SourceFile,
				signals:        []Signal{s},
			}
			keyOrder = append(keyOrder, strKey)
		}
	}

	// Sort keys for deterministic output.
	sort.Strings(keyOrder)

	results := make([]*DiscoveredEdge, 0, len(keyOrder))
	for _, strKey := range keyOrder {
		g := groups[strKey]

		edge, err := NewEdge(g.key.source, g.key.target, g.key.edgeType, g.bestConfidence, g.bestEvidence)
		if err != nil {
			continue // skip invalid edges
		}
		edge.ExtractionMethod = "code-analysis"
		edge.SourceFile = g.bestSourceFile
		edge.SourceType = "code"

		results = append(results, &DiscoveredEdge{
			Edge:    edge,
			Signals: g.signals,
		})
	}

	return results
}

// applyMultiSourceConfidence applies probabilistic OR confidence merging for
// edges that have signals from both markdown and code sources.
//
// Formula: merged = 1.0 - (1.0 - mdConf) * (1.0 - codeConf)
//
// For edges with only one source type, confidence is left unchanged.
func applyMultiSourceConfidence(edges []*DiscoveredEdge) {
	for _, de := range edges {
		var mdConf, codeConf float64
		var hasMd, hasCode bool

		for _, sig := range de.Signals {
			if sig.SourceType == SignalCode {
				if sig.Confidence > codeConf {
					codeConf = sig.Confidence
				}
				hasCode = true
			} else {
				if sig.Confidence > mdConf {
					mdConf = sig.Confidence
				}
				hasMd = true
			}
		}

		if hasMd && hasCode {
			merged := 1.0 - (1.0-mdConf)*(1.0-codeConf)
			if merged > 1.0 {
				merged = 1.0
			}
			de.Confidence = merged
			de.Edge.Confidence = merged
		}
	}
}

// determineSourceType returns "markdown", "code", or "both" based on the
// signal sources present in the DiscoveredEdge.
func determineSourceType(de *DiscoveredEdge) string {
	var hasCode, hasMd bool
	for _, sig := range de.Signals {
		if sig.SourceType == SignalCode {
			hasCode = true
		} else {
			hasMd = true
		}
	}
	switch {
	case hasCode && hasMd:
		return "both"
	case hasCode:
		return "code"
	default:
		return "markdown"
	}
}

// integrateCodeSignals is the shared integration logic used by both CmdExport
// and CmdCrawl to convert raw code signals into graph edges, merge them with
// markdown-discovered edges, apply probabilistic OR confidence, and set
// source_type on each edge. Returns the merged discovered edges slice.
func integrateCodeSignals(graph *Graph, discovered []*DiscoveredEdge, signals []code.CodeSignal, sourceComponent string) []*DiscoveredEdge {
	// Convert code signals to DiscoveredEdge values.
	codeEdges := convertCodeSignalsToDiscovered(signals, sourceComponent)

	// Create stub nodes for targets not already in the graph.
	ensureCodeTargetNodes(graph, codeEdges)

	// Merge code edges with markdown-discovered edges.
	allEdges := MergeDiscoveredEdges(discovered, codeEdges)

	// Apply probabilistic OR for edges detected by both markdown and code.
	applyMultiSourceConfidence(allEdges)

	// Set source_type on each edge and add code edges to graph.
	for _, de := range allEdges {
		if de.Edge != nil {
			de.Edge.SourceType = determineSourceType(de)
			// Add code-originated edges to the graph (markdown edges already added).
			if de.Edge.SourceType == "code" || de.Edge.SourceType == "both" {
				_ = graph.AddEdge(de.Edge)
			}
		}
	}

	return allEdges
}

// ensureCodeTargetNodes creates stub graph nodes for code-detected targets
// (and sources) that do not already exist in the graph. This prevents
// dangling edge references when code signals introduce new components not
// found in markdown documentation.
func ensureCodeTargetNodes(g *Graph, codeEdges []*DiscoveredEdge) {
	for _, de := range codeEdges {
		// Check target.
		if _, ok := g.Nodes[de.Target]; !ok {
			_ = g.AddNode(&Node{
				ID:            de.Target,
				Type:          "infrastructure",
				Title:         de.Target,
				ComponentType: ComponentTypeUnknown,
			})
		}

		// Check source (may not be a markdown-originated node).
		if _, ok := g.Nodes[de.Source]; !ok {
			_ = g.AddNode(&Node{
				ID:            de.Source,
				Type:          "infrastructure",
				Title:         de.Source,
				ComponentType: ComponentTypeUnknown,
			})
		}
	}
}
