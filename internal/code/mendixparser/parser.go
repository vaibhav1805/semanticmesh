package mendixparser

import (
	"os"
	"path/filepath"
)

// Parser handles Mendix project analysis
type Parser struct {
	config *Config
}

// New creates a new Mendix parser with default config
// The mxcliPath parameter is deprecated and ignored (mxcli is now used as a Go library)
func New(mxcliPath string) *Parser {
	cfg := DefaultConfig()
	// Store mxcliPath for backward compatibility (though it's no longer used)
	if mxcliPath != "" {
		cfg.MxcliPath = mxcliPath
	} else {
		cfg.MxcliPath = "mxcli"
	}
	return &Parser{
		config: cfg,
	}
}

// NewWithConfig creates a parser with custom configuration
func NewWithConfig(cfg *Config) *Parser {
	return &Parser{
		config: cfg,
	}
}

// DetectMendixProject checks if a path contains a Mendix project
func DetectMendixProject(path string) (bool, string, error) {
	// Check for .mpr file
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, "", err
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".mpr" {
			return true, filepath.Join(path, entry.Name()), nil
		}
	}

	// Check for mprcontents/ (MPR v2 format)
	mprContents := filepath.Join(path, "mprcontents")
	if info, err := os.Stat(mprContents); err == nil && info.IsDir() {
		// Look for .mpr file in parent
		parentEntries, _ := os.ReadDir(path)
		for _, entry := range parentEntries {
			if filepath.Ext(entry.Name()) == ".mpr" {
				return true, filepath.Join(path, entry.Name()), nil
			}
		}
	}

	return false, "", nil
}

// ExtractAppName extracts the app name from .mpr filename
func ExtractAppName(mprPath string) string {
	base := filepath.Base(mprPath)
	return base[:len(base)-len(filepath.Ext(base))]
}
