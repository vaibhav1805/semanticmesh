package knowledge

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultIgnorePatterns returns sensible default directory patterns to exclude
// from scanning. These cover common build output, dependency, and tooling
// directories that are unlikely to contain useful documentation.
func DefaultIgnorePatterns() []string {
	return []string{
		"vendor",
		"node_modules",
		".git",
		"__pycache__",
		".venv",
		"dist",
		"build",
		"target",
		".gradle",
		".next",
		"out",
		".cache",
		"bin",
		"obj",
		".bmd",
		".planning",
	}
}

// LoadIgnoreFile reads a .semanticmeshignore file from the given directory.
// It returns two slices: directory patterns and file patterns.
//
// Format rules:
//   - One pattern per line
//   - Lines starting with '#' are comments (ignored)
//   - Blank lines are skipped
//   - Patterns ending with '/' are directory patterns (trailing slash stripped)
//   - All other patterns are file patterns
//
// If the file does not exist, DefaultIgnorePatterns() is returned for dirs
// and an empty slice for files (no error).
func LoadIgnoreFile(dir string) (dirs []string, files []string, err error) {
	path := filepath.Join(dir, ".semanticmeshignore")

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultIgnorePatterns(), nil, nil
		}
		return nil, nil, fmt.Errorf("knowledge.LoadIgnoreFile: open %q: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasSuffix(line, "/") {
			// Directory pattern — strip trailing slash.
			dirs = append(dirs, strings.TrimSuffix(line, "/"))
		} else {
			files = append(files, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("knowledge.LoadIgnoreFile: read %q: %w", path, err)
	}

	return dirs, files, nil
}

// GenerateIgnoreFile creates a .semanticmeshignore file with default patterns
// in the given directory. If the file already exists, it is not overwritten.
func GenerateIgnoreFile(dir string) error {
	path := filepath.Join(dir, ".semanticmeshignore")

	// Do not overwrite an existing file.
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	var b strings.Builder
	b.WriteString("# .semanticmeshignore — patterns for files and directories to exclude from scanning.\n")
	b.WriteString("#\n")
	b.WriteString("# One pattern per line.\n")
	b.WriteString("# Lines starting with '#' are comments.\n")
	b.WriteString("# Patterns ending with '/' match directories only.\n")
	b.WriteString("# All other patterns match files.\n")
	b.WriteString("# Supports glob wildcards: *.lock, temp_*\n")
	b.WriteString("#\n")
	b.WriteString("# Directories\n")

	for _, p := range DefaultIgnorePatterns() {
		b.WriteString(p + "/\n")
	}

	b.WriteString("\n# Files\n")
	b.WriteString("# *.lock\n")

	return os.WriteFile(path, []byte(b.String()), 0o644)
}
