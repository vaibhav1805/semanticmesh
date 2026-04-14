package mendixparser

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewTypesCompilation(t *testing.T) {
	// Test new structs compile correctly
	api := PublishedAPIInfo{
		Name:           "TestAPI",
		Type:           "rest",
		Path:           "/api/v1",
		Version:        "1.0",
		ModuleName:     "TestModule",
		OperationCount: 2,
		Operations: []APIOperation{
			{
				ResourceName: "users",
				HttpMethod:   "GET",
				Path:         "/users",
				Microflow:    "GetUsers",
				Summary:      "Get all users",
			},
		},
	}

	entity := EntityInfo{
		Name:           "User",
		QualifiedName:  "TestModule.User",
		ModuleName:     "TestModule",
		EntityType:     "Persistent",
		IsExternal:     false,
		AttributeCount: 5,
	}

	microflow := MicroflowInfo{
		Name:          "GetUsers",
		QualifiedName: "TestModule.GetUsers",
		ModuleName:    "TestModule",
		Type:          "Microflow",
		ActivityCount: 10,
		Complexity:    3,
		IsScheduled:   false,
	}

	page := PageInfo{
		Name:          "UserOverview",
		QualifiedName: "TestModule.UserOverview",
		ModuleName:    "TestModule",
		URL:           "/users",
		WidgetCount:   15,
	}

	javaAction := JavaActionInfo{
		Name:           "HashPassword",
		QualifiedName:  "TestModule.HashPassword",
		ModuleName:     "TestModule",
		ReturnType:     "String",
		ParameterCount: 1,
		ExportLevel:    "Public",
	}

	constant := ConstantInfo{
		Name:            "API_URL",
		QualifiedName:   "TestModule.API_URL",
		ModuleName:      "TestModule",
		DataType:        "String",
		DefaultValue:    "https://api.example.com",
		ExposedToClient: false,
	}

	// Test extended ProjectAnalysis
	analysis := ProjectAnalysis{
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
		},
		Dependencies:      []DependencySignal{},
		Modules:           []string{"TestModule"},
		PublishedAPIs:     []PublishedAPIInfo{api},
		Entities:          []EntityInfo{entity},
		Microflows:        []MicroflowInfo{microflow},
		JavaActions:       []JavaActionInfo{javaAction},
		Pages:             []PageInfo{page},
		Constants:         []ConstantInfo{constant},
		ExtractionProfile: "standard",
		ExtractionTime:    time.Now(),
		TablesExtracted:   []string{"published_rest_services", "entities", "microflows"},
	}

	// Test JSON serialization with omitempty
	jsonData, err := json.Marshal(analysis)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("Expected non-empty JSON output")
	}

	// Verify counts
	if len(analysis.PublishedAPIs) != 1 {
		t.Errorf("Expected 1 published API, got %d", len(analysis.PublishedAPIs))
	}
	if len(analysis.Entities) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(analysis.Entities))
	}
	if len(analysis.Microflows) != 1 {
		t.Errorf("Expected 1 microflow, got %d", len(analysis.Microflows))
	}
}

func TestConfigProfiles(t *testing.T) {
	// Test Minimal profile
	minimal := MinimalConfig()
	if minimal.ExtractionProfile != ProfileMinimal {
		t.Errorf("Expected minimal profile, got %s", minimal.ExtractionProfile)
	}
	if !minimal.ExtractPublishedAPIs {
		t.Error("Minimal should extract published APIs")
	}
	if minimal.ExtractBusinessLogic {
		t.Error("Minimal should not extract business logic")
	}

	// Test Standard profile
	standard := StandardConfig()
	if standard.ExtractionProfile != ProfileStandard {
		t.Errorf("Expected standard profile, got %s", standard.ExtractionProfile)
	}
	if !standard.ExtractPublishedAPIs {
		t.Error("Standard should extract published APIs")
	}
	if !standard.ExtractBusinessLogic {
		t.Error("Standard should extract business logic")
	}

	// Test Comprehensive profile
	comprehensive := ComprehensiveConfig()
	if comprehensive.ExtractionProfile != ProfileComprehensive {
		t.Errorf("Expected comprehensive profile, got %s", comprehensive.ExtractionProfile)
	}
	if !comprehensive.ExtractPublishedAPIs {
		t.Error("Comprehensive should extract published APIs")
	}
	if !comprehensive.IncludeInternalDeps {
		t.Error("Comprehensive should include internal deps")
	}

	// Test DefaultConfig delegates to StandardConfig
	defaultCfg := DefaultConfig()
	if defaultCfg.ExtractionProfile != ProfileStandard {
		t.Errorf("DefaultConfig should use standard profile, got %s", defaultCfg.ExtractionProfile)
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that old code still works - creating ProjectAnalysis with only original fields
	analysis := ProjectAnalysis{
		AppName: "LegacyApp",
		MprPath: "/path/to/legacy.mpr",
		Components: []ComponentSignal{
			{
				Name:       "LegacyApp",
				Type:       "service",
				Confidence: 0.9,
				SourceFile: "/path/to/legacy.mpr",
				Evidence:   "test",
			},
		},
		Dependencies: []DependencySignal{},
		Modules:      []string{"Module1"},
	}

	// Should serialize without new fields (omitempty)
	jsonData, err := json.Marshal(analysis)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify it doesn't include empty arrays for new fields
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// New fields with omitempty should not appear in JSON when empty
	if _, exists := result["published_apis"]; exists {
		t.Error("Empty published_apis should not appear in JSON due to omitempty")
	}
	if _, exists := result["extraction_profile"]; exists {
		t.Error("Empty extraction_profile should not appear in JSON due to omitempty")
	}
}
