package mendixparser

// ExtractionProfile defines the level of detail for extraction
type ExtractionProfile string

const (
	ProfileMinimal       ExtractionProfile = "minimal"       // Tier 1 only (~20ms)
	ProfileStandard      ExtractionProfile = "standard"      // Tier 1 + Tier 2 (~100ms)
	ProfileComprehensive ExtractionProfile = "comprehensive" // All tiers (~300ms)
)

// Config holds configuration for Mendix parser
type Config struct {
	// MxcliPath is deprecated - mxcli is now used as a Go library, not an external binary
	// Kept for backward compatibility but no longer used
	MxcliPath string

	// RefreshCatalog controls whether to rebuild catalog before analysis
	RefreshCatalog bool

	// IncludeInternalDeps includes module-to-module and microflow dependencies
	IncludeInternalDeps bool

	// DetectModulesAsComponents creates separate component nodes for each module
	DetectModulesAsComponents bool

	// === NEW: Profile Selection ===
	ExtractionProfile ExtractionProfile

	// === NEW: Fine-grained Control ===
	ExtractPublishedAPIs  bool
	ExtractDomainModel    bool
	ExtractBusinessLogic  bool
	ExtractUIStructure    bool
	ExtractConfiguration  bool
}

// DefaultConfig returns default configuration (now delegates to StandardConfig)
func DefaultConfig() *Config {
	cfg := StandardConfig()
	cfg.MxcliPath = "mxcli" // Set default for backward compatibility
	return cfg
}

// MinimalConfig returns configuration for Tier 1 only
func MinimalConfig() *Config {
	return &Config{
		RefreshCatalog:        true,
		ExtractionProfile:     ProfileMinimal,
		ExtractPublishedAPIs:  true,
		ExtractDomainModel:    true, // entities are Tier 1
		ExtractBusinessLogic:  false,
		ExtractUIStructure:    false,
		ExtractConfiguration:  false,
	}
}

// StandardConfig returns configuration for Tier 1 + Tier 2 (recommended default)
func StandardConfig() *Config {
	return &Config{
		RefreshCatalog:        true,
		ExtractionProfile:     ProfileStandard,
		ExtractPublishedAPIs:  true,
		ExtractDomainModel:    true,
		ExtractBusinessLogic:  true,
		ExtractUIStructure:    true,
		ExtractConfiguration:  true,
	}
}

// ComprehensiveConfig returns configuration for all tiers
func ComprehensiveConfig() *Config {
	return &Config{
		RefreshCatalog:        true,
		ExtractionProfile:     ProfileComprehensive,
		ExtractPublishedAPIs:  true,
		ExtractDomainModel:    true,
		ExtractBusinessLogic:  true,
		ExtractUIStructure:    true,
		ExtractConfiguration:  true,
		IncludeInternalDeps:   true,
	}
}
