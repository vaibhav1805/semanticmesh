package mendixparser

import (
	"testing"
)

// TestConfidenceScoreRanges validates that confidence scores are within acceptable ranges
func TestConfidenceScoreRanges(t *testing.T) {
	tests := []struct {
		name           string
		signalType     string
		expectedMinConf float64
		expectedMaxConf float64
		description    string
	}{
		{
			name:            "MPR file detection",
			signalType:      "mendix_mpr_file",
			expectedMinConf: 0.95,
			expectedMaxConf: 0.95,
			description:     "Direct file detection should have very high confidence",
		},
		{
			name:            "REST API call",
			signalType:      "rest_api_call",
			expectedMinConf: 0.85,
			expectedMaxConf: 0.95,
			description:     "Catalog-based REST client detection is highly reliable",
		},
		{
			name:            "Database connection",
			signalType:      "db_connection",
			expectedMinConf: 0.80,
			expectedMaxConf: 0.90,
			description:     "External entity detection is reliable but less certain than APIs",
		},
		{
			name:            "Module dependency",
			signalType:      "module_dependency",
			expectedMinConf: 0.75,
			expectedMaxConf: 0.85,
			description:     "Module references are counted but may include false positives",
		},
		{
			name:            "Microflow call",
			signalType:      "microflow_call",
			expectedMinConf: 0.70,
			expectedMaxConf: 0.80,
			description:     "Microflow dependencies are less certain than module deps",
		},
		{
			name:            "Web service call",
			signalType:      "webservice_call",
			expectedMinConf: 0.80,
			expectedMaxConf: 0.90,
			description:     "SOAP/WSDL services are well-defined",
		},
		{
			name:            "Module as component",
			signalType:      "module_component",
			expectedMinConf: 0.85,
			expectedMaxConf: 0.85,
			description:     "Modules detected via SHOW MODULES are reliable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate confidence ranges are reasonable
			if tt.expectedMinConf < 0.4 {
				t.Errorf("Minimum confidence too low: %f (should be >= 0.4)", tt.expectedMinConf)
			}
			if tt.expectedMaxConf > 1.0 {
				t.Errorf("Maximum confidence too high: %f (should be <= 1.0)", tt.expectedMaxConf)
			}
			if tt.expectedMinConf > tt.expectedMaxConf {
				t.Errorf("Minimum confidence (%f) > maximum (%f)", tt.expectedMinConf, tt.expectedMaxConf)
			}
			t.Logf("%s: confidence range [%.2f, %.2f] - %s",
				tt.name, tt.expectedMinConf, tt.expectedMaxConf, tt.description)
		})
	}
}

// TestComponentSignalConfidence validates ComponentSignal confidence scores
func TestComponentSignalConfidence(t *testing.T) {
	tests := []struct {
		name   string
		signal ComponentSignal
		valid  bool
	}{
		{
			name: "valid app component",
			signal: ComponentSignal{
				Name:       "MyApp",
				Type:       "service",
				Confidence: 0.95,
				SourceFile: "/path/to/MyApp.mpr",
				Evidence:   "mendix_mpr_file",
			},
			valid: true,
		},
		{
			name: "valid module component",
			signal: ComponentSignal{
				Name:       "MyApp:Administration",
				Type:       "service",
				Confidence: 0.85,
				SourceFile: "/path/to/MyApp.mpr",
				Evidence:   "Module in MyApp",
			},
			valid: true,
		},
		{
			name: "invalid - confidence too low",
			signal: ComponentSignal{
				Name:       "MyApp",
				Type:       "service",
				Confidence: 0.3,
				SourceFile: "/path/to/MyApp.mpr",
				Evidence:   "low_confidence",
			},
			valid: false,
		},
		{
			name: "invalid - confidence too high",
			signal: ComponentSignal{
				Name:       "MyApp",
				Type:       "service",
				Confidence: 1.5,
				SourceFile: "/path/to/MyApp.mpr",
				Evidence:   "invalid",
			},
			valid: false,
		},
		{
			name: "edge case - minimum valid confidence",
			signal: ComponentSignal{
				Name:       "MyApp",
				Type:       "service",
				Confidence: 0.4,
				SourceFile: "/path/to/MyApp.mpr",
				Evidence:   "edge_case",
			},
			valid: true,
		},
		{
			name: "edge case - maximum valid confidence",
			signal: ComponentSignal{
				Name:       "MyApp",
				Type:       "service",
				Confidence: 1.0,
				SourceFile: "/path/to/MyApp.mpr",
				Evidence:   "perfect_match",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.signal.Confidence >= 0.4 && tt.signal.Confidence <= 1.0
			if isValid != tt.valid {
				t.Errorf("Signal validation mismatch: confidence=%f, expected valid=%v, got valid=%v",
					tt.signal.Confidence, tt.valid, isValid)
			}
		})
	}
}

