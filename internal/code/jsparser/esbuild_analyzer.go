package jsparser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// ESBuildAnalyzer uses esbuild to accurately resolve JavaScript/TypeScript imports.
// This provides better accuracy than regex-based parsing for complex import scenarios.
type ESBuildAnalyzer struct {
	tempDir string
}

// NewESBuildAnalyzer creates a new esbuild-based import analyzer.
func NewESBuildAnalyzer() *ESBuildAnalyzer {
	return &ESBuildAnalyzer{}
}

// AnalyzeImports uses esbuild to extract imports from a JavaScript/TypeScript file.
// Returns a map of local name -> package name.
func (a *ESBuildAnalyzer) AnalyzeImports(filePath string, content []byte) (map[string]string, error) {
	// Create a temporary file for esbuild to analyze
	tempFile, err := os.CreateTemp("", "semanticmesh-*."+filepath.Ext(filePath))
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	tempFile.Close()

	// Run esbuild with metafile generation to get dependency information
	result := api.Build(api.BuildOptions{
		EntryPoints: []string{tempFile.Name()},
		Bundle:      true,
		Write:       false,
		Metafile:    true,
		LogLevel:    api.LogLevelSilent,
		Format:      api.FormatESModule,
		// Mark all imports as external to prevent actual bundling
		External: []string{"*"},
	})

	if len(result.Errors) > 0 {
		// If esbuild fails, fall back to regex parsing
		return nil, fmt.Errorf("esbuild analysis failed: %v", result.Errors[0].Text)
	}

	// Parse metafile to extract imports
	importMap, err := a.parseMetafile(result.Metafile)
	if err != nil {
		return nil, fmt.Errorf("parse metafile: %w", err)
	}

	return importMap, nil
}

// parseMetafile extracts import information from esbuild's metafile JSON.
func (a *ESBuildAnalyzer) parseMetafile(metafileJSON string) (map[string]string, error) {
	var metafile struct {
		Inputs map[string]struct {
			Imports []struct {
				Path string `json:"path"`
				Kind string `json:"kind"`
			} `json:"imports"`
		} `json:"inputs"`
	}

	if err := json.Unmarshal([]byte(metafileJSON), &metafile); err != nil {
		return nil, fmt.Errorf("unmarshal metafile: %w", err)
	}

	importMap := make(map[string]string)

	// Extract package names from import paths
	for _, input := range metafile.Inputs {
		for _, imp := range input.Imports {
			// Extract package name from import path
			// e.g., "node_modules/axios/index.js" -> "axios"
			// e.g., "@aws-sdk/client-s3" -> "@aws-sdk/client-s3"
			pkg := extractPackageName(imp.Path)
			if pkg != "" {
				// Map the package to itself (we don't have local names from metafile)
				// This gives us a list of all imported packages
				importMap[pkg] = pkg
			}
		}
	}

	return importMap, nil
}

// extractPackageName extracts the npm package name from a file path.
// Examples:
//   - "node_modules/axios/index.js" -> "axios"
//   - "node_modules/@aws-sdk/client-s3/dist/index.js" -> "@aws-sdk/client-s3"
//   - "../relative/path.js" -> "" (not a package)
func extractPackageName(path string) string {
	// Skip relative imports
	if strings.HasPrefix(path, ".") || strings.HasPrefix(path, "/") {
		return ""
	}

	// Check for node_modules
	if strings.Contains(path, "node_modules") {
		parts := strings.Split(path, "node_modules/")
		if len(parts) < 2 {
			return ""
		}
		pkgPath := parts[len(parts)-1]

		// Handle scoped packages (@org/package)
		if strings.HasPrefix(pkgPath, "@") {
			pkgParts := strings.SplitN(pkgPath, "/", 3)
			if len(pkgParts) >= 2 {
				return pkgParts[0] + "/" + pkgParts[1]
			}
		}

		// Regular package
		pkgParts := strings.SplitN(pkgPath, "/", 2)
		return pkgParts[0]
	}

	// Direct package name (no node_modules in path)
	// This happens with external packages
	if !strings.Contains(path, "/") {
		return path
	}

	// Handle scoped packages
	if strings.HasPrefix(path, "@") {
		parts := strings.SplitN(path, "/", 3)
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}

	// First segment is the package name
	parts := strings.SplitN(path, "/", 2)
	return parts[0]
}
