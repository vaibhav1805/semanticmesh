package knowledge

import (
	"sort"
	"strings"

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

	// Build a map of target component → best target type from original signals.
	// This preserves type information from go.mod/SDK detection that would
	// otherwise be lost during conversion to DiscoveredEdge.
	signalTargetTypes := buildSignalTargetTypes(signals)

	// Create stub nodes for targets not already in the graph.
	ensureCodeTargetNodes(graph, codeEdges, signalTargetTypes)

	// Merge code edges with markdown-discovered edges.
	allEdges := MergeDiscoveredEdges(discovered, codeEdges)

	// Cross-reference Terraform resources with code-detected components.
	allEdges = crossReferenceTerraformSignals(graph, allEdges, signals, sourceComponent)

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
//
// Noise filtering: targets that look like common English words, bare URL
// domains, or very short tokens are skipped to prevent polluting the graph
// with non-component names extracted from code comments.
func ensureCodeTargetNodes(g *Graph, codeEdges []*DiscoveredEdge, signalTargetTypes map[string]string) {
	for _, de := range codeEdges {
		// Check target.
		if _, ok := g.Nodes[de.Target]; !ok {
			if isNoiseTarget(de.Target) {
				continue
			}
			ct := resolveComponentType(de.Target, signalTargetTypes)
			_ = g.AddNode(&Node{
				ID:            de.Target,
				Type:          "infrastructure",
				Title:         de.Target,
				ComponentType: ct,
			})
		}

		// Check source (may not be a markdown-originated node).
		if _, ok := g.Nodes[de.Source]; !ok {
			if isNoiseTarget(de.Source) {
				continue
			}
			ct := resolveComponentType(de.Source, signalTargetTypes)
			_ = g.AddNode(&Node{
				ID:            de.Source,
				Type:          "infrastructure",
				Title:         de.Source,
				ComponentType: ct,
			})
		}
	}
}

// buildSignalTargetTypes creates a map from target component name to the best
// (most specific) target type across all signals. Prefers non-"unknown" types.
func buildSignalTargetTypes(signals []code.CodeSignal) map[string]string {
	types := make(map[string]string)
	for _, sig := range signals {
		if sig.TargetComponent == "" {
			continue
		}
		existing := types[sig.TargetComponent]
		// Prefer non-unknown types; among non-unknown, prefer higher confidence signals.
		if existing == "" || existing == "unknown" {
			types[sig.TargetComponent] = sig.TargetType
		}
	}
	return types
}

// resolveComponentType determines the best ComponentType for a node by checking
// the signal-provided target type first, then falling back to InferComponentType.
func resolveComponentType(name string, signalTargetTypes map[string]string) ComponentType {
	// First try the signal's target type (from go.mod, SDK detection, etc.).
	if st, ok := signalTargetTypes[name]; ok && st != "" && st != "unknown" {
		return ComponentType(st)
	}
	// Fall back to name-based inference.
	ct, _ := InferComponentType(name)
	return ct
}

// isNoiseTarget returns true when name looks like a common English word,
// a bare URL/domain, or is too short to be a meaningful component name.
// These arise when code-analysis extractors match dependency-verb patterns
// in code comments (e.g. "uses the", "calls made", "depends on here").
func isNoiseTarget(name string) bool {
	if len(name) < 3 {
		return true
	}

	lower := strings.ToLower(name)

	// Reject bare domains (contain dots but no slashes indicating a path).
	if strings.Contains(lower, ".") && !strings.Contains(lower, "/") {
		parts := strings.Split(lower, ".")
		lastPart := parts[len(parts)-1]
		// Common TLDs indicate a URL domain, not a component.
		switch lastPart {
		case "com", "io", "org", "net", "dev", "app", "co", "cloud", "ai":
			return true
		}
	}

	// Reject common English stop-words that appear after dependency verbs
	// in code comments (e.g. "depends on the", "uses a", "calls here").
	if codeNoiseWords[lower] {
		return true
	}

	return false
}

// ─── Terraform cross-referencing ────────────────────────────────────────────

// crossReferenceTerraformSignals creates edges between code-detected components
// and Terraform resources when they likely represent the same infrastructure.
// Matching is gated on component type: a code "database" signal can only match
// a Terraform "database" resource. Three strategies are tried in priority order:
//
//  1. TFvars hostname exact match (confidence 0.80)
//  2. Name token overlap (confidence 0.55–0.70)
//  3. Sole type match — exactly one TF resource of the matching type (confidence 0.60)
func crossReferenceTerraformSignals(
	graph *Graph,
	allEdges []*DiscoveredEdge,
	signals []code.CodeSignal,
	sourceComponent string,
) []*DiscoveredEdge {
	// Partition signals by language.
	tfByType := make(map[string][]string)   // targetType → []tfTargetComponent
	tfTargetType := make(map[string]string)  // tfTargetComponent → targetType
	tfvarsHosts := make(map[string]string)   // hostname → targetType (from .tfvars)
	codeTargets := make(map[string]string)   // codeTargetComponent → targetType

	for _, sig := range signals {
		if sig.TargetComponent == "" || sig.TargetComponent == sourceComponent {
			continue
		}
		if sig.Language == "terraform" {
			tt := sig.TargetType
			if tt == "" || tt == "unknown" {
				continue
			}
			// TFvars env_var_ref signals are variable values (hostnames), not
			// actual TF resources — track them separately for Strategy 1.
			if sig.DetectionKind == "env_var_ref" {
				tfvarsHosts[sig.TargetComponent] = tt
				continue
			}
			// Deduplicate TF resource/module/data targets per type.
			if _, exists := tfTargetType[sig.TargetComponent]; !exists {
				tfTargetType[sig.TargetComponent] = tt
				tfByType[tt] = append(tfByType[tt], sig.TargetComponent)
			}
		} else {
			tt := sig.TargetType
			if tt == "" || tt == "unknown" {
				continue
			}
			// Keep best (first seen) type per code target.
			if _, exists := codeTargets[sig.TargetComponent]; !exists {
				codeTargets[sig.TargetComponent] = tt
			}
		}
	}

	// Nothing to cross-reference if either side is empty.
	if len(tfTargetType) == 0 || len(codeTargets) == 0 {
		return allEdges
	}

	// For each code target, try to find a matching TF resource.
	for codeTarget, codeType := range codeTargets {
		// Skip if this code target is also a TF target (already connected).
		if _, isTF := tfTargetType[codeTarget]; isTF {
			continue
		}

		var bestTF string
		var bestConf float64

		// Strategy 1: TFvars hostname exact match.
		// If codeTarget appears as a hostname in .tfvars AND there's a TF resource of matching type.
		if _, inTfvars := tfvarsHosts[codeTarget]; inTfvars {
			// Find a TF resource of matching type to link to.
			candidates := tfByType[codeType]
			if len(candidates) == 1 {
				bestTF = candidates[0]
				bestConf = 0.80
			} else if len(candidates) > 1 {
				// Multiple TF resources of this type — pick by name similarity.
				tf, score := bestNameMatch(codeTarget, candidates)
				if score >= 0.3 {
					bestTF = tf
					bestConf = 0.80
				}
			}
		}

		// Strategy 2: Name similarity (only if Strategy 1 didn't match).
		if bestTF == "" {
			candidates := tfByType[codeType]
			if len(candidates) > 0 {
				tf, score := bestNameMatch(codeTarget, candidates)
				if score >= 0.5 {
					bestTF = tf
					bestConf = 0.70
				} else if score >= 0.3 {
					bestTF = tf
					bestConf = 0.55
				}
			}
		}

		// Strategy 3: Sole type match (only if nothing else matched).
		if bestTF == "" {
			candidates := tfByType[codeType]
			if len(candidates) == 1 {
				bestTF = candidates[0]
				bestConf = 0.60
			}
		}

		if bestTF == "" {
			continue
		}

		evidence := "terraform-crossref: " + codeTarget + " ↔ " + bestTF
		edge, err := NewEdge(codeTarget, bestTF, EdgeDependsOn, bestConf, evidence)
		if err != nil {
			continue
		}
		edge.ExtractionMethod = "terraform-crossref"
		edge.SourceType = "code"

		de := &DiscoveredEdge{
			Edge: edge,
			Signals: []Signal{{
				SourceType: SignalCode,
				Confidence: bestConf,
				Evidence:   evidence,
				Weight:     AlgorithmWeight["code"],
			}},
		}

		_ = graph.AddEdge(edge)
		allEdges = append(allEdges, de)
	}

	return allEdges
}

// bestNameMatch finds the TF target from candidates with the highest token
// overlap score against name. Returns the best candidate and its score.
// If multiple candidates tie at the best score, returns empty (ambiguous).
func bestNameMatch(name string, candidates []string) (string, float64) {
	nameTokens := tokenizeName(name)
	if len(nameTokens) == 0 {
		return "", 0
	}

	var best string
	var bestScore float64
	var tied bool

	for _, cand := range candidates {
		score := tokenOverlapScore(nameTokens, tokenizeName(cand))
		if score > bestScore {
			best = cand
			bestScore = score
			tied = false
		} else if score == bestScore && score > 0 {
			tied = true
		}
	}

	if tied {
		return "", 0
	}
	return best, bestScore
}

// tokenizeName splits a name on common delimiters (`.`, `-`, `_`, `/`),
// lowercases the tokens, removes noise words, and deduplicates.
func tokenizeName(name string) []string {
	// Replace delimiters with spaces, then split.
	r := strings.NewReplacer(".", " ", "-", " ", "_", " ", "/", " ")
	parts := strings.Fields(r.Replace(strings.ToLower(name)))

	seen := make(map[string]bool, len(parts))
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) < 2 || xrefNoiseTokens[p] || seen[p] {
			continue
		}
		seen[p] = true
		tokens = append(tokens, p)
	}
	return tokens
}