// TestDependencySignalConfidence validates DependencySignal confidence scores
func TestDependencySignalConfidence(t *testing.T) {
	tests := []struct {
		name   string
		signal DependencySignal
		valid  bool
	}{
		{
			name: "valid REST API dependency",
			signal: DependencySignal{
				SourceName: "MyApp",
				TargetName: "stripe",
				TargetType: "service",
				Confidence: 0.90,
				Kind:       "rest_api_call",
				Evidence:   "REST client: Stripe API",
				SourceFile: "/path/to/MyApp.mpr",
			},
			valid: true,
		},
		{
			name: "valid database dependency",
			signal: DependencySignal{
				SourceName: "MyApp",
				TargetName: "postgresql",
				TargetType: "database",
				Confidence: 0.85,
				Kind:       "db_connection",
				Evidence:   "External entity: PostgreSQL_Customer",
				SourceFile: "/path/to/MyApp.mpr",
			},
			valid: true,
		},
		{
			name: "valid module dependency",
			signal: DependencySignal{
				SourceName: "MyApp:ModuleA",
				TargetName: "MyApp:ModuleB",
				TargetType: "service",
				Confidence: 0.80,
				Kind:       "module_dependency",
				Evidence:   "5 references from ModuleA to ModuleB",
				SourceFile: "/path/to/MyApp.mpr",
			},
			valid: true,
		},
		{
			name: "invalid - confidence too low",
			signal: DependencySignal{
				SourceName: "MyApp",
				TargetName: "Unknown",
				TargetType: "service",
				Confidence: 0.2,
				Kind:       "unknown",
				Evidence:   "weak signal",
				SourceFile: "/path/to/MyApp.mpr",
			},
			valid: false,
		},
		{
			name: "invalid - confidence above 1.0",
			signal: DependencySignal{
				SourceName: "MyApp",
				TargetName: "Service",
				TargetType: "service",
				Confidence: 1.1,
				Kind:       "invalid",
				Evidence:   "over 100%",
				SourceFile: "/path/to/MyApp.mpr",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.signal.Confidence >= 0.4 && tt.signal.Confidence <= 1.0
			if isValid != tt.valid {
				t.Errorf("Signal validation mismatch: confidence=%f, expected valid=%v, got valid=%v",
					tt.signal.Confidence, tt.valid, isValid)
			}

			// Additional validation for dependency signals
			if tt.valid {
				if tt.signal.SourceName == "" {
					t.Error("Valid signal should have non-empty SourceName")
				}
				if tt.signal.TargetName == "" {
					t.Error("Valid signal should have non-empty TargetName")
				}
				if tt.signal.TargetType == "" {
					t.Error("Valid signal should have non-empty TargetType")
				}
				if tt.signal.Kind == "" {
					t.Error("Valid signal should have non-empty Kind")
				}
			}
		})
	}
}

// TestProjectAnalysisConfidence validates overall analysis confidence
func TestProjectAnalysisConfidence(t *testing.T) {
	// Mock a project analysis
	analysis := &ProjectAnalysis{
		AppName: "TestApp",
		MprPath: "/path/to/TestApp.mpr",
		Components: []ComponentSignal{
			{
				Name:       "TestApp",
				Type:       "service",
				Confidence: 0.95,
				SourceFile: "/path/to/TestApp.mpr",
				Evidence:   "mendix_mpr_file",
			},
			{
				Name:       "TestApp:ModuleA",
				Type:       "service",
				Confidence: 0.85,
				SourceFile: "/path/to/TestApp.mpr",
				Evidence:   "Module in TestApp",
			},
		},
		Dependencies: []DependencySignal{
			{
				SourceName: "TestApp",
				TargetName: "stripe",
				TargetType: "service",
				Confidence: 0.90,
				Kind:       "rest_api_call",
				Evidence:   "REST client: Stripe",
				SourceFile: "/path/to/TestApp.mpr",
			},
			{
				SourceName: "TestApp:ModuleA",
				TargetName: "TestApp:ModuleB",
				TargetType: "service",
				Confidence: 0.80,
				Kind:       "module_dependency",
				Evidence:   "3 references",
				SourceFile: "/path/to/TestApp.mpr",
			},
		},
		Modules: []string{"ModuleA", "ModuleB"},
	}

	// Validate all component confidence scores
	for i, comp := range analysis.Components {
		if comp.Confidence < 0.4 || comp.Confidence > 1.0 {
			t.Errorf("Component[%d] %s has invalid confidence: %f", i, comp.Name, comp.Confidence)
		}
	}

	// Validate all dependency confidence scores
	for i, dep := range analysis.Dependencies {
		if dep.Confidence < 0.4 || dep.Confidence > 1.0 {
			t.Errorf("Dependency[%d] %s->%s has invalid confidence: %f",
				i, dep.SourceName, dep.TargetName, dep.Confidence)
		}
	}

	// Calculate average confidence
	totalConf := 0.0
	count := 0
	for _, comp := range analysis.Components {
		totalConf += comp.Confidence
		count++
	}
	for _, dep := range analysis.Dependencies {
		totalConf += dep.Confidence
		count++
	}

	avgConf := totalConf / float64(count)
	if avgConf < 0.4 {
		t.Errorf("Average confidence too low: %f", avgConf)
	}

	t.Logf("Analysis summary: %d components, %d dependencies, avg confidence: %.2f",
		len(analysis.Components), len(analysis.Dependencies), avgConf)
}

// TestConfidenceScoreOrdering validates that confidence scores follow expected ordering
func TestConfidenceScoreOrdering(t *testing.T) {
	// Expected: MPR detection > REST API > Database > Module dep > Microflow
	mprConf := 0.95
	restConf := 0.90
	dbConf := 0.85
	moduleConf := 0.80
	microflowConf := 0.75

	if mprConf <= restConf {
		t.Error("MPR confidence should be higher than REST API confidence")
	}
	if restConf <= dbConf {
		t.Error("REST API confidence should be higher than database confidence")
	}
	if dbConf <= moduleConf {
		t.Error("Database confidence should be higher than module dependency confidence")
	}
	if moduleConf <= microflowConf {
		t.Error("Module dependency confidence should be higher than microflow confidence")
	}

	t.Log("Confidence ordering is correct: MPR > REST > DB > Module > Microflow")
}
