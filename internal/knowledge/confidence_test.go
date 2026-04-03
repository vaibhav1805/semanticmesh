package knowledge

import (
	"testing"
)

// ── ScoreToTier boundary tests ───────────────────────────────────────────────

func TestScoreToTier_Explicit(t *testing.T) {
	if tier := ScoreToTier(1.0); tier != TierExplicit {
		t.Errorf("ScoreToTier(1.0) = %q, want %q", tier, TierExplicit)
	}
	if tier := ScoreToTier(0.95); tier != TierExplicit {
		t.Errorf("ScoreToTier(0.95) = %q, want %q", tier, TierExplicit)
	}
}

func TestScoreToTier_StrongInference(t *testing.T) {
	if tier := ScoreToTier(0.94); tier != TierStrongInference {
		t.Errorf("ScoreToTier(0.94) = %q, want %q", tier, TierStrongInference)
	}
	if tier := ScoreToTier(0.8); tier != TierStrongInference {
		t.Errorf("ScoreToTier(0.8) = %q, want %q", tier, TierStrongInference)
	}
	if tier := ScoreToTier(0.75); tier != TierStrongInference {
		t.Errorf("ScoreToTier(0.75) = %q, want %q", tier, TierStrongInference)
	}
}

func TestScoreToTier_Moderate(t *testing.T) {
	if tier := ScoreToTier(0.74); tier != TierModerate {
		t.Errorf("ScoreToTier(0.74) = %q, want %q", tier, TierModerate)
	}
	if tier := ScoreToTier(0.6); tier != TierModerate {
		t.Errorf("ScoreToTier(0.6) = %q, want %q", tier, TierModerate)
	}
	if tier := ScoreToTier(0.55); tier != TierModerate {
		t.Errorf("ScoreToTier(0.55) = %q, want %q", tier, TierModerate)
	}
}

func TestScoreToTier_Weak(t *testing.T) {
	if tier := ScoreToTier(0.54); tier != TierWeak {
		t.Errorf("ScoreToTier(0.54) = %q, want %q", tier, TierWeak)
	}
	if tier := ScoreToTier(0.5); tier != TierWeak {
		t.Errorf("ScoreToTier(0.5) = %q, want %q", tier, TierWeak)
	}
	if tier := ScoreToTier(0.45); tier != TierWeak {
		t.Errorf("ScoreToTier(0.45) = %q, want %q", tier, TierWeak)
	}
}

func TestScoreToTier_Semantic(t *testing.T) {
	if tier := ScoreToTier(0.44); tier != TierSemantic {
		t.Errorf("ScoreToTier(0.44) = %q, want %q", tier, TierSemantic)
	}
	if tier := ScoreToTier(0.42); tier != TierSemantic {
		t.Errorf("ScoreToTier(0.42) = %q, want %q", tier, TierSemantic)
	}
}

func TestScoreToTier_Threshold(t *testing.T) {
	if tier := ScoreToTier(0.41); tier != TierThreshold {
		t.Errorf("ScoreToTier(0.41) = %q, want %q", tier, TierThreshold)
	}
	if tier := ScoreToTier(0.4); tier != TierThreshold {
		t.Errorf("ScoreToTier(0.4) = %q, want %q", tier, TierThreshold)
	}
}

func TestScoreToTier_BelowMinimum_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for score 0.39, got none")
		}
	}()
	ScoreToTier(0.39)
}

func TestScoreToTier_Negative_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for negative score, got none")
		}
	}()
	ScoreToTier(-0.1)
}

// ── TierToScore tests ────────────────────────────────────────────────────────

func TestTierToScore_AllTiers(t *testing.T) {
	cases := []struct {
		tier  ConfidenceTier
		score float64
	}{
		{TierExplicit, 1.0},
		{TierStrongInference, 0.8},
		{TierModerate, 0.6},
		{TierWeak, 0.5},
		{TierSemantic, 0.45},
		{TierThreshold, 0.4},
	}
	for _, tc := range cases {
		score, err := TierToScore(tc.tier)
		if err != nil {
			t.Errorf("TierToScore(%q) unexpected error: %v", tc.tier, err)
		}
		if score != tc.score {
			t.Errorf("TierToScore(%q) = %.2f, want %.2f", tc.tier, score, tc.score)
		}
	}
}

func TestTierToScore_UnknownTier_Error(t *testing.T) {
	_, err := TierToScore("bogus")
	if err == nil {
		t.Error("expected error for unknown tier, got nil")
	}
}

// ── Bidirectional conversion ─────────────────────────────────────────────────

