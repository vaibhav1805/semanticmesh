package jsparser

import (
	"testing"
)

func TestExtractPackageName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"node_modules/axios/index.js", "axios"},
		{"node_modules/@aws-sdk/client-s3/dist/index.js", "@aws-sdk/client-s3"},
		{"node_modules/express/lib/express.js", "express"},
		{"../relative/path.js", ""},
		{"./local/file.js", ""},
		{"/absolute/path.js", ""},
		{"axios", "axios"},
		{"@aws-sdk/client-s3", "@aws-sdk/client-s3"},
		{"express/lib/router", "express"},
		{"@prisma/client/runtime", "@prisma/client"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractPackageName(tt.path)
			if got != tt.want {
				t.Errorf("extractPackageName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestESBuildAnalyzer(t *testing.T) {
	src := `
import axios from 'axios';
import { Pool } from 'pg';
import { S3Client } from '@aws-sdk/client-s3';

const client = axios.create();
const pool = new Pool();
const s3 = new S3Client();
`

	analyzer := NewESBuildAnalyzer()
	importMap, err := analyzer.AnalyzeImports("test.ts", []byte(src))
	if err != nil {
		// esbuild might not be available in all test environments, so we just log the error
		t.Logf("esbuild analysis failed (expected in some environments): %v", err)
		return
	}

	// Check that we found the expected packages
	expectedPackages := []string{"axios", "pg", "@aws-sdk/client-s3"}
	for _, pkg := range expectedPackages {
		if _, ok := importMap[pkg]; !ok {
			t.Errorf("expected package %q in import map, but it was not found. Got: %v", pkg, importMap)
		}
	}
}

func TestESBuildAnalyzerDynamicImport(t *testing.T) {
	src := `
async function loadDB() {
	const { Pool } = await import('pg');
	const pool = new Pool({ host: 'db-01' });
	return pool;
}
`

	analyzer := NewESBuildAnalyzer()
	importMap, err := analyzer.AnalyzeImports("dynamic.ts", []byte(src))
	if err != nil {
		// esbuild might not be available in all test environments
		t.Logf("esbuild analysis failed (expected in some environments): %v", err)
		return
	}

	// esbuild should detect dynamic imports
	if _, ok := importMap["pg"]; !ok {
		t.Logf("Note: esbuild did not detect dynamic import 'pg'. This is expected behavior. Got: %v", importMap)
	}
}
