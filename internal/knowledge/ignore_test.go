package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreDefaults(t *testing.T) {
	patterns := DefaultIgnorePatterns()
	if len(patterns) == 0 {
		t.Fatal("DefaultIgnorePatterns returned empty slice")
	}
	// Spot-check a few expected patterns.
	want := map[string]bool{"vendor": false, "node_modules": false, ".git": false, ".planning": false}
	for _, p := range patterns {
		if _, ok := want[p]; ok {
			want[p] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("DefaultIgnorePatterns missing %q", name)
		}
	}
}

func TestIgnoreLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	dirs, files, err := LoadIgnoreFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected no file patterns, got %v", files)
	}
	// Should return defaults for dirs.
	if len(dirs) == 0 {
		t.Error("expected default dir patterns, got empty")
	}
	if dirs[0] != DefaultIgnorePatterns()[0] {
		t.Errorf("expected default patterns, got %v", dirs)
	}
}

func TestIgnoreLoadExistingFile(t *testing.T) {
	dir := t.TempDir()
	content := `# Comment line
vendor/
node_modules/

*.lock
CLAUDE.md
# Another comment
build/
`
	if err := os.WriteFile(filepath.Join(dir, ".semanticmeshignore"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	dirs, files, err := LoadIgnoreFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDirs := []string{"vendor", "node_modules", "build"}
	if len(dirs) != len(wantDirs) {
		t.Fatalf("dirs: got %v, want %v", dirs, wantDirs)
	}
	for i, d := range dirs {
		if d != wantDirs[i] {
			t.Errorf("dirs[%d]: got %q, want %q", i, d, wantDirs[i])
		}
	}

	wantFiles := []string{"*.lock", "CLAUDE.md"}
	if len(files) != len(wantFiles) {
		t.Fatalf("files: got %v, want %v", files, wantFiles)
	}
	for i, f := range files {
		if f != wantFiles[i] {
			t.Errorf("files[%d]: got %q, want %q", i, f, wantFiles[i])
		}
	}
}

func TestIgnoreGenerate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".semanticmeshignore")

	// Generate should create the file.
	if err := GenerateIgnoreFile(dir); err != nil {
		t.Fatalf("GenerateIgnoreFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	// Read it back and verify it can be parsed.
	dirs, files, err := LoadIgnoreFile(dir)
	if err != nil {
		t.Fatalf("LoadIgnoreFile after generate: %v", err)
	}
	if len(dirs) == 0 {
		t.Error("generated file should contain directory patterns")
	}
	// The commented-out *.lock should not be parsed as a file pattern.
	if len(files) != 0 {
		t.Errorf("generated file should have no active file patterns, got %v", files)
	}

	// Generate again should not overwrite.
	// Write a sentinel to verify.
	if err := os.WriteFile(path, []byte("sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := GenerateIgnoreFile(dir); err != nil {
		t.Fatalf("second GenerateIgnoreFile: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "sentinel" {
		t.Error("GenerateIgnoreFile overwrote existing file")
	}
}
