package pyparser

import (
	"testing"

	"github.com/graphmd/graphmd/internal/code"
)

// assertSignal is a helper that finds a signal by detection kind and target, then validates fields.
func assertSignal(t *testing.T, signals []code.CodeSignal, kind, target string) code.CodeSignal {
	t.Helper()
	for _, s := range signals {
		if s.DetectionKind == kind && s.TargetComponent == target {
			if s.Language != "python" {
				t.Errorf("expected language=python, got %q", s.Language)
			}
			return s
		}
	}
	t.Fatalf("signal not found: kind=%q target=%q (got %d signals: %v)", kind, target, len(signals), signals)
	return code.CodeSignal{}
}

func TestPythonParserImplementsInterface(t *testing.T) {
	// Compile-time check is in parser.go; this verifies constructor works.
	p := NewPythonParser()
	if p.Name() != "python" {
		t.Errorf("expected name=python, got %q", p.Name())
	}
	if exts := p.Extensions(); len(exts) != 1 || exts[0] != ".py" {
		t.Errorf("expected extensions=[.py], got %v", exts)
	}
}

func TestHTTPRequests(t *testing.T) {
	src := `
import requests

response = requests.get("http://payment-api:8080/pay")
data = requests.post("http://user-svc:3000/create", json=payload)
`
	p := NewPythonParser()
	signals, err := p.ParseFile("app.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d: %v", len(signals), signals)
	}

	s := assertSignal(t, signals, "http_call", "payment-api")
	if s.Confidence != 0.9 {
		t.Errorf("expected confidence=0.9, got %f", s.Confidence)
	}
	if s.TargetType != "service" {
		t.Errorf("expected target_type=service, got %q", s.TargetType)
	}

	assertSignal(t, signals, "http_call", "user-svc")
}

