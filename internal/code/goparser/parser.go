package goparser

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code"
	"github.com/vaibhav1805/semanticmesh/internal/code/comments"
	"github.com/vaibhav1805/semanticmesh/internal/code/connstring"
)

// Compile-time checks.
var _ code.LanguageParser = (*GoParser)(nil)
var _ code.ManifestAnalyzer = (*GoParser)(nil)

// GoParser analyzes Go source files for infrastructure dependency signals
// using the standard library's go/ast package.
type GoParser struct {
	patterns map[string]DetectionPattern // "importPath.Function" -> pattern
}

// NewGoParser creates a GoParser with the default detection patterns.
func NewGoParser() *GoParser {
	return &GoParser{
		patterns: buildPatternIndex(DefaultPatterns),
	}
}

// Name returns "go".
func (p *GoParser) Name() string { return "go" }

// Extensions returns [".go"].
func (p *GoParser) Extensions() []string { return []string{".go"} }

// ParseFile analyzes a Go source file and returns detected infrastructure signals.
// Returns nil, nil for test files (*_test.go).
func (p *GoParser) ParseFile(filePath string, content []byte) ([]code.CodeSignal, error) {
	// Skip test files
	if strings.HasSuffix(filePath, "_test.go") {
		return nil, nil
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Build import alias map: local name -> full import path
	importMap := buildImportMap(f)

	var signals []code.CodeSignal

	// Walk AST for function calls matching patterns
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		// Resolve local package name to import path
		importPath, ok := importMap[ident.Name]
		if !ok {
			return true
		}

		// Look up pattern
		key := importPath + "." + sel.Sel.Name
		pattern, ok := p.patterns[key]
		if !ok {
			return true
		}

		// Extract target component
		target, targetType := extractTarget(call, pattern)

		lineNum := fset.Position(call.Pos()).Line

		signals = append(signals, code.CodeSignal{
			LineNumber:      lineNum,
			TargetComponent: target,
			TargetType:      targetType,
			DetectionKind:   pattern.Kind,
			Evidence:        evidenceSnippet(content, lineNum),
			Language:        "go",
			Confidence:      pattern.Confidence,
		})

		return true
	})

	// Scan comments using shared comment analyzer
	lines := strings.Split(string(content), "\n")
	commentSignals := comments.Analyze(lines, comments.SyntaxGo, nil)
	for i := range commentSignals {
		commentSignals[i].Language = "go"
		commentSignals[i].SourceFile = filePath
	}
	signals = append(signals, commentSignals...)

	// Scan for env var references
	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		refs := connstring.ParseEnvVarRef(line)
		for _, ref := range refs {
			if !connstring.IsConnectionEnvVar(ref.Name) {
				continue
			}
			targetType := inferEnvVarTargetType(ref.Name)
			signals = append(signals, code.CodeSignal{
				LineNumber:      lineNum,
				TargetComponent: ref.Name,
				TargetType:      targetType,
				DetectionKind:   "env_var_ref",
				Evidence:        evidenceSnippet(content, lineNum),
				Language:        "go",
				Confidence:      0.7,
			})
		}
	}

	return signals, nil
}

// buildImportMap creates a mapping from local package name to full import path,
// handling renamed imports (e.g., pg "database/sql").
func buildImportMap(f *ast.File) map[string]string {
	m := make(map[string]string)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)

		var localName string
		if imp.Name != nil {
			localName = imp.Name.Name
		} else {
			localName = defaultPackageName(path)
		}

		m[localName] = path
	}
	return m
}

// defaultPackageName infers the Go package name from an import path.
// Handles versioned paths (e.g., "github.com/redis/go-redis/v9" -> "redis")
// and paths ending in .go (e.g., "github.com/nats-io/nats.go" -> "nats").
func defaultPackageName(importPath string) string {
	base := filepath.Base(importPath)

	// Handle versioned modules: if last segment is vN, use the previous segment
	if isVersionSegment(base) {
		parts := strings.Split(importPath, "/")
		if len(parts) >= 2 {
			base = parts[len(parts)-2]
		}
	}

	// Handle paths ending in .go (e.g., nats.go -> nats)
	base = strings.TrimSuffix(base, ".go")

	// Handle hyphenated package names: go-redis -> redis
	// In Go, hyphens aren't valid in package names, so the actual package name
	// is typically the last segment without the prefix before the hyphen.
	// However, the convention varies. For common patterns like "go-redis",
	// the package name is "redis". We use the part after the last hyphen.
	if idx := strings.LastIndex(base, "-"); idx >= 0 {
		candidate := base[idx+1:]
		if len(candidate) > 0 {
			base = candidate
		}
	}

	return base
}

