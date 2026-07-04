// Package idx is the typed client for Bursa Efek Indonesia (idx.co.id) data.
// It composes a fetcher (Cloudflare-aware HTTP) and a cache (SQLite TTL) into
// domain methods: trading info, company profile, and financial report.
//
// Endpoints and payload shapes were validated against live responses during
// the Phase 1 spike; see docs/spike-findings.md.
package idx

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lovanto/idx-mcp/internal/cache"
	"github.com/lovanto/idx-mcp/internal/fetcher"
)

// baseURL is the IDX site root. All API endpoints live under /primary/.
const baseURL = "https://www.idx.co.id"

// Cache TTLs per data class (Phase 1 recommendation):
//   - trading: refreshes through the day, historical rows immutable
//   - profile: rarely changes
//   - financial report: immutable once published (cached forever via ttl<=0)
const (
	ttlTrading = 6 * time.Hour
	ttlProfile = 30 * 24 * time.Hour
	ttlIndex   = 6 * time.Hour
	ttlFinList = 24 * time.Hour
	ttlFinData = 0 // immutable
)

// Client is the high-level IDX data client. It is safe for concurrent use; the
// fetcher serialises outbound requests to honour the rate limit.
type Client struct {
	fetch fetcher.Fetcher
	cache *cache.Cache
}

// New wires a Client from a fetcher and cache. Both are required.
func New(f fetcher.Fetcher, c *cache.Cache) *Client {
	return &Client{fetch: f, cache: c}
}

// getJSON fetches url (through the cache under key with ttl) and unmarshals it
// into out. Cache stores the raw response bytes so a schema change here doesn't
// invalidate on-disk data of a different shape.
func (c *Client) getJSON(ctx context.Context, key, url string, ttl time.Duration, out any) error {
	body, err := c.getRaw(ctx, key, url, ttl)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode %s: %w", key, err)
	}
	return nil
}

// getRaw returns the raw response bytes for url, serving from cache when fresh.
func (c *Client) getRaw(ctx context.Context, key, url string, ttl time.Duration) ([]byte, error) {
	if c.cache != nil {
		if b, ok, err := c.cache.Get(ctx, key); err != nil {
			return nil, err
		} else if ok {
			return b, nil
		}
	}
	body, err := c.fetch.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	if c.cache != nil {
		if err := c.cache.Set(ctx, key, body, ttl); err != nil {
			return nil, fmt.Errorf("cache store %s: %w", key, err)
		}
	}
	return body, nil
}
