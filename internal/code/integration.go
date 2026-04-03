package code

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// RunCodeAnalysis runs the full code analysis pipeline on a directory:
// infer the source component, register the provided language parsers, and analyze all files.
// Callers provide parser instances to avoid import cycles within this package.
func RunCodeAnalysis(dir string, parsers ...LanguageParser) ([]CodeSignal, error) {
	sourceComponent := InferSourceComponent(dir)

	analyzer := NewCodeAnalyzer(sourceComponent)
	for _, p := range parsers {
		analyzer.RegisterParser(p)
	}

	signals, err := analyzer.AnalyzeDir(dir)
	if err != nil {
		return nil, fmt.Errorf("code analysis: %w", err)
	}

	// Set SourceFile to relative paths for cleaner output
	for i := range signals {
		if rel, relErr := filepath.Rel(dir, signals[i].SourceFile); relErr == nil {
			signals[i].SourceFile = rel
		}
	}

	return signals, nil
}

// PrintCodeSignalsSummary prints a concise summary of code analysis results to w.
// This is diagnostic output intended for stderr.
func PrintCodeSignalsSummary(w io.Writer, signals []CodeSignal) {
	if len(signals) == 0 {
		fmt.Fprintf(w, "  Code analysis: no signals detected\n")
		return
	}

	// Count by detection_kind
	kindCounts := make(map[string]int)
	for _, s := range signals {
		kindCounts[s.DetectionKind]++
	}

	// Sort kinds for deterministic output
	kinds := make([]string, 0, len(kindCounts))
	for k := range kindCounts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	// Format kind summary
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		label := strings.ReplaceAll(k, "_", " ")
		parts = append(parts, fmt.Sprintf("%d %s", kindCounts[k], label))
	}

	fmt.Fprintf(w, "  Code signals: %s\n", strings.Join(parts, ", "))
}