// isVersionSegment returns true if s looks like a Go module version segment (v2, v9, etc.)
func isVersionSegment(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// extractTarget determines the target component name and optionally enriches
// the signal's TargetType using connstring.Parse.
func extractTarget(call *ast.CallExpr, pattern DetectionPattern) (string, string) {
	targetType := pattern.TargetType

	if pattern.ArgIndex >= 0 && pattern.ArgIndex < len(call.Args) {
		if lit, ok := call.Args[pattern.ArgIndex].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			raw := strings.Trim(lit.Value, `"`)
			if result, ok := connstring.Parse(raw); ok {
				// Use connstring's TargetType if it's more specific
				if result.TargetType != "unknown" {
					targetType = result.TargetType
				}
				return result.Host, targetType
			}
		}
		// For db_connection, try to extract driver name from first arg
		if pattern.Kind == "db_connection" && pattern.ArgIndex == 1 && len(call.Args) > 0 {
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				driver := strings.Trim(lit.Value, `"`)
				return driver, targetType
			}
		}
	}

	// Use explicit fallback target if provided.
	if pattern.FallbackTarget != "" {
		return pattern.FallbackTarget, targetType
	}

	// Fallback: derive generic name from import path
	parts := strings.Split(pattern.ImportPath, "/")
	lastPart := parts[len(parts)-1]
	// Clean up version suffixes like "v9", "v8"
	if len(lastPart) > 1 && lastPart[0] == 'v' && lastPart[1] >= '0' && lastPart[1] <= '9' {
		if len(parts) >= 2 {
			lastPart = parts[len(parts)-2]
		}
	}
	return lastPart, targetType
}

// inferEnvVarTargetType infers a target type from an environment variable name.
func inferEnvVarTargetType(name string) string {
	upper := strings.ToUpper(name)
	switch {
	case strings.HasPrefix(upper, "DATABASE_") || strings.HasPrefix(upper, "DB_") || strings.HasPrefix(upper, "MONGO_"):
		return "database"
	case strings.HasPrefix(upper, "REDIS_"):
		return "cache"
	case strings.HasPrefix(upper, "KAFKA_") || strings.HasPrefix(upper, "RABBIT_") || strings.HasPrefix(upper, "AMQP_") || strings.HasPrefix(upper, "NATS_"):
		return "message-broker"
	case strings.HasPrefix(upper, "S3_"):
		return "storage"
	case strings.HasPrefix(upper, "ECR_"):
		return "container-registry"
	case strings.HasPrefix(upper, "DD_") || strings.HasPrefix(upper, "DATADOG_") ||
		strings.HasPrefix(upper, "SENTRY_") || strings.HasPrefix(upper, "PROMETHEUS_"):
		return "monitoring"
	case strings.HasPrefix(upper, "ELASTICSEARCH_") || strings.HasPrefix(upper, "ELASTIC_"):
		return "search"
	case strings.HasPrefix(upper, "VAULT_"):
		return "secrets-manager"
	case strings.HasPrefix(upper, "OAUTH_") || strings.HasPrefix(upper, "MXID_"):
		return "auth-service"
	case strings.HasPrefix(upper, "SLACK_"):
		return "notification"
	case strings.HasPrefix(upper, "ALERT_"):
		return "alerting"
	case strings.HasPrefix(upper, "CLOUDPORTAL_"):
		return "service"
	case strings.HasPrefix(upper, "GRPC_"):
		return "service"
	default:
		return "unknown"
	}
}

// AnalyzeManifests implements code.ManifestAnalyzer by analyzing go.mod.
func (p *GoParser) AnalyzeManifests(dir string) ([]code.CodeSignal, error) {
	return AnalyzeGoMod(dir)
}

// evidenceSnippet returns the source line at lineNum (1-based), trimmed, max 200 chars.
func evidenceSnippet(content []byte, lineNum int) string {
	lines := strings.Split(string(content), "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return ""
	}

	line := strings.TrimSpace(lines[lineNum-1])
	if len(line) > 200 {
		line = line[:200]
	}
	return line
}
