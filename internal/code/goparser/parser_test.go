package goparser

import (
	"strings"
	"testing"

	"github.com/semanticmesh/semanticmesh/internal/code"
)

// Compile-time interface check.
var _ code.LanguageParser = (*GoParser)(nil)

func TestHTTPDetection(t *testing.T) {
	src := []byte(`package main

import "net/http"

func fetch() {
	http.Get("http://payment-api:8080/pay")
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	s := signals[0]
	if s.TargetComponent != "payment-api" {
		t.Errorf("TargetComponent = %q, want %q", s.TargetComponent, "payment-api")
	}
	if s.DetectionKind != "http_call" {
		t.Errorf("DetectionKind = %q, want %q", s.DetectionKind, "http_call")
	}
	if s.Confidence != 0.9 {
		t.Errorf("Confidence = %v, want 0.9", s.Confidence)
	}
	if s.TargetType != "service" {
		t.Errorf("TargetType = %q, want %q", s.TargetType, "service")
	}
	if s.Language != "go" {
		t.Errorf("Language = %q, want %q", s.Language, "go")
	}
}

func TestHTTPPost(t *testing.T) {
	src := []byte(`package main

import "net/http"

func send() {
	http.Post("http://notification-svc:3000/notify", "application/json", nil)
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].TargetComponent != "notification-svc" {
		t.Errorf("TargetComponent = %q, want %q", signals[0].TargetComponent, "notification-svc")
	}
}

func TestHTTPNewRequest(t *testing.T) {
	src := []byte(`package main

import "net/http"

func call() {
	http.NewRequest("GET", "http://auth-api:9090/validate", nil)
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].TargetComponent != "auth-api" {
		t.Errorf("TargetComponent = %q, want %q", signals[0].TargetComponent, "auth-api")
	}
}

func TestDBDetection(t *testing.T) {
	src := []byte(`package main

import "database/sql"

func connect() {
	sql.Open("postgres", "postgres://primary-db:5432/mydb")
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	s := signals[0]
	if s.TargetComponent != "primary-db" {
		t.Errorf("TargetComponent = %q, want %q", s.TargetComponent, "primary-db")
	}
	if s.DetectionKind != "db_connection" {
		t.Errorf("DetectionKind = %q, want %q", s.DetectionKind, "db_connection")
	}
	if s.Confidence != 0.85 {
		t.Errorf("Confidence = %v, want 0.85", s.Confidence)
	}
	if s.TargetType != "database" {
		t.Errorf("TargetType = %q, want %q", s.TargetType, "database")
	}
}

func TestCacheDetection(t *testing.T) {
	src := []byte(`package main

import "github.com/redis/go-redis/v9"

func cache() {
	redis.NewClient(nil)
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	s := signals[0]
	if s.DetectionKind != "cache_client" {
		t.Errorf("DetectionKind = %q, want %q", s.DetectionKind, "cache_client")
	}
	if s.Confidence != 0.85 {
		t.Errorf("Confidence = %v, want 0.85", s.Confidence)
	}
	if s.TargetType != "cache" {
		t.Errorf("TargetType = %q, want %q", s.TargetType, "cache")
	}
}

func TestQueueProducerDetection(t *testing.T) {
	src := []byte(`package main

import "github.com/IBM/sarama"

func produce() {
	sarama.NewSyncProducer(nil, nil)
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	s := signals[0]
	if s.DetectionKind != "queue_producer" {
		t.Errorf("DetectionKind = %q, want %q", s.DetectionKind, "queue_producer")
	}
	if s.Confidence != 0.85 {
		t.Errorf("Confidence = %v, want 0.85", s.Confidence)
	}
	if s.TargetType != "message-broker" {
		t.Errorf("TargetType = %q, want %q", s.TargetType, "message-broker")
	}
}

func TestQueueConsumerDetection(t *testing.T) {
	src := []byte(`package main

import "github.com/IBM/sarama"

func consume() {
	sarama.NewConsumer(nil, nil)
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].DetectionKind != "queue_consumer" {
		t.Errorf("DetectionKind = %q, want %q", signals[0].DetectionKind, "queue_consumer")
	}
}

func TestRenamedImport(t *testing.T) {
	src := []byte(`package main

import pg "database/sql"

func connect() {
	pg.Open("postgres", "postgres://renamed-db:5432/mydb")
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	if signals[0].TargetComponent != "renamed-db" {
		t.Errorf("TargetComponent = %q, want %q", signals[0].TargetComponent, "renamed-db")
	}
	if signals[0].DetectionKind != "db_connection" {
		t.Errorf("DetectionKind = %q, want %q", signals[0].DetectionKind, "db_connection")
	}
}

func TestTestFileSkip(t *testing.T) {
	src := []byte(`package main

import "net/http"

func TestFoo(t *testing.T) {
	http.Get("http://test-api:8080/test")
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("foo_test.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if signals != nil {
		t.Errorf("expected nil signals for test file, got %d signals", len(signals))
	}
}

func TestImportOnlyNoSignals(t *testing.T) {
	src := []byte(`package main

import "database/sql"

var _ = sql.ErrNoRows
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 0 {
		t.Errorf("expected 0 signals for import-only, got %d", len(signals))
	}
}

func TestCommentHint(t *testing.T) {
	src := []byte(`package main

// Calls payment-api
func handle() {}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	s := signals[0]
	if s.TargetComponent != "payment-api" {
		t.Errorf("TargetComponent = %q, want %q", s.TargetComponent, "payment-api")
	}
	if s.DetectionKind != "comment_hint" {
		t.Errorf("DetectionKind = %q, want %q", s.DetectionKind, "comment_hint")
	}
	if s.Confidence != 0.4 {
		t.Errorf("Confidence = %v, want 0.4", s.Confidence)
	}
}

func TestCommentHintVariants(t *testing.T) {
	src := []byte(`package main

// Depends on auth-service
// Uses redis-cache
// Connects to primary-db
func handle() {}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 3 {
		t.Fatalf("expected 3 comment hint signals, got %d", len(signals))
	}

	expected := []string{"auth-service", "redis-cache", "primary-db"}
	for i, want := range expected {
		if signals[i].TargetComponent != want {
			t.Errorf("signals[%d].TargetComponent = %q, want %q", i, signals[i].TargetComponent, want)
		}
		if signals[i].DetectionKind != "comment_hint" {
			t.Errorf("signals[%d].DetectionKind = %q, want %q", i, signals[i].DetectionKind, "comment_hint")
		}
	}
}

func TestURLTargetExtraction(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantHost string
	}{
		{"hostname with port", "http://payment-api:8080/pay", "payment-api"},
		{"hostname without port", "http://auth-service/validate", "auth-service"},
		{"IP address", "http://10.0.0.5:3000/api", "10.0.0.5"},
		{"FQDN", "https://api.example.com/v1/data", "api.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(`package main

import "net/http"

func fetch() {
	http.Get("` + tt.url + `")
}
`)
			p := NewGoParser()
			signals, err := p.ParseFile("main.go", src)
			if err != nil {
				t.Fatal(err)
			}

			if len(signals) != 1 {
				t.Fatalf("expected 1 signal, got %d", len(signals))
			}
			if signals[0].TargetComponent != tt.wantHost {
				t.Errorf("TargetComponent = %q, want %q", signals[0].TargetComponent, tt.wantHost)
			}
		})
	}
}

func TestVariableArgFallback(t *testing.T) {
	src := []byte(`package main

import "database/sql"

func connect() {
	connStr := "postgres://primary-db:5432/mydb"
	sql.Open("postgres", connStr)
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	// When the connection string is a variable, should fall back to driver name
	if signals[0].TargetComponent != "postgres" {
		t.Errorf("TargetComponent = %q, want %q", signals[0].TargetComponent, "postgres")
	}
}

func TestMultipleSignals(t *testing.T) {
	src := []byte(`package main

import (
	"net/http"
	"database/sql"
)

func main() {
	http.Get("http://user-api:8080/users")
	sql.Open("postgres", "postgres://main-db:5432/app")
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(signals))
	}

	// First signal: HTTP
	if signals[0].DetectionKind != "http_call" {
		t.Errorf("signals[0].DetectionKind = %q, want %q", signals[0].DetectionKind, "http_call")
	}
	if signals[0].TargetComponent != "user-api" {
		t.Errorf("signals[0].TargetComponent = %q, want %q", signals[0].TargetComponent, "user-api")
	}

	// Second signal: DB
	if signals[1].DetectionKind != "db_connection" {
		t.Errorf("signals[1].DetectionKind = %q, want %q", signals[1].DetectionKind, "db_connection")
	}
	if signals[1].TargetComponent != "main-db" {
		t.Errorf("signals[1].TargetComponent = %q, want %q", signals[1].TargetComponent, "main-db")
	}
}

func TestEvidenceSnippet(t *testing.T) {
	src := []byte(`package main

import "net/http"

func fetch() {
	http.Get("http://payment-api:8080/pay")
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	if signals[0].Evidence == "" {
		t.Error("Evidence should not be empty")
	}

	// Evidence should contain the call
	if !containsStr(signals[0].Evidence, "http.Get") {
		t.Errorf("Evidence = %q, should contain %q", signals[0].Evidence, "http.Get")
	}
}

func TestCacheRedisGenericTarget(t *testing.T) {
	// Redis NewClient doesn't take a URL arg, should produce a generic target
	src := []byte(`package main

import "github.com/redis/go-redis/v9"

func cache() {
	redis.NewClient(nil)
}
`)
	p := NewGoParser()
	signals, err := p.ParseFile("main.go", src)
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	// Should get a generic name derived from the import path
	s := signals[0]
	if s.TargetComponent != "go-redis" {
		t.Errorf("TargetComponent = %q, want generic fallback like %q", s.TargetComponent, "go-redis")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
}
