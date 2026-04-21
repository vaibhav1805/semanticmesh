package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LLMCacheConfig holds configuration for the LLM cache system.
type LLMCacheConfig struct {
	// CacheDir is the directory where cache files are stored.
	// Defaults to ".bmd-llm-cache/" in the indexed directory.
	CacheDir string

	// DocCacheFile is the document-level cache filename.
	DocCacheFile string

	// CompCacheFile is the component-level cache filename.
	CompCacheFile string

	// MaxAge is the maximum age for cached entries before they're considered stale.
	// Zero means no expiration.
	MaxAge time.Duration

	// Enabled controls whether caching is enabled.
	Enabled bool
}

// DefaultLLMCacheConfig returns an LLMCacheConfig with sensible defaults.
func DefaultLLMCacheConfig() LLMCacheConfig {
	return LLMCacheConfig{
		CacheDir:      ".bmd-llm-cache",
		DocCacheFile:  "document-cache.json",
		CompCacheFile: "component-cache.json",
		MaxAge:        7 * 24 * time.Hour, // 1 week
		Enabled:       true,
	}
}

// ─── document-level cache ────────────────────────────────────────────────────

// DocumentCacheEntry holds LLM-processed results for a single document.
type DocumentCacheEntry struct {
	DocID       string    `json:"doc_id"`
	ContentHash string    `json:"content_hash"`
	Components  []string  `json:"components"`  // Raw component names extracted
	Edges       []string  `json:"edges"`       // Raw edges in "source->target" format
	GeneratedAt time.Time `json:"generated_at"`
}

// DocumentCache is an in-memory document-level cache with JSON persistence.
type DocumentCache struct {
	mu      sync.RWMutex
	entries map[string]*DocumentCacheEntry // docID -> entry
	path    string
	maxAge  time.Duration
}

// NewDocumentCache creates a new document cache.
func NewDocumentCache(cachePath string, maxAge time.Duration) *DocumentCache {
	return &DocumentCache{
		entries: make(map[string]*DocumentCacheEntry),
		path:    cachePath,
		maxAge:  maxAge,
	}
}

// Load reads the cache from disk.
func (c *DocumentCache) Load() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file yet
		}
		return fmt.Errorf("read document cache: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var entries []DocumentCacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse document cache: %w", err)
	}

	// Load entries into map, filtering out stale ones.
	for i := range entries {
		if c.maxAge > 0 && time.Since(entries[i].GeneratedAt) > c.maxAge {
			continue // Skip stale entry
		}
		c.entries[entries[i].DocID] = &entries[i]
	}

	return nil
}

// Save persists the cache to disk.
func (c *DocumentCache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Convert map to slice.
	entries := make([]DocumentCacheEntry, 0, len(c.entries))
	for _, e := range c.entries {
		entries = append(entries, *e)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal document cache: %w", err)
	}

	// Ensure cache directory exists.
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0644); err != nil {
		return fmt.Errorf("write document cache: %w", err)
	}

	return nil
}

// Get retrieves a cached entry for the given document.
// Returns (nil, false) if not found or content hash doesn't match.
func (c *DocumentCache) Get(docID, contentHash string) (*DocumentCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[docID]
	if !ok {
		return nil, false
	}

	// Validate content hash.
	if entry.ContentHash != contentHash {
		return nil, false
	}

	// Check staleness.
	if c.maxAge > 0 && time.Since(entry.GeneratedAt) > c.maxAge {
		return nil, false
	}

	return entry, true
}

// Put stores a cache entry for the given document.
func (c *DocumentCache) Put(entry DocumentCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[entry.DocID] = &entry
}

// ─── component-level cache ───────────────────────────────────────────────────

// ComponentCacheEntry holds normalized metadata for a component name variant.
type ComponentCacheEntry struct {
	NameVariant    string    `json:"name_variant"`     // Original name (e.g., "payments-service")
	CanonicalName  string    `json:"canonical_name"`   // Normalized name (e.g., "payment-service")
	ComponentType  string    `json:"component_type"`   // service, database, etc.
	Description    string    `json:"description"`      // LLM-generated description
	Tags           []string  `json:"tags"`             // LLM-extracted tags
	Confidence     float64   `json:"confidence"`       // Normalization confidence
	IsValidComponent bool    `json:"is_valid"`         // False if marked as false positive
	GeneratedAt    time.Time `json:"generated_at"`
}

