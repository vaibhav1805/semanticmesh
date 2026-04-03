package knowledge

import "fmt"

// ConfidenceTier represents the semantic interpretation of a numeric confidence
// score. Tiers map raw scores in [0.4, 1.0] to human-readable categories that
// describe how a relationship was discovered and how trustworthy it is.
type ConfidenceTier string

const (
	// TierExplicit indicates a relationship sourced from service manifests,
	// configuration files, or explicit dependency declarations. Score >= 0.95.
	TierExplicit ConfidenceTier = "explicit"

	// TierStrongInference indicates a relationship inferred from code analysis,
	// structural patterns, or multiple algorithm agreement. Score >= 0.75.
	TierStrongInference ConfidenceTier = "strong-inference"

	// TierModerate indicates a relationship validated by two or more discovery
	// algorithms with reasonable confidence. Score >= 0.55.
	TierModerate ConfidenceTier = "moderate"

	// TierWeak indicates a relationship detected by a single algorithm with
	// lower signal confidence. Score >= 0.45.
	TierWeak ConfidenceTier = "weak"

	// TierSemantic indicates a relationship inferred from NLP/LLM similarity
	// or speculative analysis. Score >= 0.42.
	TierSemantic ConfidenceTier = "semantic"

	// TierThreshold is the minimum acceptable confidence for any relationship
	// to be included in the graph. Score >= 0.4.
	TierThreshold ConfidenceTier = "threshold"
)

// allConfidenceTiers is the canonical list of confidence tiers in descending
// confidence order. Used by AllConfidenceTiers() and tier comparison.
var allConfidenceTiers = []ConfidenceTier{
	TierExplicit,
	TierStrongInference,
	TierModerate,
	TierWeak,
	TierSemantic,
	TierThreshold,
}

// tierRank maps each tier to its ordinal rank (0 = highest confidence).
// Used for tier comparison in filtering.
var tierRank map[ConfidenceTier]int

func init() {
	tierRank = make(map[ConfidenceTier]int, len(allConfidenceTiers))
	for i, t := range allConfidenceTiers {
		tierRank[t] = i
	}
}

// ScoreToTier maps a numeric confidence score in [0.4, 1.0] to its
// corresponding ConfidenceTier. Panics if score is below 0.4.
func ScoreToTier(score float64) ConfidenceTier {
	switch {
	case score >= 0.95:
		return TierExplicit
	case score >= 0.75:
		return TierStrongInference
	case score >= 0.55:
		return TierModerate
	case score >= 0.45:
		return TierWeak
	case score >= 0.42:
		return TierSemantic
	case score >= 0.4:
		return TierThreshold
	default:
		panic(fmt.Sprintf("confidence score %.4f is below minimum threshold 0.4", score))
	}
}

// TierToScore returns the canonical numeric score for a named tier.
// Returns an error for unknown tier names.
func TierToScore(tier ConfidenceTier) (float64, error) {
	switch tier {
	case TierExplicit:
		return 1.0, nil
	case TierStrongInference:
		return 0.8, nil
	case TierModerate:
		return 0.6, nil
	case TierWeak:
		return 0.5, nil
	case TierSemantic:
		return 0.45, nil
	case TierThreshold:
		return 0.4, nil
	default:
		return 0, fmt.Errorf("unknown confidence tier: %q", tier)
	}
}

// IsValidConfidenceScore returns true when score is within the valid
// confidence range [0.4, 1.0].
func IsValidConfidenceScore(score float64) bool {
	return score >= 0.4 && score <= 1.0
}

// AllConfidenceTiers returns all 6 confidence tiers in descending confidence
// order, from explicit (highest) to threshold (lowest).
func AllConfidenceTiers() []ConfidenceTier {
	out := make([]ConfidenceTier, len(allConfidenceTiers))
	copy(out, allConfidenceTiers)
	return out
}

// TierDisplayName returns a human-readable tier name with its canonical score.
func TierDisplayName(tier ConfidenceTier) string {
	score, err := TierToScore(tier)
	if err != nil {
		return string(tier)
	}
	return fmt.Sprintf("%s (%.2f)", tier, score)
}

// TierAtLeast returns true when tier is at least as confident as minTier.
// For example, TierAtLeast(TierModerate, TierWeak) returns true because
// moderate is more confident than weak.
func TierAtLeast(tier, minTier ConfidenceTier) bool {
	r, ok1 := tierRank[tier]
	m, ok2 := tierRank[minTier]
	if !ok1 || !ok2 {
		return false
	}
	return r <= m // lower rank = higher confidence
}
