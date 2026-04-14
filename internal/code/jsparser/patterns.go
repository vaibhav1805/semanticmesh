package jsparser

import "regexp"

// JSDetectionPattern defines a mapping from a JS/TS import + usage pattern to an
// infrastructure dependency signal.
type JSDetectionPattern struct {
	// Package is the npm package name (e.g., "axios", "pg", "ioredis").
	Package string

	// Function is the function or method name (e.g., "get", "connect", "createConnection").
	Function string

	// Kind is the detection_kind for the signal (e.g., "http_call", "db_connection").
	Kind string

	// TargetType classifies the dependency target (e.g., "service", "database", "cache").
	TargetType string

	// Confidence is the detection certainty in [0.4, 1.0].
	Confidence float64

	// ArgIndex is the position of the argument containing the target URL.
	// -1 means no argument extraction.
	ArgIndex int

	// IsConstructor indicates the pattern matches `new Constructor(...)` syntax.
	IsConstructor bool

	// IsBareCall indicates the pattern matches bare function calls (no package prefix),
	// such as fetch() or destructured imports like get() from axios.
	IsBareCall bool
}

// DefaultJSPatterns contains the built-in detection patterns for JS/TS infrastructure calls.
var DefaultJSPatterns = []JSDetectionPattern{
	// HTTP calls - axios
	{Package: "axios", Function: "get", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "axios", Function: "post", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "axios", Function: "put", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "axios", Function: "delete", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "axios", Function: "patch", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "axios", Function: "request", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},

	// HTTP calls - fetch (global bare call, no import needed)
	{Package: "", Function: "fetch", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0, IsBareCall: true},

	// HTTP calls - got
	{Package: "got", Function: "get", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "got", Function: "post", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},

	// Database - pg
	{Package: "pg", Function: "Pool", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: -1, IsConstructor: true},
	{Package: "pg", Function: "Client", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: -1, IsConstructor: true},

	// Database - mysql2
	{Package: "mysql2", Function: "createConnection", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},
	{Package: "mysql2", Function: "createPool", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},

	// Database - mongoose
	{Package: "mongoose", Function: "connect", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},

	// Database - mongodb
	{Package: "mongodb", Function: "MongoClient", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0, IsConstructor: true},

	// Database - prisma
	{Package: "@prisma/client", Function: "PrismaClient", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: -1, IsConstructor: true},

	// Database - sequelize
	{Package: "sequelize", Function: "Sequelize", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0, IsConstructor: true},

	// Cache - ioredis
	{Package: "ioredis", Function: "Redis", Kind: "cache_client", TargetType: "cache", Confidence: 0.85, ArgIndex: 0, IsConstructor: true},

	// Cache - redis
	{Package: "redis", Function: "createClient", Kind: "cache_client", TargetType: "cache", Confidence: 0.85, ArgIndex: -1},

	// Queue - kafkajs
	{Package: "kafkajs", Function: "Kafka", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85, ArgIndex: -1, IsConstructor: true},

	// Queue - amqplib
	{Package: "amqplib", Function: "connect", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85, ArgIndex: 0},

	// Queue - AWS SQS
	{Package: "@aws-sdk/client-sqs", Function: "SQSClient", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85, ArgIndex: -1, IsConstructor: true},

	// Web frameworks - Express
	{Package: "express", Function: "Router", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1},

	// Web frameworks - Fastify
	{Package: "fastify", Function: "fastify", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1},

	// Web frameworks - Koa
	{Package: "koa", Function: "Koa", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1, IsConstructor: true},

	// Database - TypeORM
	{Package: "typeorm", Function: "createConnection", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: -1},
	{Package: "typeorm", Function: "DataSource", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: -1, IsConstructor: true},

	// Database - Knex
	{Package: "knex", Function: "knex", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: -1},

	// Cache - node-cache
	{Package: "node-cache", Function: "NodeCache", Kind: "cache_client", TargetType: "cache", Confidence: 0.85, ArgIndex: -1, IsConstructor: true},

	// Search - Elasticsearch
	{Package: "@elastic/elasticsearch", Function: "Client", Kind: "search_client", TargetType: "search", Confidence: 0.85, ArgIndex: -1, IsConstructor: true},

	// AWS SDK - S3
	{Package: "@aws-sdk/client-s3", Function: "S3Client", Kind: "storage_client", TargetType: "storage", Confidence: 0.85, ArgIndex: -1, IsConstructor: true},

	// AWS SDK - DynamoDB
	{Package: "@aws-sdk/client-dynamodb", Function: "DynamoDBClient", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: -1, IsConstructor: true},

	// GraphQL
	{Package: "apollo-server", Function: "ApolloServer", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1, IsConstructor: true},
	{Package: "@apollo/server", Function: "ApolloServer", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1, IsConstructor: true},
}

// Import regexes for ESM and CommonJS module systems.
var (
	// ESM default: import axios from 'axios'
	// Also: import axios from "axios"
	esmDefaultRe = regexp.MustCompile(`^\s*import\s+(\w+)\s+from\s+['"]([^'"]+)['"]`)

	// ESM named: import { Pool, Client } from 'pg'
	esmNamedRe = regexp.MustCompile(`^\s*import\s+\{([^}]+)\}\s+from\s+['"]([^'"]+)['"]`)

	// CJS default: const axios = require('axios')
	cjsDefaultRe = regexp.MustCompile(`^\s*(?:const|let|var)\s+(\w+)\s*=\s*require\s*\(\s*['"]([^'"]+)['"]\s*\)`)

	// CJS destructured: const { Pool } = require('pg')
	cjsDestructuredRe = regexp.MustCompile(`^\s*(?:const|let|var)\s+\{([^}]+)\}\s*=\s*require\s*\(\s*['"]([^'"]+)['"]\s*\)`)

	// Dynamic import: const module = await import('package')
	// Also: import('package').then(...)
	dynamicImportRe = regexp.MustCompile(`(?:await\s+)?import\s*\(\s*['"]([^'"]+)['"]\s*\)`)

	// Dynamic import destructured: const { Pool } = await import('pg')
	dynamicImportDestructuredRe = regexp.MustCompile(`(?:const|let|var)\s+\{([^}]+)\}\s*=\s*(?:await\s+)?import\s*\(\s*['"]([^'"]+)['"]\s*\)`)
)

// packageMethodCallRe matches package.method(...) calls.
// Group 1: package/alias name, Group 2: method name
var packageMethodCallRe = regexp.MustCompile(`\b(\w+)\.(\w+)\s*\(`)

// bareCallRe matches bare function calls: funcName(...)
// Group 1: function name
var bareCallRe = regexp.MustCompile(`\b(\w+)\s*\(`)

// constructorCallRe matches new Constructor(...) calls.
// Group 1: constructor name
var constructorCallRe = regexp.MustCompile(`\bnew\s+(\w+)\s*\(`)

// stringArgRe extracts the first string argument from the rest of a line after a call.
// Matches single quotes, double quotes, or backticks (no interpolation).
var stringArgRe = regexp.MustCompile(`['"\x60]([^'"\x60]+)['"\x60]`)
