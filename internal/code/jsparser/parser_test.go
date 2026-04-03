package jsparser

import (
	"testing"

	"github.com/vaibhav1805/semanticmesh/internal/code"
)

// signalByKind finds the first signal with the given detection kind.
func signalByKind(signals []code.CodeSignal, kind string) *code.CodeSignal {
	for i := range signals {
		if signals[i].DetectionKind == kind {
			return &signals[i]
		}
	}
	return nil
}

// signalByTarget finds the first signal with the given target component.
func signalByTarget(signals []code.CodeSignal, target string) *code.CodeSignal {
	for i := range signals {
		if signals[i].TargetComponent == target {
			return &signals[i]
		}
	}
	return nil
}

func TestJSParserInterface(t *testing.T) {
	p := NewJSParser()
	if p.Name() != "javascript" {
		t.Errorf("Name() = %q, want %q", p.Name(), "javascript")
	}
	exts := p.Extensions()
	want := map[string]bool{".js": true, ".ts": true, ".jsx": true, ".tsx": true}
	for _, ext := range exts {
		if !want[ext] {
			t.Errorf("unexpected extension %q", ext)
		}
		delete(want, ext)
	}
	if len(want) > 0 {
		t.Errorf("missing extensions: %v", want)
	}
}

func TestAxiosGet(t *testing.T) {
	src := `import axios from 'axios';

async function pay() {
  const resp = await axios.get("http://payment-api:8080/pay");
  return resp.data;
}
`
	p := NewJSParser()
	signals, err := p.ParseFile("service.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	s := signals[0]
	if s.DetectionKind != "http_call" {
		t.Errorf("kind = %q, want http_call", s.DetectionKind)
	}
	if s.TargetComponent != "payment-api" {
		t.Errorf("target = %q, want payment-api", s.TargetComponent)
	}
	if s.Confidence != 0.9 {
		t.Errorf("confidence = %f, want 0.9", s.Confidence)
	}
	if s.Language != "javascript" {
		t.Errorf("language = %q, want javascript", s.Language)
	}
}

func TestAxiosCommonJS(t *testing.T) {
	src := `const axios = require('axios');

axios.post("http://user-svc/create", { name: "test" });
`
	p := NewJSParser()
	signals, err := p.ParseFile("handler.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	if signals[0].TargetComponent != "user-svc" {
		t.Errorf("target = %q, want user-svc", signals[0].TargetComponent)
	}
}

func TestFetchGlobalNoimport(t *testing.T) {
	src := `async function getOrders() {
  const resp = await fetch("http://order-api:3000/orders");
  return resp.json();
}
`
	p := NewJSParser()
	signals, err := p.ParseFile("api.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	s := signals[0]
	if s.DetectionKind != "http_call" {
		t.Errorf("kind = %q, want http_call", s.DetectionKind)
	}
	if s.TargetComponent != "order-api" {
		t.Errorf("target = %q, want order-api", s.TargetComponent)
	}
}

func TestPgPool(t *testing.T) {
	src := `import { Pool } from 'pg';

const pool = new Pool({
  host: "primary-db",
  port: 5432,
  database: "myapp",
});
`
	p := NewJSParser()
	signals, err := p.ParseFile("db.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	s := signals[0]
	if s.DetectionKind != "db_connection" {
		t.Errorf("kind = %q, want db_connection", s.DetectionKind)
	}
	if s.TargetType != "database" {
		t.Errorf("target_type = %q, want database", s.TargetType)
	}
}

func TestMongooseConnect(t *testing.T) {
	src := `import mongoose from 'mongoose';

mongoose.connect("mongodb://primary-db:27017/mydb");
`
	p := NewJSParser()
	signals, err := p.ParseFile("models.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	s := signals[0]
	if s.DetectionKind != "db_connection" {
		t.Errorf("kind = %q, want db_connection", s.DetectionKind)
	}
	if s.TargetComponent != "primary-db" {
		t.Errorf("target = %q, want primary-db", s.TargetComponent)
	}
}

func TestIoredisConstructor(t *testing.T) {
	src := `import Redis from 'ioredis';

const client = new Redis("redis://cache-01:6379");
`
	p := NewJSParser()
	signals, err := p.ParseFile("cache.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	s := signals[0]
	if s.DetectionKind != "cache_client" {
		t.Errorf("kind = %q, want cache_client", s.DetectionKind)
	}
	if s.TargetComponent != "cache-01" {
		t.Errorf("target = %q, want cache-01", s.TargetComponent)
	}
}

func TestKafkaJSConstructor(t *testing.T) {
	src := `import { Kafka } from 'kafkajs';

const kafka = new Kafka({
  brokers: ["kafka-01:9092"],
  clientId: "my-app",
});
`
	p := NewJSParser()
	signals, err := p.ParseFile("events.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	s := signals[0]
	if s.DetectionKind != "queue_producer" {
		t.Errorf("kind = %q, want queue_producer", s.DetectionKind)
	}
	if s.TargetType != "message-broker" {
		t.Errorf("target_type = %q, want message-broker", s.TargetType)
	}
}

func TestAmqplibConnect(t *testing.T) {
	src := `const amqplib = require('amqplib');

async function connect() {
  const conn = await amqplib.connect("amqp://rabbitmq-host");
  return conn;
}
`
	p := NewJSParser()
	signals, err := p.ParseFile("queue.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	s := signals[0]
	if s.DetectionKind != "queue_producer" {
		t.Errorf("kind = %q, want queue_producer", s.DetectionKind)
	}
	if s.TargetComponent != "rabbitmq-host" {
		t.Errorf("target = %q, want rabbitmq-host", s.TargetComponent)
	}
}

func TestImportOnlyNoSignals(t *testing.T) {
	src := `import axios from 'axios';

const config = { timeout: 5000 };
`
	p := NewJSParser()
	signals, err := p.ParseFile("config.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for import-only, got %d: %+v", len(signals), signals)
	}
}

func TestSingleLineCommentNoSignals(t *testing.T) {
	src := `import axios from 'axios';

// axios.get("http://old-api/endpoint")
const x = 1;
`
	p := NewJSParser()
	signals, err := p.ParseFile("handler.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	// Should get 0 real signals (the comment line is skipped)
	for _, s := range signals {
		if s.DetectionKind == "http_call" {
			t.Errorf("got http_call from comment line, should be skipped")
		}
	}
}

func TestBlockCommentNoSignals(t *testing.T) {
	src := `/*
fetch("http://test-api/endpoint")
axios.get("http://another/path")
*/
const x = 1;
`
	p := NewJSParser()
	signals, err := p.ParseFile("handler.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range signals {
		if s.DetectionKind == "http_call" {
			t.Errorf("got http_call from block comment, should be skipped")
		}
	}
}

func TestTestFileSkipped(t *testing.T) {
	src := `import axios from 'axios';
axios.get("http://payment-api:8080/pay");
`
	p := NewJSParser()

	for _, name := range []string{"handler.test.ts", "handler.test.js", "handler.spec.ts", "handler.spec.js", "handler.test.jsx", "handler.test.tsx"} {
		signals, err := p.ParseFile(name, []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if len(signals) != 0 {
			t.Errorf("%s: expected 0 signals for test file, got %d", name, len(signals))
		}
	}
}

func TestCommentHint(t *testing.T) {
	src := `// Calls payment-api
function process() {
  // business logic
}
`
	p := NewJSParser()
	signals, err := p.ParseFile("worker.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	s := signals[0]
	if s.DetectionKind != "comment_hint" {
		t.Errorf("kind = %q, want comment_hint", s.DetectionKind)
	}
	if s.TargetComponent != "payment-api" {
		t.Errorf("target = %q, want payment-api", s.TargetComponent)
	}
	if s.Confidence != 0.4 {
		t.Errorf("confidence = %f, want 0.4", s.Confidence)
	}
}

func TestESMAndCommonJSInSameFile(t *testing.T) {
	src := `import axios from 'axios';
const { Pool } = require('pg');

axios.get("http://api-gateway:8080/health");
const pool = new Pool({ host: "primary-db" });
`
	p := NewJSParser()
	signals, err := p.ParseFile("mixed.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d: %+v", len(signals), signals)
	}

	httpSig := signalByTarget(signals, "api-gateway")
	dbSig := signalByKind(signals, "db_connection")

	if httpSig == nil {
		t.Error("missing http_call signal for api-gateway")
	}
	if dbSig == nil {
		t.Error("missing db_connection signal")
	}
}

func TestDestructuredESMImport(t *testing.T) {
	src := `import { get } from 'axios';

const resp = await get("http://internal-svc:3000/data");
`
	p := NewJSParser()
	signals, err := p.ParseFile("client.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	// The destructured import 'get' from 'axios' should map back to axios
	// and trigger detection when used as a bare call
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	if signals[0].TargetComponent != "internal-svc" {
		t.Errorf("target = %q, want internal-svc", signals[0].TargetComponent)
	}
}

func TestMysql2CreateConnection(t *testing.T) {
	src := `import mysql from 'mysql2';

const conn = mysql.createConnection("mysql://db-host:3306/mydb");
`
	p := NewJSParser()
	signals, err := p.ParseFile("db.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	if signals[0].DetectionKind != "db_connection" {
		t.Errorf("kind = %q, want db_connection", signals[0].DetectionKind)
	}
	if signals[0].TargetComponent != "db-host" {
		t.Errorf("target = %q, want db-host", signals[0].TargetComponent)
	}
}

func TestPrismaClient(t *testing.T) {
	src := `import { PrismaClient } from '@prisma/client';

const prisma = new PrismaClient();
`
	p := NewJSParser()
	signals, err := p.ParseFile("prisma.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	if signals[0].DetectionKind != "db_connection" {
		t.Errorf("kind = %q, want db_connection", signals[0].DetectionKind)
	}
}

func TestRedisCreateClient(t *testing.T) {
	src := `const redis = require('redis');

const client = redis.createClient({ url: "redis://cache-02:6379" });
`
	p := NewJSParser()
	signals, err := p.ParseFile("cache.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	if signals[0].DetectionKind != "cache_client" {
		t.Errorf("kind = %q, want cache_client", signals[0].DetectionKind)
	}
}

func TestSQSClient(t *testing.T) {
	src := `import { SQSClient } from '@aws-sdk/client-sqs';

const sqs = new SQSClient({ region: "us-east-1" });
`
	p := NewJSParser()
	signals, err := p.ParseFile("sqs.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	if signals[0].DetectionKind != "queue_producer" {
		t.Errorf("kind = %q, want queue_producer", signals[0].DetectionKind)
	}
}

func TestMongoClient(t *testing.T) {
	src := `const { MongoClient } = require('mongodb');

const client = new MongoClient("mongodb://mongo-host:27017");
`
	p := NewJSParser()
	signals, err := p.ParseFile("mongo.js", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	if signals[0].DetectionKind != "db_connection" {
		t.Errorf("kind = %q, want db_connection", signals[0].DetectionKind)
	}
	if signals[0].TargetComponent != "mongo-host" {
		t.Errorf("target = %q, want mongo-host", signals[0].TargetComponent)
	}
}

func TestSequelizeConstructor(t *testing.T) {
	src := `import { Sequelize } from 'sequelize';

const seq = new Sequelize("postgres://seq-db:5432/app");
`
	p := NewJSParser()
	signals, err := p.ParseFile("orm.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	if signals[0].DetectionKind != "db_connection" {
		t.Errorf("kind = %q, want db_connection", signals[0].DetectionKind)
	}
	if signals[0].TargetComponent != "seq-db" {
		t.Errorf("target = %q, want seq-db", signals[0].TargetComponent)
	}
}

func TestGotHTTPCalls(t *testing.T) {
	src := `import got from 'got';

const resp = await got.get("http://metrics-svc:9090/health");
`
	p := NewJSParser()
	signals, err := p.ParseFile("health.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d: %+v", len(signals), signals)
	}
	if signals[0].TargetComponent != "metrics-svc" {
		t.Errorf("target = %q, want metrics-svc", signals[0].TargetComponent)
	}
}

func TestCommentHintVariants(t *testing.T) {
	src := `// Depends on auth-service
// Uses redis-cache
// Connects to primary-db
function init() {}
`
	p := NewJSParser()
	signals, err := p.ParseFile("init.ts", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 3 {
		t.Fatalf("expected 3 comment hints, got %d: %+v", len(signals), signals)
	}
	targets := map[string]bool{}
	for _, s := range signals {
		targets[s.TargetComponent] = true
		if s.DetectionKind != "comment_hint" {
			t.Errorf("kind = %q, want comment_hint", s.DetectionKind)
		}
	}
	for _, want := range []string{"auth-service", "redis-cache", "primary-db"} {
		if !targets[want] {
			t.Errorf("missing comment hint target %q", want)
		}
	}
}
