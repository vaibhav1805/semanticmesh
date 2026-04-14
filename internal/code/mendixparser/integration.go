package mendixparser

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vaibhav1805/semanticmesh/internal/code"
)

// MendixParser implements code.LanguageParser but operates at the project level.
// It detects Mendix projects during directory walking and analyzes them as a whole.
type MendixParser struct {
	parser          *Parser
	analyzedDirs    map[string]bool // Track which dirs we've already analyzed
	pendingSignals  []code.CodeSignal
	pendingAnalysis *ProjectAnalysis
}

// NewMendixParser creates a new Mendix parser that implements LanguageParser
func NewMendixParser(mxcliPath string) *MendixParser {
	return &MendixParser{
		parser:       New(mxcliPath),
		analyzedDirs: make(map[string]bool),
	}
}

// Name returns "mendix"
func (mp *MendixParser) Name() string { return "mendix" }

// Extensions returns [".mpr"] to trigger on Mendix project files
func (mp *MendixParser) Extensions() []string { return []string{".mpr"} }

// ParseFile is called when the analyzer encounters an .mpr file.
// We analyze the entire Mendix project and convert signals to CodeSignal format.
func (mp *MendixParser) ParseFile(filePath string, content []byte) ([]code.CodeSignal, error) {
	// Get the directory containing the .mpr file
	projectDir := filepath.Dir(filePath)

	// Skip if we've already analyzed this directory
	if mp.analyzedDirs[projectDir] {
		return nil, nil
	}
	mp.analyzedDirs[projectDir] = true

	// Analyze the Mendix project
	analysis, err := mp.parser.AnalyzeProject(filePath)
	if err != nil {
		// If mxcli is not available, silently skip (user hasn't installed it)
		if isNotAvailableError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("mendix analysis: %w", err)
	}

	// Convert ComponentSignals and DependencySignals to CodeSignals
	var signals []code.CodeSignal

	// Convert component signals to CodeSignals (for Mendix app itself)
	for _, comp := range analysis.Components {
		signals = append(signals, code.CodeSignal{
			SourceFile:      comp.SourceFile,
			LineNumber:      1,
			TargetComponent: comp.Name,
			TargetType:      comp.Type,
			DetectionKind:   "mendix_app",
			Evidence:        fmt.Sprintf("Mendix app: %s", comp.Name),
			Language:        "mendix",
			Confidence:      comp.Confidence,
		})
	}

	// Convert dependency signals to CodeSignals
	for _, dep := range analysis.Dependencies {
		signals = append(signals, code.CodeSignal{
			SourceFile:      dep.SourceFile,
			LineNumber:      1,
			TargetComponent: dep.TargetName,
			TargetType:      dep.TargetType,
			DetectionKind:   dep.Kind,
			Evidence:        dep.Evidence,
			Language:        "mendix",
			Confidence:      dep.Confidence,
		})
	}

	return signals, nil
}

// AnalyzeMendixProjects scans a directory tree for Mendix projects and returns
// detected components and dependencies as CodeSignals.
// This is an alternative entry point that doesn't rely on the LanguageParser interface.
func AnalyzeMendixProjects(rootPath string) ([]code.CodeSignal, error) {
	parser := New("")
	var allSignals []code.CodeSignal
	analyzedDirs := make(map[string]bool)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Skip if already analyzed
			if analyzedDirs[path] {
				return filepath.SkipDir
			}

			// Check if this is a Mendix project
			isMendix, mprPath, detectErr := DetectMendixProject(path)
			if detectErr != nil {
				return nil // Continue walking
			}

			if isMendix {
				analyzedDirs[path] = true

				// Analyze the project
				analysis, analyzeErr := parser.AnalyzeProject(mprPath)
				if analyzeErr != nil {
					// Skip if mxcli not available
					if isNotAvailableError(analyzeErr) {
						return filepath.SkipDir
					}
					// Log warning but continue
					fmt.Fprintf(os.Stderr, "Warning: Failed to analyze Mendix project at %s: %v\n", path, analyzeErr)
					return filepath.SkipDir
				}

				// Convert to CodeSignals
				for _, comp := range analysis.Components {
					allSignals = append(allSignals, code.CodeSignal{
						SourceFile:      comp.SourceFile,
						LineNumber:      1,
						TargetComponent: comp.Name,
						TargetType:      comp.Type,
						DetectionKind:   "mendix_app",
						Evidence:        fmt.Sprintf("Mendix app: %s", comp.Name),
						Language:        "mendix",
						Confidence:      comp.Confidence,
					})
				}

				for _, dep := range analysis.Dependencies {
					allSignals = append(allSignals, code.CodeSignal{
						SourceFile:      dep.SourceFile,
						LineNumber:      1,
						TargetComponent: dep.TargetName,
						TargetType:      dep.TargetType,
						DetectionKind:   dep.Kind,
						Evidence:        dep.Evidence,
						Language:        "mendix",
						Confidence:      dep.Confidence,
					})
				}

				return filepath.SkipDir // Don't descend into Mendix project
			}
		}

		return nil
	})

	return allSignals, err
}

// isNotAvailableError checks if the error is due to mxcli not being available
func isNotAvailableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "mxcli not found") || contains(errStr, "executable file not found")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// AnalyzeMendixWorkspace scans a workspace for Mendix projects and returns all signals
func AnalyzeMendixWorkspace(rootPath string) ([]code.CodeSignal, error) {
	parser := New("")

	analysis, err := parser.AnalyzeWorkspace(rootPath)
	if err != nil {
		return nil, err
	}

	var allSignals []code.CodeSignal

	// Convert each project's signals
	for _, projAnalysis := range analysis.Projects {
		signals := convertToCodeSignals(&projAnalysis)
		allSignals = append(allSignals, signals...)
	}

	// Add inter-app dependencies
	for _, dep := range analysis.InterAppDependencies {
		allSignals = append(allSignals, code.CodeSignal{
			SourceFile:      dep.SourceFile,
			LineNumber:      1,
			TargetComponent: dep.TargetName,
			TargetType:      dep.TargetType,
			Confidence:      dep.Confidence,
			DetectionKind:   dep.Kind,
			Evidence:        dep.Evidence,
			Language:        "mendix",
		})
	}

	return allSignals, nil
}

// convertToCodeSignals converts ProjectAnalysis to CodeSignals
func convertToCodeSignals(analysis *ProjectAnalysis) []code.CodeSignal {
	var signals []code.CodeSignal

	// Convert component signals to CodeSignals (for Mendix app itself)
	for _, comp := range analysis.Components {
		signals = append(signals, code.CodeSignal{
			SourceFile:      comp.SourceFile,
			LineNumber:      1,
			TargetComponent: comp.Name,
			TargetType:      comp.Type,
			DetectionKind:   "mendix_app",
			Evidence:        fmt.Sprintf("Mendix app: %s", comp.Name),
			Language:        "mendix",
			Confidence:      comp.Confidence,
		})
	}

	// Convert dependency signals to CodeSignals
	for _, dep := range analysis.Dependencies {
		signals = append(signals, code.CodeSignal{
			SourceFile:      dep.SourceFile,
			LineNumber:      1,
			TargetComponent: dep.TargetName,
			TargetType:      dep.TargetType,
			DetectionKind:   dep.Kind,
			Evidence:        dep.Evidence,
			Language:        "mendix",
			Confidence:      dep.Confidence,
		})
	}

	return signals
}