func TestBidirectionalConversion(t *testing.T) {
	// For most tiers, converting tier -> score -> tier should return the original.
	// Exception: TierSemantic has canonical score 0.45, which maps to TierWeak
	// because the weak boundary (>= 0.45) encompasses it. This is by design:
	// the canonical score is representative, but boundary mapping is the authority.
	expected := map[ConfidenceTier]ConfidenceTier{
		TierExplicit:        TierExplicit,
		TierStrongInference: TierStrongInference,
		TierModerate:        TierModerate,
		TierWeak:            TierWeak,
		TierSemantic:        TierWeak, // 0.45 >= 0.45 -> weak
		TierThreshold:       TierThreshold,
	}
	for _, tier := range AllConfidenceTiers() {
		score, err := TierToScore(tier)
		if err != nil {
			t.Fatalf("TierToScore(%q): %v", tier, err)
		}
		roundTrip := ScoreToTier(score)
		want := expected[tier]
		if roundTrip != want {
			t.Errorf("roundtrip for %q: score=%.2f -> tier=%q (want %q)",
				tier, score, roundTrip, want)
		}
	}
}

// ── IsValidConfidenceScore ───────────────────────────────────────────────────

func TestIsValidConfidenceScore(t *testing.T) {
	valid := []float64{0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 0.42, 0.45, 0.55}
	for _, s := range valid {
		if !IsValidConfidenceScore(s) {
			t.Errorf("IsValidConfidenceScore(%.2f) = false, want true", s)
		}
	}

	invalid := []float64{0.0, 0.39, -0.1, 1.01, 2.0}
	for _, s := range invalid {
		if IsValidConfidenceScore(s) {
			t.Errorf("IsValidConfidenceScore(%.2f) = true, want false", s)
		}
	}
}

// ── AllConfidenceTiers ───────────────────────────────────────────────────────

func TestAllConfidenceTiers_Count(t *testing.T) {
	tiers := AllConfidenceTiers()
	if len(tiers) != 6 {
		t.Errorf("AllConfidenceTiers() returned %d tiers, want 6", len(tiers))
	}
}

func TestAllConfidenceTiers_Unique(t *testing.T) {
	seen := make(map[ConfidenceTier]bool)
	for _, tier := range AllConfidenceTiers() {
		if seen[tier] {
			t.Errorf("duplicate tier: %q", tier)
		}
		seen[tier] = true
	}
}

func TestAllConfidenceTiers_IsCopy(t *testing.T) {
	a := AllConfidenceTiers()
	b := AllConfidenceTiers()
	a[0] = "mutated"
	if b[0] == "mutated" {
		t.Error("AllConfidenceTiers() does not return a defensive copy")
	}
}

// ── TierDisplayName ──────────────────────────────────────────────────────────

func TestTierDisplayName(t *testing.T) {
	name := TierDisplayName(TierExplicit)
	if name != "explicit (1.00)" {
		t.Errorf("TierDisplayName(TierExplicit) = %q, want %q", name, "explicit (1.00)")
	}
	name = TierDisplayName(TierThreshold)
	if name != "threshold (0.40)" {
		t.Errorf("TierDisplayName(TierThreshold) = %q, want %q", name, "threshold (0.40)")
	}
}

func TestTierDisplayName_UnknownTier(t *testing.T) {
	name := TierDisplayName("bogus")
	if name != "bogus" {
		t.Errorf("TierDisplayName(bogus) = %q, want %q", name, "bogus")
	}
}

// ── TierAtLeast ──────────────────────────────────────────────────────────────

func TestTierAtLeast(t *testing.T) {
	cases := []struct {
		tier, minTier ConfidenceTier
		want          bool
	}{
		{TierExplicit, TierExplicit, true},
		{TierExplicit, TierThreshold, true},
		{TierModerate, TierWeak, true},
		{TierModerate, TierModerate, true},
		{TierWeak, TierModerate, false},
		{TierThreshold, TierExplicit, false},
		{TierThreshold, TierThreshold, true},
	}
	for _, tc := range cases {
		got := TierAtLeast(tc.tier, tc.minTier)
		if got != tc.want {
			t.Errorf("TierAtLeast(%q, %q) = %v, want %v",
				tc.tier, tc.minTier, got, tc.want)
		}
	}
}

func TestTierAtLeast_UnknownTier(t *testing.T) {
	if TierAtLeast("bogus", TierModerate) {
		t.Error("TierAtLeast with unknown tier should return false")
	}
	if TierAtLeast(TierModerate, "bogus") {
		t.Error("TierAtLeast with unknown minTier should return false")
	}
}

// ── TierOrdering ─────────────────────────────────────────────────────────────

func TestTierOrdering_Descending(t *testing.T) {
	tiers := AllConfidenceTiers()
	for i := 0; i < len(tiers)-1; i++ {
		if !TierAtLeast(tiers[i], tiers[i+1]) {
			t.Errorf("tier[%d]=%q should be >= tier[%d]=%q",
				i, tiers[i], i+1, tiers[i+1])
		}
	}
}