func TestHTTPRequestsAliased(t *testing.T) {
	src := `
import requests as req

resp = req.post("http://user-svc/create")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("handler.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "http_call", "user-svc")
}

func TestHTTPx(t *testing.T) {
	src := `
import httpx

resp = httpx.get("http://auth-svc:9090/verify")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("client.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "http_call", "auth-svc")
}

func TestDBPsycopg2(t *testing.T) {
	src := `
import psycopg2

conn = psycopg2.connect("postgres://primary-db:5432/mydb")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("db.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	s := assertSignal(t, signals, "db_connection", "primary-db")
	if s.Confidence != 0.85 {
		t.Errorf("expected confidence=0.85, got %f", s.Confidence)
	}
	if s.TargetType != "database" {
		t.Errorf("expected target_type=database, got %q", s.TargetType)
	}
}

func TestDBAsyncpg(t *testing.T) {
	src := `
import asyncpg

conn = await asyncpg.connect("postgres://replica-db:5432/mydb")
pool = await asyncpg.create_pool("postgres://pool-db:5432/mydb")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("async_db.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(signals))
	}
	assertSignal(t, signals, "db_connection", "replica-db")
	assertSignal(t, signals, "db_connection", "pool-db")
}

func TestDBSQLAlchemy(t *testing.T) {
	src := `
from sqlalchemy import create_engine

engine = create_engine("postgresql://analytics-db:5432/warehouse")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("models.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "db_connection", "analytics-db")
}

func TestDBPyMongo(t *testing.T) {
	src := `
from pymongo import MongoClient

client = MongoClient("mongodb://mongo-primary:27017/mydb")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("mongo.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "db_connection", "mongo-primary")
}

func TestDBPyMySQL(t *testing.T) {
	src := `
import pymysql

conn = pymysql.connect("mysql://mysql-primary:3306/mydb")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("mysql_conn.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "db_connection", "mysql-primary")
}

func TestCacheRedis(t *testing.T) {
	src := `
from redis import Redis

r = Redis(host="cache-01")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("cache.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	s := assertSignal(t, signals, "cache_client", "cache-01")
	if s.Confidence != 0.85 {
		t.Errorf("expected confidence=0.85, got %f", s.Confidence)
	}
	if s.TargetType != "cache" {
		t.Errorf("expected target_type=cache, got %q", s.TargetType)
	}
}

func TestCacheRedisFromURL(t *testing.T) {
	src := `
import redis

r = redis.from_url("redis://cache-primary:6379/0")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("cache2.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "cache_client", "cache-primary")
}

func TestCacheRedisStrictRedis(t *testing.T) {
	src := `
from redis import StrictRedis

r = StrictRedis(host="redis-main")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("strict_cache.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "cache_client", "redis-main")
}

func TestCacheMemcache(t *testing.T) {
	src := `
from pymemcache.client import Client

c = Client("memcache-01")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("memcache.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "cache_client", "memcache-01")
}

func TestQueueKafkaProducer(t *testing.T) {
	src := `
from kafka import KafkaProducer

producer = KafkaProducer(bootstrap_servers="kafka-broker:9092")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("producer.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	s := assertSignal(t, signals, "queue_producer", "kafka-broker")
	if s.Confidence != 0.85 {
		t.Errorf("expected confidence=0.85, got %f", s.Confidence)
	}
	if s.TargetType != "message-broker" {
		t.Errorf("expected target_type=message-broker, got %q", s.TargetType)
	}
}

func TestQueueKafkaConsumer(t *testing.T) {
	src := `
from kafka import KafkaConsumer

consumer = KafkaConsumer("topic", bootstrap_servers="kafka-primary:9092")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("consumer.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "queue_consumer", "kafka-primary")
}

func TestQueuePika(t *testing.T) {
	src := `
import pika

connection = pika.BlockingConnection(pika.ConnectionParameters("rabbitmq-host"))
`
	p := NewPythonParser()
	signals, err := p.ParseFile("rabbit.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "queue_producer", "rabbitmq-host")
}

func TestQueueBoto3SQS(t *testing.T) {
	src := `
import boto3

sqs = boto3.client("sqs")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("sqs_handler.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "queue_producer", "sqs")
}

func TestCommentHint(t *testing.T) {
	src := `
# Calls payment-api
# Depends on auth-service
def handler():
    pass
`
	p := NewPythonParser()
	signals, err := p.ParseFile("hints.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d: %v", len(signals), signals)
	}

	s := assertSignal(t, signals, "comment_hint", "payment-api")
	if s.Confidence != 0.4 {
		t.Errorf("expected confidence=0.4, got %f", s.Confidence)
	}
	assertSignal(t, signals, "comment_hint", "auth-service")
}

// False positive tests

func TestImportOnlyNoSignals(t *testing.T) {
	src := `
import requests
import psycopg2

# No actual calls, just imports
x = 42
`
	p := NewPythonParser()
	signals, err := p.ParseFile("no_calls.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 0 {
		t.Fatalf("expected 0 signals for import-only, got %d: %v", len(signals), signals)
	}
}

func TestCommentLineSkipped(t *testing.T) {
	// Commented-out code should not produce http_call signals from the main
	// pattern-matching loop. However, the shared comment analyzer may detect
	// URLs in comments as comment_hint signals (this is intended behavior).
	src := `
import requests

# requests.get("http://old-service/api")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("commented.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	// No http_call signals should be produced (the comment line is not code)
	for _, s := range signals {
		if s.DetectionKind == "http_call" {
			t.Errorf("got http_call from comment line, should be skipped")
		}
	}
	// The shared comment analyzer may detect the URL as a comment_hint, which is fine
}

func TestDecoratorLineSkipped(t *testing.T) {
	src := `
import requests

@app.route("/api/v1")
def handler():
    pass
`
	p := NewPythonParser()
	signals, err := p.ParseFile("decorated.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 0 {
		t.Fatalf("expected 0 signals for decorator lines, got %d: %v", len(signals), signals)
	}
}

func TestTestFileSkipped(t *testing.T) {
	src := `
import requests

requests.get("http://test-svc/api")
`
	p := NewPythonParser()

	// test_*.py pattern
	signals, err := p.ParseFile("test_handler.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if signals != nil {
		t.Fatalf("expected nil for test_handler.py, got %d signals", len(signals))
	}

	// *_test.py pattern
	signals, err = p.ParseFile("handler_test.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if signals != nil {
		t.Fatalf("expected nil for handler_test.py, got %d signals", len(signals))
	}

	// conftest.py
	signals, err = p.ParseFile("conftest.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if signals != nil {
		t.Fatalf("expected nil for conftest.py, got %d signals", len(signals))
	}
}

func TestFromImportAliased(t *testing.T) {
	src := `
from kafka import KafkaProducer as KP

producer = KP(bootstrap_servers="kafka-alias:9092")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("aliased.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	assertSignal(t, signals, "queue_producer", "kafka-alias")
}

func TestMultipleImportsAndCalls(t *testing.T) {
	src := `
import requests
import psycopg2
from redis import Redis

resp = requests.get("http://api-gateway:8080/health")
conn = psycopg2.connect("postgres://main-db:5432/app")
r = Redis(host="session-cache")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("mixed.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 3 {
		t.Fatalf("expected 3 signals, got %d: %v", len(signals), signals)
	}
	assertSignal(t, signals, "http_call", "api-gateway")
	assertSignal(t, signals, "db_connection", "main-db")
	assertSignal(t, signals, "cache_client", "session-cache")
}

func TestEvidenceSnippetPresent(t *testing.T) {
	src := `
import requests

resp = requests.get("http://payment-api:8080/pay")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("evidence.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].Evidence == "" {
		t.Error("expected non-empty evidence snippet")
	}
}

func TestLineNumberCorrect(t *testing.T) {
	src := `import requests

# some comment
x = 1
resp = requests.get("http://line-test:8080/api")
`
	p := NewPythonParser()
	signals, err := p.ParseFile("lines.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].LineNumber != 5 {
		t.Errorf("expected line 5, got %d", signals[0].LineNumber)
	}
}
