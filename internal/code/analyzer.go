package code

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/mod/modfile"
)

// LanguageParser defines the interface for language-specific code analyzers.
// Each parser detects infrastructure dependency signals from source files
// written in a particular language.
type LanguageParser interface {
	// Name returns the parser's language name (e.g., "go", "python", "javascript").
	Name() string

	// Extensions returns the file extensions this parser handles (e.g., [".go"]).
	Extensions() []string

	// ParseFile analyzes the given file content and returns detected signals.
	// Returns nil, nil if the file should be skipped (e.g., test files).
	ParseFile(filePath string, content []byte) ([]CodeSignal, error)
}

// ManifestAnalyzer is an optional interface that LanguageParsers can implement
// to analyze project-level manifest files (go.mod, package.json, requirements.txt)
// for infrastructure dependencies.
type ManifestAnalyzer interface {
	AnalyzeManifests(dir string) ([]CodeSignal, error)
}

// CodeAnalyzer orchestrates code analysis by dispatching files to the appropriate
// language parser based on file extension.
type CodeAnalyzer struct {
	parsers         map[string]LanguageParser // extension -> parser
	sourceComponent string
}

// NewCodeAnalyzer creates a CodeAnalyzer. sourceComponent is the name of the
// component whose source code is being analyzed (used as context, not embedded
// in signals directly).
func NewCodeAnalyzer(sourceComponent string) *CodeAnalyzer {
	return &CodeAnalyzer{
		parsers:         make(map[string]LanguageParser),
		sourceComponent: sourceComponent,
	}
}

// RegisterParser registers a LanguageParser for each of its declared file extensions.
func (a *CodeAnalyzer) RegisterParser(p LanguageParser) {
	for _, ext := range p.Extensions() {
		a.parsers[ext] = p
	}
}

// AnalyzeFile dispatches a single file to the appropriate parser by extension.
// Sets SourceFile on all returned signals.
func (a *CodeAnalyzer) AnalyzeFile(filePath string, content []byte) ([]CodeSignal, error) {
	ext := filepath.Ext(filePath)
	parser, ok := a.parsers[ext]
	if !ok {
		return nil, nil // no parser for this extension
	}

	signals, err := parser.ParseFile(filePath, content)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filePath, err)
	}

	for i := range signals {
		signals[i].SourceFile = filePath
	}
	return signals, nil
}

// skipDirs contains directory names that should be excluded from analysis.
var skipDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	".git":         true,
	"__tests__":    true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	".tox":         true,
	"dist":         true,
	"build":        true,
}

// AnalyzeDir walks a directory tree, analyzing each file with a registered parser.
// Skips test files (*_test.go), vendor/, and node_modules/ directories.
func (a *CodeAnalyzer) AnalyzeDir(dir string) ([]CodeSignal, error) {
	var allSignals []CodeSignal

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip test files
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		ext := filepath.Ext(path)
		if _, ok := a.parsers[ext]; !ok {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		signals, err := a.AnalyzeFile(path, content)
		if err != nil {
			return err
		}

		allSignals = append(allSignals, signals...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", dir, err)
	}
	return allSignals, nil
}

// pyprojectNameRe matches the name field in pyproject.toml [project] section.
var pyprojectNameRe = regexp.MustCompile(`(?m)^name\s*=\s*"([^"]+)"`)

// setupPyNameRe matches the name argument in setup.py.
var setupPyNameRe = regexp.MustCompile(`name\s*=\s*["']([^"']+)["']`)

// InferSourceComponent walks up from dir to find a project manifest and extracts
// the project name. Checks go.mod, pyproject.toml, setup.py, and package.json
// in order at each directory level. Falls back to filepath.Base(dir) if no
// manifest is found.
func InferSourceComponent(dir string) string {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Base(dir)
	}

	current := absDir
	for {
		// Check go.mod
		if data, err := os.ReadFile(filepath.Join(current, "go.mod")); err == nil {
			if modulePath := modfile.ModulePath(data); modulePath != "" {
				return modulePath
			}
		}

		// Check pyproject.toml
		if data, err := os.ReadFile(filepath.Join(current, "pyproject.toml")); err == nil {
			if m := pyprojectNameRe.FindSubmatch(data); len(m) > 1 {
				return string(m[1])
			}
		}

		// Check setup.py
		if data, err := os.ReadFile(filepath.Join(current, "setup.py")); err == nil {
			if m := setupPyNameRe.FindSubmatch(data); len(m) > 1 {
				return string(m[1])
			}
		}

		// Check package.json
		if data, err := os.ReadFile(filepath.Join(current, "package.json")); err == nil {
			var pkg struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(data, &pkg) == nil && pkg.Name != "" {
				return pkg.Name
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break // reached filesystem root
		}
		current = parent
	}

	return filepath.Base(dir)
}