// ComponentCache is an in-memory component-level cache with JSON persistence.
type ComponentCache struct {
	mu      sync.RWMutex
	entries map[string]*ComponentCacheEntry // nameVariant -> entry
	path    string
	maxAge  time.Duration
}

// NewComponentCache creates a new component cache.
func NewComponentCache(cachePath string, maxAge time.Duration) *ComponentCache {
	return &ComponentCache{
		entries: make(map[string]*ComponentCacheEntry),
		path:    cachePath,
		maxAge:  maxAge,
	}
}

// Load reads the cache from disk.
func (c *ComponentCache) Load() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file yet
		}
		return fmt.Errorf("read component cache: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var entries []ComponentCacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse component cache: %w", err)
	}

	// Load entries into map, filtering out stale ones.
	for i := range entries {
		if c.maxAge > 0 && time.Since(entries[i].GeneratedAt) > c.maxAge {
			continue // Skip stale entry
		}
		c.entries[entries[i].NameVariant] = &entries[i]
	}

	return nil
}

// Save persists the cache to disk.
func (c *ComponentCache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Convert map to slice.
	entries := make([]ComponentCacheEntry, 0, len(c.entries))
	for _, e := range c.entries {
		entries = append(entries, *e)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal component cache: %w", err)
	}

	// Ensure cache directory exists.
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	if err := os.WriteFile(c.path, data, 0644); err != nil {
		return fmt.Errorf("write component cache: %w", err)
	}

	return nil
}

// Get retrieves a cached entry for the given component name variant.
// Returns (nil, false) if not found.
func (c *ComponentCache) Get(nameVariant string) (*ComponentCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[nameVariant]
	if !ok {
		return nil, false
	}

	// Check staleness.
	if c.maxAge > 0 && time.Since(entry.GeneratedAt) > c.maxAge {
		return nil, false
	}

	return entry, true
}

// Put stores a cache entry for the given component name variant.
func (c *ComponentCache) Put(entry ComponentCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[entry.NameVariant] = &entry
}

// GetAllValid returns all cached components marked as valid (not false positives).
func (c *ComponentCache) GetAllValid() []ComponentCacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var valid []ComponentCacheEntry
	for _, entry := range c.entries {
		if entry.IsValidComponent {
			// Check staleness.
			if c.maxAge > 0 && time.Since(entry.GeneratedAt) > c.maxAge {
				continue
			}
			valid = append(valid, *entry)
		}
	}
	return valid
}

// ─── cache manager ───────────────────────────────────────────────────────────

// LLMCacheManager coordinates document-level and component-level caches.
type LLMCacheManager struct {
	docCache  *DocumentCache
	compCache *ComponentCache
	config    LLMCacheConfig
}

// NewLLMCacheManager creates a new LLM cache manager.
func NewLLMCacheManager(cfg LLMCacheConfig) *LLMCacheManager {
	docCachePath := filepath.Join(cfg.CacheDir, cfg.DocCacheFile)
	compCachePath := filepath.Join(cfg.CacheDir, cfg.CompCacheFile)

	return &LLMCacheManager{
		docCache:  NewDocumentCache(docCachePath, cfg.MaxAge),
		compCache: NewComponentCache(compCachePath, cfg.MaxAge),
		config:    cfg,
	}
}

// Load reads both caches from disk.
func (m *LLMCacheManager) Load() error {
	if !m.config.Enabled {
		return nil
	}

	if err := m.docCache.Load(); err != nil {
		return fmt.Errorf("load document cache: %w", err)
	}
	if err := m.compCache.Load(); err != nil {
		return fmt.Errorf("load component cache: %w", err)
	}
	return nil
}

// Save persists both caches to disk.
func (m *LLMCacheManager) Save() error {
	if !m.config.Enabled {
		return nil
	}

	if err := m.docCache.Save(); err != nil {
		return fmt.Errorf("save document cache: %w", err)
	}
	if err := m.compCache.Save(); err != nil {
		return fmt.Errorf("save component cache: %w", err)
	}
	return nil
}

// GetDocumentCache returns the document-level cache.
func (m *LLMCacheManager) GetDocumentCache() *DocumentCache {
	return m.docCache
}

// GetComponentCache returns the component-level cache.
func (m *LLMCacheManager) GetComponentCache() *ComponentCache {
	return m.compCache
}

// ─── utility functions ───────────────────────────────────────────────────────

// HashContent computes a SHA256 hash of the given content.
func HashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
