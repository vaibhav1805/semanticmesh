package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
)

// GraphStorageDir returns the base directory for storing imported graphs.
// Uses $XDG_DATA_HOME/semanticmesh/graphs/ if set, otherwise falls back to
// ~/.local/share/semanticmesh/graphs/.
func GraphStorageDir() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "semanticmesh", "graphs"), nil
}

// graphStoragePath returns the full path for a named graph directory.
func graphStoragePath(name string) (string, error) {
	storageDir, err := GraphStorageDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(storageDir, name), nil
}

// getCurrentGraph reads the "current" marker file to get the default graph name.
// Returns empty string and an error if no current graph is set.
func getCurrentGraph(storageDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(storageDir, "current"))
	if err != nil {
		return "", fmt.Errorf("no default graph set: %w", err)
	}
	name := string(data)
	if name == "" {
		return "", fmt.Errorf("current graph marker is empty")
	}
	return name, nil
}

// setCurrentGraph writes the "current" marker file with the given graph name.
func setCurrentGraph(storageDir, name string) error {
	return os.WriteFile(filepath.Join(storageDir, "current"), []byte(name), 0o644)
}
