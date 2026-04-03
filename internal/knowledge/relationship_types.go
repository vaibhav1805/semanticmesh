package knowledge

import "strings"

// Additional SignalSource constants for discovery algorithms.
const (
	// SignalCoOccurrence is a relationship inferred from co-occurrence of
	// component names within a sliding window of text (0.4–0.65).
	SignalCoOccurrence SignalSource = "co-occurrence"

	// SignalStructural is a relationship inferred from structural patterns
	// in heading hierarchy (e.g. "Dependencies", "Requires" sections) (0.75+).
	SignalStructural SignalSource = "structural"

	// SignalNER is a relationship detected via Named Entity Recognition
	// and Subject-Verb-Object extraction from prose (0.5–0.7).
	SignalNER SignalSource = "NER"
)

// DiscoveredEdge wraps an Edge with additional discovery metadata: the list of
// supporting signals and the algorithm that produced it.
type DiscoveredEdge struct {
	*Edge

	// Signals holds all evidence signals supporting this edge.
	Signals []Signal
}

// classifyHeadingToEdgeType maps a heading text (lowercased) to an EdgeType.
// Returns EdgeMentions as a fallback when no specific pattern matches.
func classifyHeadingToEdgeType(heading string) EdgeType {
	lower := strings.ToLower(heading)

	switch {
	case strings.Contains(lower, "dependencies") || strings.Contains(lower, "depends on"):
		return EdgeDependsOn
	case strings.Contains(lower, "requires") || strings.Contains(lower, "prerequisites"):
		return EdgeDependsOn
	case strings.Contains(lower, "calls") || strings.Contains(lower, "invokes"):
		return EdgeCalls
	case strings.Contains(lower, "implements") || strings.Contains(lower, "implementation"):
		return EdgeImplements
	case strings.Contains(lower, "integrat"):
		return EdgeMentions
	case strings.Contains(lower, "related"):
		return EdgeMentions
	default:
		return EdgeMentions
	}
}

// dependencySectionNames is the set of heading keywords that indicate a section
// listing relationships to other components.
var dependencySectionNames = []string{
	"dependencies",
	"depends on",
	"requires",
	"prerequisites",
	"calls",
	"integrates",
	"integration points",
	"integration",
	"related services",
	"related",
}

// isDependencySection returns true if heading (lowercased) matches one of the
// known dependency-indicator section names.
func isDependencySection(heading string) bool {
	lower := strings.ToLower(strings.TrimSpace(heading))
	for _, name := range dependencySectionNames {
		if strings.Contains(lower, name) {
			return true
		}
	}
	return false
}
