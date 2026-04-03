// Package connstring provides shared connection string parsing for extracting
// target component names from URLs, DSNs, and environment variable references.
package connstring

import (
	"net/url"
	"regexp"
	"strings"
)

// Result holds the parsed output of a connection string.
type Result struct {
	Host       string  // Extracted hostname or env var name
	TargetType string  // database, cache, message-broker, service, unknown
	Confidence float64 // 0.85 for parsed connection strings, 0.7 for env var refs
	Kind       string  // "conn_string" or "env_var_ref"
}

// EnvVarRef represents an environment variable reference found in a line of code.
type EnvVarRef struct {
	Name     string // The variable name (e.g., "DATABASE_URL")
	Position int    // Byte offset within the line
}

// schemeToType maps URL schemes to component target types.
var schemeToType = map[string]string{
	"postgres":    "database",
	"postgresql":  "database",
	"mysql":       "database",
	"mongodb":     "database",
	"mongodb+srv": "database",
	"redis":       "cache",
	"rediss":      "cache",
	"amqp":        "message-broker",
	"amqps":       "message-broker",
	"nats":        "message-broker",
	"http":        "service",
	"https":       "service",
}

// filteredHosts contains hostnames that should be excluded (documentation, examples, loopback).
var filteredHosts = map[string]bool{
	"example.com":             true,
	"example.org":             true,
	"example.net":             true,
	"localhost":               true,
	"127.0.0.1":               true,
	"0.0.0.0":                 true,
	"docs.python.org":         true,
	"pkg.go.dev":              true,
	"developer.mozilla.org":   true,
	"github.com":              true,
	"stackoverflow.com":       true,
	"en.wikipedia.org":        true,
	"www.w3.org":              true,
	"tools.ietf.org":          true,
	"registry.npmjs.org":      true,
	"pypi.org":                true,
	"rubygems.org":            true,
	"hub.docker.com":          true,
	"schemas.xmlsoap.org":     true,
	"www.googleapis.com":      true,
	"json-schema.org":         true,
	"purl.org":                true,
}

// mysqlDSNRe matches MySQL DSN format: user:pass@tcp(host:port)/dbname
var mysqlDSNRe = regexp.MustCompile(`@tcp\(([^:)]+)(?::\d+)?\)`)

// pgKeyValueRe matches PostgreSQL key-value DSN: host=X
var pgKeyValueRe = regexp.MustCompile(`\bhost=(\S+)`)

// hostPortRe matches bare host:port patterns.
var hostPortRe = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9._-]+):(\d+)$`)

// Parse attempts to extract a target component from a raw connection string.
// It tries URL parsing, then DSN formats, then host:port fallback.
// Returns ok=false if no valid target could be extracted.
func Parse(raw string) (Result, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Result{}, false
	}

	// Try URL parsing first.
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
		host := u.Hostname()
		if host == "" || filteredHosts[host] {
			return Result{}, false
		}
		targetType := "unknown"
		if t, ok := schemeToType[strings.ToLower(u.Scheme)]; ok {
			targetType = t
		}
		return Result{
			Host:       host,
			TargetType: targetType,
			Confidence: 0.85,
			Kind:       "conn_string",
		}, true
	}

	// Try MySQL DSN: user:pass@tcp(host:port)/dbname
	if m := mysqlDSNRe.FindStringSubmatch(raw); m != nil {
		host := m[1]
		if filteredHosts[host] {
			return Result{}, false
		}
		return Result{
			Host:       host,
			TargetType: "database",
			Confidence: 0.85,
			Kind:       "conn_string",
		}, true
	}

	// Try PostgreSQL key-value DSN: host=X port=Y dbname=Z
	if m := pgKeyValueRe.FindStringSubmatch(raw); m != nil {
		host := m[1]
		if filteredHosts[host] {
			return Result{}, false
		}
		return Result{
			Host:       host,
			TargetType: "database",
			Confidence: 0.85,
			Kind:       "conn_string",
		}, true
	}

	// Try bare host:port
	if m := hostPortRe.FindStringSubmatch(raw); m != nil {
		host := m[1]
		if filteredHosts[host] {
			return Result{}, false
		}
		return Result{
			Host:       host,
			TargetType: "unknown",
			Confidence: 0.85,
			Kind:       "conn_string",
		}, true
	}

	return Result{}, false
}

// Env var reference patterns across languages.
var envVarPatterns = []*regexp.Regexp{
	// Go: os.Getenv("VAR"), os.LookupEnv("VAR")
	regexp.MustCompile(`os\.(?:Getenv|LookupEnv)\("([A-Z_][A-Z0-9_]*)"\)`),
	// Python: os.environ["VAR"], os.environ.get("VAR"), os.getenv("VAR")
	regexp.MustCompile(`os\.environ\["([A-Z_][A-Z0-9_]*)"\]`),
	regexp.MustCompile(`os\.environ\.get\("([A-Z_][A-Z0-9_]*)"\)`),
	regexp.MustCompile(`os\.getenv\("([A-Z_][A-Z0-9_]*)"\)`),
	// JS: process.env.VAR, process.env["VAR"]
	regexp.MustCompile(`process\.env\.([A-Z_][A-Z0-9_]*)`),
	regexp.MustCompile(`process\.env\["([A-Z_][A-Z0-9_]*)"\]`),
	// Shell: ${VAR}, $VAR (only uppercase env-var-like names)
	regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`),
	regexp.MustCompile(`\$([A-Z_][A-Z0-9_]*)\b`),
}

// ParseEnvVarRef scans a line for environment variable references and returns
// all matches found. Supports Go, Python, JS, and shell-style references.
func ParseEnvVarRef(line string) []EnvVarRef {
	seen := make(map[string]bool)
	var refs []EnvVarRef

	for _, pat := range envVarPatterns {
		for _, m := range pat.FindAllStringSubmatchIndex(line, -1) {
			name := line[m[2]:m[3]]
			if seen[name] {
				continue
			}
			seen[name] = true
			refs = append(refs, EnvVarRef{
				Name:     name,
				Position: m[0],
			})
		}
	}
	return refs
}

// Connection-related env var suffixes.
var connSuffixes = []string{
	"_URL", "_DSN", "_URI", "_HOST", "_ADDR", "_ENDPOINT", "_CONNECTION",
}

// Connection-related env var prefixes.
var connPrefixes = []string{
	"DATABASE_", "DB_", "REDIS_", "MONGO_", "RABBIT_", "KAFKA_", "NATS_", "AMQP_",
}

// IsConnectionEnvVar returns true if the environment variable name suggests
// it holds a connection string or host reference.
func IsConnectionEnvVar(name string) bool {
	upper := strings.ToUpper(name)
	for _, suffix := range connSuffixes {
		if strings.HasSuffix(upper, suffix) {
			return true
		}
	}
	for _, prefix := range connPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}
