package mendixparser

import (
	"testing"
)

func TestScanWorkspace(t *testing.T) {
	// Test scanning for multiple projects
	// This is a placeholder - needs actual test data
	t.Skip("Requires test workspace with multiple .mpr files")
}

func TestDetectInterAppDependencies(t *testing.T) {
	// Test inter-app dependency detection
	parser := New("")

	analyses := []ProjectAnalysis{
		{
			AppName: "FrontendApp",
			MprPath: "/workspace/frontend.mpr",
			Dependencies: []DependencySignal{
				{
					SourceName: "FrontendApp",
					TargetName: "BackendApp",
					TargetType: "service",
					Kind:       "rest_api_call",
				},
			},
		},
		{
			AppName: "BackendApp",
			MprPath: "/workspace/backend.mpr",
			Dependencies: []DependencySignal{},
		},
	}

	interAppDeps := parser.detectInterAppDependencies(analyses)

	if len(interAppDeps) == 0 {
		t.Error("Expected to find inter-app dependencies")
	}

	found := false
	for _, dep := range interAppDeps {
		if dep.SourceName == "FrontendApp" && dep.TargetName == "BackendApp" {
			found = true
			if dep.Kind != "inter_app_dependency" {
				t.Errorf("Expected kind 'inter_app_dependency', got %s", dep.Kind)
			}
		}
	}

	if !found {
		t.Error("Expected to find FrontendApp -> BackendApp dependency")
	}
}

func TestDetectSharedDatabases(t *testing.T) {
	analyses := []ProjectAnalysis{
		{
			AppName: "App1",
			Dependencies: []DependencySignal{
				{TargetName: "postgres-db", TargetType: "database"},
			},
		},
		{
			AppName: "App2",
			Dependencies: []DependencySignal{
				{TargetName: "postgres-db", TargetType: "database"},
			},
		},
		{
			AppName: "App3",
			Dependencies: []DependencySignal{
				{TargetName: "redis-cache", TargetType: "cache"},
			},
		},
	}

	signals := detectSharedDatabases(analyses)

	// Should find postgres-db used by both App1 and App2
	if len(signals) != 2 { // One signal per app using shared DB
		t.Errorf("Expected 2 signals, got %d", len(signals))
	}

	for _, sig := range signals {
		if sig.TargetName != "postgres-db" {
			t.Errorf("Expected target 'postgres-db', got %s", sig.TargetName)
		}
		if sig.Kind != "shared_database" {
			t.Errorf("Expected kind 'shared_database', got %s", sig.Kind)
		}
	}
}