// tokenOverlapScore returns |intersection| / min(|a|, |b|).
// Returns 0 if either slice is empty.
func tokenOverlapScore(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	set := make(map[string]bool, len(b))
	for _, t := range b {
		set[t] = true
	}

	shared := 0
	for _, t := range a {
		if set[t] {
			shared++
		}
	}

	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	return float64(shared) / float64(minLen)
}

// xrefNoiseTokens are common tokens in Terraform resource addresses and
// hostnames that carry no semantic meaning for cross-reference matching.
var xrefNoiseTokens = map[string]bool{
	"aws": true, "instance": true, "cluster": true, "group": true,
	"internal": true, "local": true, "default": true, "main": true,
	"this": true, "module": true, "data": true, "resource": true,
	"replication": true, "terraform": true, "provider": true,
}

// codeNoiseWords is a set of common English words that should never become
// component names. These frequently appear after dependency-verb patterns
// ("uses X", "depends on X", "calls X") in code comments.
var codeNoiseWords = map[string]bool{
	// Articles and pronouns
	"the": true, "a": true, "an": true, "this": true, "that": true,
	"it": true, "its": true, "they": true, "them": true, "their": true,
	"we": true, "our": true, "you": true, "your": true,
	// Common adverbs / adjectives
	"all": true, "any": true, "each": true, "every": true, "some": true,
	"non": true, "new": true, "old": true, "same": true, "other": true,
	"more": true, "most": true, "only": true, "own": true,
	// Prepositions and conjunctions
	"for": true, "from": true, "with": true, "into": true, "here": true,
	"there": true, "where": true, "when": true, "then": true, "also": true,
	"not": true, "but": true, "and": true,
	// Generic programming words
	"data": true, "value": true, "result": true, "error": true,
	"input": true, "output": true, "file": true, "path": true,
	"name": true, "type": true, "list": true, "map": true,
	"set": true, "get": true, "run": true, "test": true,
	"true": true, "false": true, "null": true, "nil": true,
	"made": true, "just": true, "been": true, "being": true,
	"both": true, "well": true, "still": true,
	// Common verbs that slip through as captured "names"
	"done": true, "used": true, "called": true, "given": true,
	"based": true, "needed": true, "required": true,
	// Pagination / generic nouns from code comments
	"pagination": true, "information": true, "configuration": true,
	"implementation": true, "documentation": true,
}
